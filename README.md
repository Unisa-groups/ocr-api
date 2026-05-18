# Squint

An OCR (Optical Character Recognition) microservice built with Go and Tesseract, designed to extract text from images with a simple HTTP API.

## 🎯 Project Info

- **Primary Language**: Go (71.5%)
- **Secondary Language**: Python (28.5%)
- **Type**: Microservice
- **API**: RESTful HTTP interface

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

- `image_url` (required): URL of the image to process (supports Telegram image URLs, HTTP/HTTPS URLs, and other public image URLs)

### Request Examples

**Basic usage with a public image URL:**
```bash
curl "http://localhost:8080/api/v1/ocr?image_url=https://example.com/image.jpg"
```

**With a Telegram image URL:**
```bash
curl "http://localhost:8080/api/v1/ocr?image_url=https://t.me/channel/message"
```

**Using JavaScript/Fetch:**
```javascript
const imageUrl = 'https://example.com/image.jpg';
fetch(`http://localhost:8080/api/v1/ocr?image_url=${encodeURIComponent(imageUrl)}`)
  .then(response => response.json())
  .then(data => console.log('Extracted text:', data.text))
  .catch(error => console.error('Error:', error));
```

**Using Python:**
```python
import requests

image_url = 'https://example.com/image.jpg'
response = requests.get('http://localhost:8080/api/v1/ocr', params={'image_url': image_url})
data = response.json()
print('Extracted text:', data['text'])
```

### Response Example (Success)

```json
{
  "text": "The extracted text from the image",
  "status": "[SQUINT]: Success"
}
```

### Response Example (Error - Missing Parameter)

```json
{
  "status": "error",
  "error": "[SQUINT]: Missing image_url parameter"
}
```

### Response Example (Error - Download Failed)

```json
{
  "status": "error",
  "error": "[SQUINT]: Failed to download image from URL"
}
```

## 🔍 HTTP Status Codes

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
├── test_squint.py          # Python test script
└── README.md               # This file
```

## 🧪 Testing

A Python test script is included to validate the OCR service:

```bash
# Install test dependencies (if needed)
pip install requests

# Run the test script
python test_squint.py
```

The test script will perform several requests to verify the OCR service is working correctly.

## 🏗️ How It Works

1. **Receives Request** - Client sends GET request with `image_url` parameter
2. **Validates Input** - Ensures the `image_url` parameter is provided
3. **Downloads Image** - Service downloads the image directly into RAM
4. **Initializes Tesseract** - Creates and configures a Tesseract OCR client
5. **Processes Image** - Tesseract extracts text from the image bytes
6. **Returns Result** - Responds with extracted text in JSON format

The service uses single-block page segmentation mode for optimal text extraction from most common image layouts.

## 📝 Dependencies

### Go Dependencies
- `github.com/otiai10/gosseract/v2` - Go wrapper for Tesseract OCR

### System Dependencies
- Tesseract OCR engine

### Python Test Dependencies
- `requests` - HTTP library for Python

## 💡 Tips & Best Practices

- **Large Images**: The service processes images in memory. For very large images, ensure your server has sufficient RAM.
- **Timeout Handling**: Consider setting timeouts on the client side for slow image downloads.
- **URL Encoding**: Make sure to properly URL-encode the `image_url` parameter if it contains special characters.
- **Supported Formats**: Works with JPEG, PNG, GIF, BMP, TIFF, and other common image formats supported by Tesseract.
- **Performance**: First request may take longer as Tesseract initializes; subsequent requests are faster.

## 📄 License

No license specified. See the repository for more details.

## 🤝 Contributing

Contributions are welcome! Feel free to open issues and pull requests.

## 📧 Contact

Created by [@RedChlorine](https://github.com/RedChlorine)

---

**Note**: This is a private repository. Please ensure you have appropriate permissions to access and use this code.
