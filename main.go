package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/otiai10/gosseract/v2"
)

// Config represents the customizable parameters for the OCR Engine
type Config struct {
	WorkerPoolSize  int `json:"worker_pool_size"`
	QueueBufferSize int `json:"queue_buffer_size"`
	Port            int `json:"port"`
}

// Job represents an OCR task waiting for an available worker thread
type Job struct {
	ImageBytes []byte
	ResultChan chan JobResult
}

// JobResult is the payload sent back from the worker to the HTTP thread
type JobResult struct {
	Text  string
	Error error
}

type OCRResponse struct {
	Text   string `json:"text"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var jobQueue chan Job
var appConfig Config

// loadConfig reads from config.json or applies safe defaults
func loadConfig() {
	appConfig = Config{
		WorkerPoolSize:  runtime.NumCPU(), // Default to available cores
		QueueBufferSize: 100,
		Port:            8080,
	}

	file, err := os.Open("config.json")
	if err != nil {
		log.Println("[SQUINT]: No config.json found. Using default settings.")
		return
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&appConfig); err != nil {
		log.Printf("[SQUINT]: Failed to parse config.json (%v). Using default settings.\n", err)
		return
	}

	// Safety rails so bad configs don't crash the server
	if appConfig.WorkerPoolSize < 1 {
		appConfig.WorkerPoolSize = 1
	}
	if appConfig.QueueBufferSize < 1 {
		appConfig.QueueBufferSize = 10
	}
	log.Println("[SQUINT]: Successfully loaded configuration from config.json")
}

func init() {
	loadConfig()

	// Create a buffered channel for incoming requests
	jobQueue = make(chan Job, appConfig.QueueBufferSize)

	// Spin up the background worker pool
	for i := 1; i <= appConfig.WorkerPoolSize; i++ {
		go ocrWorker(i, jobQueue)
	}
	log.Printf("[SQUINT]: Initialized worker pool with %d parallel Tesseract daemons.", appConfig.WorkerPoolSize)
}

// ocrWorker runs infinitely in the background, grabbing jobs from the queue
func ocrWorker(id int, queue chan Job) {
	log.Printf("[SQUINT]: Worker thread #%d spawned and idling...", id)

	// Crucial optimization: C++ client is initialized ONCE per thread
	tesseractClient := gosseract.NewClient()
	defer tesseractClient.Close()
	tesseractClient.SetPageSegMode(gosseract.PSM_SINGLE_BLOCK)

	for job := range queue {
		var res JobResult

		if err := tesseractClient.SetImageFromBytes(job.ImageBytes); err != nil {
			res.Error = fmt.Errorf("failed to load image bytes into worker #%d: %v", id, err)
		} else {
			text, err := tesseractClient.Text()
			if err != nil {
				res.Error = fmt.Errorf("worker #%d core exception: %v", id, err)
			} else {
				res.Text = text
			}
		}

		// Ship result payload directly back to the HTTP handler
		job.ResultChan <- res
	}
}

func ocrHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")

	imageURL := request.URL.Query().Get("image_url")
	if imageURL == "" {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "[SQUINT]: Missing image_url parameter"})
		return
	}

	// Download image using the browser spoof to bypass scraping blockers (e.g., Wikimedia)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", imageURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	response, err := client.Do(req)
	if err != nil {
		writer.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: fmt.Sprintf("[SQUINT]: Failed download: %v", err)})
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		writer.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: fmt.Sprintf("[SQUINT]: Host returned HTTP %d", response.StatusCode)})
		return
	}

	imgBytes, err := io.ReadAll(response.Body)
	if err != nil {
		writer.WriteHeader(http.StatusFailedDependency)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "[SQUINT]: Stream read failure"})
		return
	}

	// Create a communication channel and dispatch the job to the workers
	resultChan := make(chan JobResult)
	job := Job{
		ImageBytes: imgBytes,
		ResultChan: resultChan,
	}

	jobQueue <- job              // Send to the pool
	workerResult := <-resultChan // Block and wait for a worker to finish

	if workerResult.Error != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: workerResult.Error.Error()})
		return
	}

	json.NewEncoder(writer).Encode(OCRResponse{
		Text:   workerResult.Text,
		Status: "[SQUINT]: Success",
	})
}

func main() {
	http.HandleFunc("/api/v1/ocr", ocrHandler)

	portStr := fmt.Sprintf(":%d", appConfig.Port)
	log.Printf("[SQUINT]: Production Multi-Threaded Gateway live on port %d...", appConfig.Port)

	if err := http.ListenAndServe(portStr, nil); err != nil {
		log.Fatalf("[SQUINT]: Server initialization failure: %v", err)
	}
}
