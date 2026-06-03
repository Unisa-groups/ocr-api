# Squint

An OCR (Optical Character Recognition) microservice built with Go and Tesseract, designed to extract text from images with a simple HTTP API.

## 🎯 Project Info

- **Primary Language**: Go (71.5%)
- **Secondary Language**: Python (28.5%)
- **Type**: Microservice
- **API**: RESTful HTTP interface
- **Architecture**: Multi-threaded worker pool with queue-based job processing

## 🚀 Features

- **Web UI** - Interactive web interface for easy image uploads and OCR result visualization
- **HTTP REST API** - Simple POST endpoint to process images and extract text
- **Multipart Form Upload** - Direct image file upload via multipart/form-data
- **Worker Pool Architecture** - Configurable worker threads with queue-based job distribution
- **Tesseract Integration** - Leverages the powerful open-source Tesseract OCR engine
- **In-Memory Processing** - Images are processed directly in memory without disk storage
- **Configurable** - Customizable worker pool size, queue buffer, and max image size via config.json
- **Docker Ready** - Includes Docker and Docker Compose configuration for easy deployment
- **Comprehensive Error Handling** - Detailed error messages for debugging and monitoring
- **Thread-Safe** - Uses OS-level thread locking for CGO stability with concurrent requests

## 📋 Requirements

- Go 1.25 or later
- Tesseract OCR engine (5.x recommended)
- Docker & Docker Compose (for containerized deployment)
- Python 3.x (for testing scripts)

## 🛠️ Installation

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/RedChlorine/Squint.git
cd Squint
```

2. Install dependencies:
```bash
go mod download
```

3. Install Tesseract OCR (system dependency):
   - **Ubuntu/Debian**: `sudo apt-get install tesseract-ocr tesseract-ocr-eng`
   - **macOS**: `brew install tesseract`
   - **Windows**: Download from [GNU Tesseract OCR](https://github.com/UB-Mannheim/tesseract/wiki)

4. (Optional) Configure settings in `config.json`:
```json
{
    "worker_pool_size": 4,
    "queue_buffer_size": 50,
    "port": 3000,
    "max_image_size_mb": 10
}
```

5. Run the service:
```bash
go run main.go
```

The service will start on `http://localhost:30000`

### Web Interface

Simply open `http://localhost:30000` in your web browser to access the Squint OCR web interface. You can drag and drop images to extract text immediately.

### Docker Deployment

1. Using Docker Compose:
```bash
docker-compose up
```

2. Or build and run with Docker directly:
```bash
docker build -t squint .
docker run -p 30000:30000 squint
```

## 📡 API Usage

### Endpoint

```
POST /api/v1/ocr
```

### Parameters

- `image` (required): Image file uploaded via multipart/form-data (supports JPEG, PNG, BMP, GIF, TIFF, and other formats supported by Tesseract)

### Request Examples

**Using cURL:**
```bash
curl -X POST -F "image=@/path/to/image.jpg" "http://localhost:30000/api/v1/ocr"
```

**Using JavaScript/Fetch:**
```javascript
const formData = new FormData();
formData.append('image', fileInputElement.files[0]);

fetch('http://localhost:30000/api/v1/ocr', {
  method: 'POST',
  body: formData
})
  .then(response => response.json())
  .then(data => console.log('Extracted text:', data.text))
  .catch(error => console.error('Error:', error));
```

**Using Python (with requests library):**
```python
import requests

with open('/path/to/image.jpg', 'rb') as img_file:
    files = {'image': img_file}
    response = requests.post('http://localhost:30000/api/v1/ocr', files=files)
    data = response.json()
    print('Extracted text:', data['text'])
```

### Response Example (Success - 200 OK)

```json
{
  "text": "The extracted text from the image",
  "status": "✅ Success"
}
```

### Response Example (Error - Missing Image - 400 Bad Request)

```json
{
  "status": "error",
  "error": "❌ Missing 'image' file in form data"
}
```

### Response Example (Error - File Too Large - 400 Bad Request)

```json
{
  "status": "error",
  "error": "❌ File exceeds 10 MB limit"
}
```

### Response Example (Error - Processing Timeout - 408 Request Timeout)

```json
{
  "status": "error",
  "error": "⏱️  Worker processing timeout (30s exceeded)"
}
```

### Response Example (Error - Queue Full - 503 Service Unavailable)

```json
{
  "status": "error",
  "error": "🔄 Job queue full, service unavailable"
}
```

## 🔍 HTTP Status Codes

| Status Code | Scenario |
|---|---|
| `200 OK` | OCR processing successful |
| `400 Bad Request` | Missing image file or file exceeds size limit |
| `405 Method Not Allowed` | Non-POST request to the endpoint |
| `408 Request Timeout` | Worker processing exceeded 30 seconds |
| `500 Internal Server Error` | Tesseract OCR processing failed or image stream read error |
| `503 Service Unavailable` | Job queue full, unable to accept new requests |

## 📦 Project Structure

```
Squint/
├── main.go                 # Main application with HTTP handler and worker pool
├── index.html              # Web UI template
├── config.json             # Configuration file (worker pool, queue, port, max file size)
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── dockerfile              # Docker container configuration
├── docker-compose.yml      # Docker Compose setup
├── test_squint.py          # Python test script
└── README.md               # This file
```

## 🧪 Testing

A Python test script is included to validate the OCR service:

1. Place test images in a `test_images` directory:
```bash
mkdir test_images
# Copy your .jpg, .png, .bmp, .tiff files into test_images/
```

2. Run the test script:
```bash
# Install dependencies (if needed)
pip install requests

# Run the tests
python test_squint.py
```

The test script will process all images in the `test_images` directory and display the extracted text.

## 🏗️ How It Works

1. **Application Startup** - Loads configuration and spins up a worker pool with configurable parallel Tesseract daemons
2. **Worker Pool** - Each worker thread is bound to a dedicated OS thread for CGO stability
3. **Receives Request** - Client sends POST request with an image file via multipart/form-data
4. **Validates Input** - Ensures the 'image' parameter is provided and file doesn't exceed size limit
5. **Queues Job** - Dispatches OCR job to the worker queue with a 5-second timeout
6. **Worker Processing** - Available worker picks up job, processes image through Tesseract with 30-second timeout
7. **Returns Result** - Responds with extracted text in JSON format

The service uses single-block page segmentation mode (PSM_SINGLE_BLOCK) for optimal text extraction from most common image layouts.

## ⚙️ Configuration

The `config.json` file allows customization:

```json
{
    "worker_pool_size": 4,      // Number of parallel OCR workers (defaults to CPU count)
    "queue_buffer_size": 50,    // Size of the job queue buffer
    "port": 30000,               // HTTP server port
    "max_image_size_mb": 10     // Maximum image file size in MB
}
```

**Default Behavior**: If `config.json` is missing or invalid, the service uses safe defaults:
- `worker_pool_size`: Number of available CPU cores
- `queue_buffer_size`: 100
- `port`: 30000
- `max_image_size_mb`: 10 MB

## 📝 Dependencies

### Go Dependencies
- `github.com/otiai10/gosseract/v2` - Go wrapper for Tesseract OCR

### System Dependencies
- Tesseract OCR engine (v5.x recommended)
- Tesseract language data (English included by default)

### Python Test Dependencies
- `requests` - HTTP library for Python

## 💡 Tips & Best Practices

- **Worker Pool Tuning**: Set `worker_pool_size` to match your CPU count for I/O-heavy tasks, or higher for CPU-heavy OCR workloads
- **Queue Buffer**: Increase `queue_buffer_size` if you expect burst traffic
- **Large Images**: The service processes images in memory. For very large images, ensure your server has sufficient RAM
- **File Size Limits**: Adjust `max_image_size_mb` based on your memory constraints
- **Performance**: First OCR request may take slightly longer as Tesseract initializes; subsequent requests are faster
- **Concurrency**: Each worker thread is OS-locked for maximum CGO stability
- **Language Support**: Additional language data can be installed via `tesseract-ocr-<lang>` packages

## 📄 License

No license specified. See the repository for more details.

## 🤝 Contributing

Contributions are welcome! Feel free to open issues and pull requests.

## 📧 Contact

Created by [@RedChlorine](https://github.com/RedChlorine)

---

**Last Updated**: 2026-05-18
