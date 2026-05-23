# =============================================================================
# Paw Diary - Multi-stage Dockerfile for Render deployment
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Build Go backend
# -----------------------------------------------------------------------------
FROM golang:1.21-alpine AS go-builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# -----------------------------------------------------------------------------
# Stage 2: Build Vue frontend
# -----------------------------------------------------------------------------
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files first for better caching
COPY frontend/package*.json ./
RUN npm ci --silent

# Copy frontend source and build
COPY frontend/ ./
RUN npm run build

# -----------------------------------------------------------------------------
# Stage 3: Final runtime image
# Use Ubuntu 24.04 for up-to-date Intel VAAPI driver (xe kernel driver support)
# -----------------------------------------------------------------------------
FROM ubuntu:26.04

ENV DEBIAN_FRONTEND=noninteractive

# Install FFmpeg, Intel VAAPI driver (non-free, supports xe/Arrow Lake), and fonts
RUN apt-get update && apt-get install -y \
    ffmpeg \
    ca-certificates \
    tzdata \
    wget \
    fonts-noto-cjk \
    intel-media-va-driver-non-free \
    libva-drm2 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Set timezone
ENV TZ=Asia/Taipei

WORKDIR /app

# Copy Go binary from builder
COPY --from=go-builder /app/main .

# Copy frontend dist from builder
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Copy BGM assets
COPY "assets_bgm" "./assets_bgm"

# Create storage directories
RUN mkdir -p storage/videos storage/projects storage/frames storage/highlights

# Set default environment variables
ENV PORT=8080
ENV STORAGE_PATH=./storage

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -sf http://localhost:8080/api/health || exit 1

# Run the application
CMD ["./main"]
