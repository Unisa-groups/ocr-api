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
	log.Printf("[SQUINT]: Worker thread #%d starting up...\n", id)
	// CRITICAL FIX: Bind this goroutine to a dedicated OS thread.
	// This ensures CGO stability and prevents OpenMP state corruption.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log.Printf("[SQUINT]: Worker thread #%d spawned and idling...", id)

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
	// IMPROVEMENT: Log immediately upon entry to verify the connection is active
	log.Printf("[SQUINT]: Incoming %s request from %s to %s\n", request.Method, request.RemoteAddr, request.URL.Path)
	writer.Header().Set("Content-Type", "application/json")

	// 1. Enforce POST requests only
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "[SQUINT]: Only POST requests are allowed"})
		log.Printf("[SQUINT]: Rejected non-POST request from %s\n", request.RemoteAddr)
		return
	}

	// 2. Parse the multipart form (Limit upload size to 10MB to protect memory)
	err := request.ParseMultipartForm(10 << 20)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "[SQUINT]: Failed to parse form or file exceeds 10MB limit"})
		log.Printf("[SQUINT]: Failed to parse multipart form from %s: %v\n", request.RemoteAddr, err)
		return
	}

	// 3. Retrieve the file from the form data (Expecting the key "image")
	file, _, err := request.FormFile("image")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "[SQUINT]: Missing 'image' file in form data"})
		log.Printf("[SQUINT]: Missing 'image' file in request from %s: %v\n", request.RemoteAddr, err)
		return
	}
	defer file.Close()

	// 4. Read the file bytes directly into memory
	imgBytes, err := io.ReadAll(file)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "[SQUINT]: Stream read failure"})
		log.Printf("[SQUINT]: Failed to read image bytes from %s: %v\n", request.RemoteAddr, err)
		return
	}

	// 5. Create a communication channel and dispatch the job to the workers
	resultChan := make(chan JobResult, 1)
	job := Job{
		ImageBytes: imgBytes,
		ResultChan: resultChan,
	}

	jobQueue <- job              // Send to the pool
	workerResult := <-resultChan // Block and wait for a worker to finish

	if workerResult.Error != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: workerResult.Error.Error()})
		log.Printf("[SQUINT]: Worker error for request from %s: %v\n", request.RemoteAddr, workerResult.Error)
		return
	}

	// 6. Return success
	json.NewEncoder(writer).Encode(OCRResponse{
		Text:   workerResult.Text,
		Status: "[SQUINT]: Success",
	})
}

func main() {
	log.Printf("STARTING OCR")
	http.HandleFunc("/api/v1/ocr", ocrHandler)
	log.Printf("[SQUINT]: Starting server with configuration: %+v\n", appConfig)

	portStr := fmt.Sprintf(":%d", appConfig.Port)
	log.Printf("[SQUINT]: Production Multi-Threaded Gateway live on port %d...", appConfig.Port)

	if err := http.ListenAndServe(portStr, nil); err != nil {
		log.Fatalf("[SQUINT]: Server initialization failure: %v", err)
	}
	log.Println("[SQUINT]: Server shutdown gracefully.")
}
