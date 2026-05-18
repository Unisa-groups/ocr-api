package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/otiai10/gosseract/v2"
)

// Config represents the customizable parameters for the OCR Engine
type Config struct {
	WorkerPoolSize  int `json:"worker_pool_size"`
	QueueBufferSize int `json:"queue_buffer_size"`
	Port            int `json:"port"`
	MaxImageSizeMB  int `json:"max_image_size_mb"`
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
		MaxImageSizeMB:  10, // Default to 10 MB
	}

	file, err := os.Open("config.json")
	if err != nil {
		log.Println("⚙️  [SQUINT]: No config.json found. Using default settings.")
		return
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&appConfig); err != nil {
		log.Printf("⚠️  [SQUINT]: Failed to parse config.json (%v). Using default settings.\n", err)
		return
	}

	// Safety rails so bad configs don't crash the server
	if appConfig.WorkerPoolSize < 1 {
		appConfig.WorkerPoolSize = 1
	}
	if appConfig.QueueBufferSize < 1 {
		appConfig.QueueBufferSize = 10
	}
	if appConfig.MaxImageSizeMB < 1 {
		appConfig.MaxImageSizeMB = 10
	}
	log.Println("✅ [SQUINT]: Successfully loaded configuration from config.json")
}

func init() {
	loadConfig()

	// Create a buffered channel for incoming requests
	jobQueue = make(chan Job, appConfig.QueueBufferSize)

	// Spin up the background worker pool
	for i := 1; i <= appConfig.WorkerPoolSize; i++ {
		go ocrWorker(i, jobQueue)
	}
	log.Printf("🔧 [SQUINT]: Initialized worker pool with %d parallel Tesseract daemons.\n", appConfig.WorkerPoolSize)
}

// ocrWorker runs infinitely in the background, grabbing jobs from the queue
func ocrWorker(id int, queue chan Job) {
	log.Printf("🚀 [SQUINT]: Worker thread #%d starting up...\n", id)
	// CRITICAL FIX: Bind this goroutine to a dedicated OS thread.
	// This ensures CGO stability and prevents OpenMP state corruption.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log.Printf("✨ [SQUINT]: Worker thread #%d spawned and idling...\n", id)

	tesseractClient := gosseract.NewClient()
	defer tesseractClient.Close()
	tesseractClient.SetPageSegMode(gosseract.PSM_SINGLE_BLOCK)

	for job := range queue {
		var res JobResult

		if err := tesseractClient.SetImageFromBytes(job.ImageBytes); err != nil {
			res.Error = fmt.Errorf("❌ failed to load image bytes into worker #%d: %v", id, err)
		} else {
			text, err := tesseractClient.Text()
			if err != nil {
				res.Error = fmt.Errorf("❌ worker #%d core exception: %v", id, err)
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
	log.Printf("📨 [SQUINT]: Incoming %s request from %s to %s\n", request.Method, request.RemoteAddr, request.URL.Path)
	writer.Header().Set("Content-Type", "application/json")

	// 1. Enforce POST requests only
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "❌ Only POST requests are allowed"})
		log.Printf("🚫 [SQUINT]: Rejected non-POST request from %s\n", request.RemoteAddr)
		return
	}

	// 2. Parse the multipart form (Limit upload size using configured MaxImageSizeMB)
	maxSize := int64(appConfig.MaxImageSizeMB) << 20
	err := request.ParseMultipartForm(maxSize)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: fmt.Sprintf("❌ Failed to parse form or file exceeds %d MB limit", appConfig.MaxImageSizeMB)})
		log.Printf("⚠️  [SQUINT]: Failed to parse multipart form from %s: %v\n", request.RemoteAddr, err)
		return
	}

	// 3. Retrieve the file from the form data (Expecting the key "image")
	file, fileHeader, err := request.FormFile("image")
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "❌ Missing 'image' file in form data"})
		log.Printf("🚫 [SQUINT]: Missing 'image' file in request from %s: %v\n", request.RemoteAddr, err)
		return
	}
	defer file.Close()

	// Pre-flight file size check
	fileSizeKB := fileHeader.Size / 1024
	if fileHeader.Size > maxSize {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: fmt.Sprintf("❌ File exceeds %d MB limit", appConfig.MaxImageSizeMB)})
		log.Printf("📦 [SQUINT]: REJECTED oversized file from %s: %s (%d KB, max: %d MB)\n", request.RemoteAddr, fileHeader.Filename, fileSizeKB, appConfig.MaxImageSizeMB)
		return
	}

	// 4. Read the file bytes directly into memory
	imgBytes, err := io.ReadAll(file)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "❌ Stream read failure"})
		log.Printf("⚠️  [SQUINT]: Failed to read image bytes from %s: %v\n", request.RemoteAddr, err)
		return
	}

	log.Printf("📥 [SQUINT]: Read image file %s from %s (%d KB)\n", fileHeader.Filename, request.RemoteAddr, fileSizeKB)

	// 5. Create a communication channel and dispatch the job to the workers
	resultChan := make(chan JobResult, 1)
	job := Job{
		ImageBytes: imgBytes,
		ResultChan: resultChan,
	}

	// Use a non-blocking send with timeout to detect queue saturation
	select {
	case jobQueue <- job:
		log.Printf("⏳ [SQUINT]: Job queued for request from %s\n", request.RemoteAddr)
		// Successfully queued, now wait for result with timeout
		select {
		case workerResult := <-resultChan:
			if workerResult.Error != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: fmt.Sprintf("❌ %v", workerResult.Error)})
				log.Printf("❌ [SQUINT]: Worker error for request from %s: %v\n", request.RemoteAddr, workerResult.Error)
				return
			}

			// 6. Return success
			json.NewEncoder(writer).Encode(OCRResponse{
				Text:   workerResult.Text,
				Status: "✅ Success",
			})
			log.Printf("✅ [SQUINT]: Successfully processed request from %s\n", request.RemoteAddr)
			imgBytes = nil // Clear image from memory
		case <-time.After(30 * time.Second):
			writer.WriteHeader(http.StatusRequestTimeout)
			json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "⏱️  Worker processing timeout (30s exceeded)"})
			log.Printf("⏱️  [SQUINT]: Worker timeout for request from %s\n", request.RemoteAddr)
		}
	case <-time.After(5 * time.Second):
		writer.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: "🔄 Job queue full, service unavailable"})
		log.Printf("🔄 [SQUINT]: Queue timeout for request from %s\n", request.RemoteAddr)
	}
}

func main() {
	log.Printf("🎯 STARTING OCR\n")
	http.HandleFunc("/api/v1/ocr", ocrHandler)
	log.Printf("⚙️  [SQUINT]: Starting server with configuration: %+v\n", appConfig)

	portStr := fmt.Sprintf(":%d", appConfig.Port)
	log.Printf("🌐 [SQUINT]: Production Multi-Threaded Gateway live on port %d...\n", appConfig.Port)

	if err := http.ListenAndServe(portStr, nil); err != nil {
		log.Fatalf("💥 [SQUINT]: Server initialization failure: %v", err)
	}
	log.Println("👋 [SQUINT]: Server shutdown gracefully.")
}
