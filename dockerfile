# --- Stage 1: Build Environment ---
FROM golang:1.26-bookworm AS builder

# Force apt to refresh indexes completely and handle stale caches aggressively
RUN apt-get clean && \
    apt-get update --allow-releaseinfo-change && \
    apt-get install -y --no-install-recommends \
    libtesseract-dev \
    tesseract-ocr \
    g++ \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy Go dependency manifests first to leverage caching
COPY go.mod go.sum ./
RUN go mod download

# FIX: Kept strictly on a single line to avoid argument syntax errors
COPY . .

# Build the binary with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o squint main.go

# --- Stage 2: Final Runtime ---
FROM debian:bookworm-slim

# Apply the identical robust package manager fixes to the runtime layer
RUN apt-get clean && \
    apt-get update --allow-releaseinfo-change && \
    apt-get install -y --no-install-recommends \
    tesseract-ocr \
    tesseract-ocr-eng \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Point explicitly to the Tesseract 5 folder path inside Debian Bookworm
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata/

ENV OMP_THREAD_LIMIT=1

WORKDIR /app

# Bring over the compiled binary from Stage 1
COPY --from=builder /app/squint .

EXPOSE 8080

CMD ["./squint"]
