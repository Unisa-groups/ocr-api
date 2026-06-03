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
		Port:            30000,
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

func processImage(request *http.Request) ([]byte, string, error) {
	// 2. Parse the multipart form (Limit upload size using configured MaxImageSizeMB)
	maxSize := int64(appConfig.MaxImageSizeMB) << 20
	err := request.ParseMultipartForm(maxSize)
	if err != nil {
		return nil, "", fmt.Errorf("❌ Failed to parse form or file exceeds %d MB limit", appConfig.MaxImageSizeMB)
	}

	// 3. Retrieve the file from the form data (Expecting the key "image")
	file, fileHeader, err := request.FormFile("image")
	if err != nil {
		return nil, "", fmt.Errorf("❌ Missing 'image' file in form data")
	}
	defer file.Close()

	// Pre-flight file size check
	if fileHeader.Size > maxSize {
		return nil, "", fmt.Errorf("❌ File exceeds %d MB limit", appConfig.MaxImageSizeMB)
	}

	// 4. Read the file bytes directly into memory
	imgBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("❌ Stream read failure")
	}

	return imgBytes, fileHeader.Filename, nil
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

	imgBytes, filename, err := processImage(request)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(writer).Encode(OCRResponse{Status: "error", Error: err.Error()})
		log.Printf("⚠️  [SQUINT]: Image processing failed for %s: %v\n", request.RemoteAddr, err)
		return
	}

	fileSizeKB := len(imgBytes) / 1024
	log.Printf("📥 [SQUINT]: Read image file %s from %s (%d KB)\n", filename, request.RemoteAddr, fileSizeKB)

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

func testHandler(writer http.ResponseWriter, request *http.Request) {
	log.Printf("📨 [SQUINT-TEST]: Incoming %s request from %s to %s\n", request.Method, request.RemoteAddr, request.URL.Path)

	if request.Method == http.MethodGet {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(writer, `
			<!DOCTYPE html>
			<html>
			<head>
				<title>Squint OCR Test</title>
				<style>
					body { font-family: sans-serif; max-width: 800px; margin: 40px auto; padding: 20px; line-height: 1.6; background: #f4f7f6; }
					.container { background: white; padding: 30px; border-radius: 12px; box-shadow: 0 4px 15px rgba(0,0,0,0.1); }
					h1 { color: #333; margin-top: 0; }
					form { border: 2px dashed #007bff; padding: 20px; border-radius: 8px; text-align: center; margin-bottom: 20px; transition: background 0.3s; }
					form:hover { background: #f0f7ff; }
					input[type="file"] { margin-bottom: 20px; display: block; margin-left: auto; margin-right: auto; }
					.buttons { display: flex; gap: 10px; justify-content: center; }
					input[type="submit"], button { background: #007bff; color: white; border: none; padding: 12px 24px; border-radius: 6px; cursor: pointer; font-size: 16px; font-weight: bold; transition: background 0.2s; }
					input[type="submit"]:hover { background: #0056b3; }
					button.clear { background: #6c757d; }
					button.clear:hover { background: #5a6268; }
					#results-container { margin-top: 30px; display: none; }
					h3 { margin-bottom: 10px; color: #444; }
					#results { background: #2d2d2d; color: #f8f8f2; padding: 20px; border-radius: 8px; white-space: pre-wrap; word-break: break-all; min-height: 100px; max-height: 500px; overflow-y: auto; font-family: 'Courier New', Courier, monospace; border-left: 5px solid #007bff; }
					.loading { display: none; color: #007bff; font-weight: bold; margin-bottom: 10px; }
				</style>
			</head>
			<body>
				<div class="container">
					<h1>📷 Squint OCR Test</h1>
					<p>Upload an image to see the raw OCR results in real-time.</p>
					
					<form id="ocrForm">
						<input type="file" id="imageInput" name="image" accept="image/*" required>
						<div class="buttons">
							<input type="submit" value="Extract Text">
							<button type="button" class="clear" onclick="resetForm()">Clear</button>
						</div>
					</form>

					<div id="results-container">
						<div class="loading" id="loading">⌛ Processing image...</div>
						<h3>Results:</h3>
						<pre id="results"></pre>
					</div>
				</div>

				<script>
					const form = document.getElementById('ocrForm');
					const resultsContainer = document.getElementById('results-container');
					const resultsElement = document.getElementById('results');
					const loading = document.getElementById('loading');

					form.onsubmit = async (e) => {
						e.preventDefault();
						const formData = new FormData(form);
						
						resultsContainer.style.display = 'block';
						resultsElement.textContent = '';
						loading.style.display = 'block';
						
						try {
							const response = await fetch('/test', {
								method: 'POST',
								body: formData
							});
							
							if (!response.ok) {
								const errorText = await response.text();
								throw new Error(errorText || 'Server error');
							}
							
							const text = await response.text();
							resultsElement.textContent = text || '(No text detected)';
						} catch (err) {
							resultsElement.textContent = '❌ Error: ' + err.message;
							resultsElement.style.color = '#ff6b6b';
						} finally {
							loading.style.display = 'none';
						}
					};

					function resetForm() {
						form.reset();
						resultsElement.textContent = '';
						resultsElement.style.color = '#f8f8f2';
						resultsContainer.style.display = 'none';
					}
				</script>
			</body>
			</html>
		`)
		return
	}

	if request.Method != http.MethodPost {
		http.Error(writer, "❌ Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	imgBytes, filename, err := processImage(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("📥 [SQUINT-TEST]: Read image file %s from %s (%d KB)\n", filename, request.RemoteAddr, len(imgBytes)/1024)

	resultChan := make(chan JobResult, 1)
	job := Job{
		ImageBytes: imgBytes,
		ResultChan: resultChan,
	}

	select {
	case jobQueue <- job:
		select {
		case workerResult := <-resultChan:
			if workerResult.Error != nil {
				http.Error(writer, workerResult.Error.Error(), http.StatusInternalServerError)
				return
			}
			writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(writer, workerResult.Text)
			log.Printf("✅ [SQUINT-TEST]: Successfully processed request from %s\n", request.RemoteAddr)
		case <-time.After(30 * time.Second):
			http.Error(writer, "⏱️  Worker processing timeout", http.StatusRequestTimeout)
		}
	case <-time.After(5 * time.Second):
		http.Error(writer, "🔄 Job queue full", http.StatusServiceUnavailable)
	}
}

func main() {
	log.Printf("🎯 STARTING OCR\n")
	http.HandleFunc("/api/v1/ocr", ocrHandler)
	http.HandleFunc("/test", testHandler)
	log.Printf("⚙️  [SQUINT]: Starting server with configuration: %+v\n", appConfig)

	portStr := fmt.Sprintf(":%d", appConfig.Port)
	log.Printf("🌐 [SQUINT]: Production Multi-Threaded Gateway live on port %d...\n", appConfig.Port)

	if err := http.ListenAndServe(portStr, nil); err != nil {
		log.Fatalf("💥 [SQUINT]: Server initialization failure: %v", err)
	}
	log.Println("👋 [SQUINT]: Server shutdown gracefully.")
}
