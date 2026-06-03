package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestProcessImage_Valid(t *testing.T) {
	// Set up appConfig for the test
	appConfig.MaxImageSizeMB = 10

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a form file
	filename := "test.png"
	fileContent := []byte("fake-image-data")
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(fileContent)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	imgBytes, gotFilename, err := processImage(req)
	if err != nil {
		t.Fatalf("processImage failed: %v", err)
	}

	if !bytes.Equal(imgBytes, fileContent) {
		t.Errorf("expected %s, got %s", string(fileContent), string(imgBytes))
	}

	if gotFilename != filename {
		t.Errorf("expected filename %s, got %s", filename, gotFilename)
	}
}

func TestProcessImage_MissingImage(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, _, err := processImage(req)
	if err == nil {
		t.Fatal("expected error for missing image, got nil")
	}
}

func TestProcessImage_FileTooLarge(t *testing.T) {
	// Set a very small limit
	appConfig.MaxImageSizeMB = 1

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create a part that is larger than 1MB
	part, err := writer.CreateFormFile("image", "large.png")
	if err != nil {
		t.Fatal(err)
	}

	largeContent := make([]byte, 2<<20) // 2MB
	part.Write(largeContent)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, _, err = processImage(req)
	if err == nil {
		t.Fatal("expected error for file too large, got nil")
	}
}

func TestOCRResponse_JSON(t *testing.T) {
	resp := OCRResponse{
		Text:   "hello",
		Status: "success",
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"text":"hello","status":"success"}`
	if string(b) != expected {
		t.Errorf("expected %s, got %s", expected, string(b))
	}

	respErr := OCRResponse{
		Status: "error",
		Error:  "something went wrong",
	}
	b, err = json.Marshal(respErr)
	if err != nil {
		t.Fatal(err)
	}
	expectedErr := `{"text":"","status":"error","error":"something went wrong"}`
	if string(b) != expectedErr {
		t.Errorf("expected %s, got %s", expectedErr, string(b))
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Remove config.json if it exists to test defaults
	os.Rename("config.json", "config.json.bak")
	defer os.Rename("config.json.bak", "config.json")

	loadConfig()

	if appConfig.QueueBufferSize != 100 {
		t.Errorf("expected default QueueBufferSize 100, got %d", appConfig.QueueBufferSize)
	}
	if appConfig.Port != 30000 {
		t.Errorf("expected default Port 30000, got %d", appConfig.Port)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"UP","version":"1.0.0"}`
	if rr.Body.String() != expected+"\n" {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestIndexHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(indexHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("handler returned wrong content type: got %v want text/html; charset=utf-8", contentType)
	}
}

func TestOCRHandler_GET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ocr", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ocrHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("handler returned wrong content type: got %v want text/html; charset=utf-8", contentType)
	}
}

func TestOCRHandler_Success(t *testing.T) {
	// Initialize jobQueue
	appConfig.QueueBufferSize = 1
	jobQueue = make(chan Job, 1)

	// Start a mock worker
	go func() {
		job := <-jobQueue
		job.ResultChan <- JobResult{Text: "mocked text", Error: nil}
	}()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.png")
	part.Write([]byte("fake-image-data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ocr", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	ocrHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp OCRResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Text != "mocked text" {
		t.Errorf("expected 'mocked text', got '%s'", resp.Text)
	}
}

func TestTestHandler_Success(t *testing.T) {
	// Initialize jobQueue
	appConfig.QueueBufferSize = 1
	jobQueue = make(chan Job, 1)

	// Start a mock worker
	go func() {
		job := <-jobQueue
		job.ResultChan <- JobResult{Text: "mocked text", Error: nil}
	}()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("image", "test.png")
	part.Write([]byte("fake-image-data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	testHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if rr.Body.String() != "mocked text" {
		t.Errorf("expected 'mocked text', got '%s'", rr.Body.String())
	}
}

func TestLoadConfig_Custom(t *testing.T) {
	configContent := `{"worker_pool_size": 5, "queue_buffer_size": 20, "port": 8080, "max_image_size_mb": 5}`
	err := os.WriteFile("config_test.json", []byte(configContent), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("config_test.json")

	// We need to modify loadConfig to accept a filename or use a hack
	// Since we can't easily modify main.go's loadConfig without affecting it,
	// let's temporarily swap config.json
	os.Rename("config.json", "config.json.bak")
	defer os.Rename("config.json.bak", "config.json")

	os.WriteFile("config.json", []byte(configContent), 0644)
	defer os.Remove("config.json")

	loadConfig()

	if appConfig.WorkerPoolSize != 5 {
		t.Errorf("expected 5, got %d", appConfig.WorkerPoolSize)
	}
	if appConfig.Port != 8080 {
		t.Errorf("expected 8080, got %d", appConfig.Port)
	}
}
