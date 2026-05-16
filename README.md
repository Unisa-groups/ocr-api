# Squint

An OCR (Optical Character Recognition) microservice built with Go and Tesseract, designed to extract text from images with a simple HTTP API.

## 🚀 Features

- **HTTP REST API** - Simple endpoint to process images and extract text
- **In-Memory Processing** - Images are downloaded and processed in RAM without disk storage
- **Tesseract Integration** - Leverages the powerful open-source Tesseract OCR engine
- **Telegram Image Support** - Built to work with Telegram image URLs
- **Docker Ready** - Includes Docker and Docker Compose configuration for easy deployment
- **Comprehensive Error Handling** - Detailed error messages for debugging and monitoring

## 📋 Requirements

- Go 1.x or later
- Tesseract OCR engine
- Docker & Docker Compose (for containerized deployment)

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
   - **Ubuntu/Debian**: `sudo apt-get install tesseract-ocr`
   - **macOS**: `brew install tesseract`
   - **Windows**: Download from [GNU Tesseract OCR](https://github.com/UB-Mannheim/tesseract/wiki)

4. Run the service:
```bash
go run main.go
```

The service will start on `http://localhost:8080`

### Docker Deployment

1. Using Docker Compose:
```bash
docker-compose up
```

2. Or build and run with Docker directly:
```bash
docker build -t squint .
docker run -p 8080:8080 squint
```

## 📡 API Usage

### Endpoint

```
GET /api/v1/ocr?image_url=<url>
```

### Parameters

- `image_url` (required): URL of the image to process (supports Telegram image URLs)

### Request Example

```bash
curl "http://localhost:8080/api/v1/ocr?image_url=https://example.com/image.jpg"
```

### Response Example (Success)

```json
{
  "text": "The extracted text from the image",
  "status": "[SQUINT]: Success"
}
```

### Response Example (Error)

```json
{
  "status": "error",
  "error": "[SQUINT]: Missing image_url parameter"
}
```

## 🔍 Error Codes

| Status Code | Scenario |
|---|---|
| `200 OK` | OCR processing successful |
| `400 Bad Request` | Missing or invalid `image_url` parameter |
| `502 Bad Gateway` | Failed to download image from URL |
| `424 Failed Dependency` | Failed to read image stream |
| `500 Internal Server Error` | Tesseract OCR processing failed |

## 📦 Project Structure

```
Squint/
├── main.go                 # Main application code with HTTP handler
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── dockerfile              # Docker container configuration
├── docker-compose.yml      # Docker Compose setup
└── test_squint.py          # Python test script
```

## 🧪 Testing

A Python test script is included to validate the OCR service:

```bash
python test_squint.py
```

## 🏗️ How It Works

1. **Receives Request** - Client sends GET request with `image_url` parameter
2. **Downloads Image** - Service downloads the image directly into RAM
3. **Initializes Tesseract** - Creates and configures a Tesseract OCR client
4. **Processes Image** - Tesseract extracts text from the image bytes
5. **Returns Result** - Responds with extracted text in JSON format

The service uses single-block page segmentation mode for optimal text extraction from most common image layouts.

## 📝 Dependencies

- `github.com/otiai10/gosseract/v2` - Go wrapper for Tesseract OCR

## 📄 License

No license specified. See the repository for more details.

## 🤝 Contributing

Contributions are welcome! Feel free to open issues and pull requests.

## 📧 Contact

Created by [@RedChlorine](https://github.com/RedChlorine)

---

**Note**: This is a private repository. Please ensure you have appropriate permissions to access and use this code.
