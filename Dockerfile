# Build Stage
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary (no dependency on system libc)
RUN CGO_ENABLED=0 GOOS=linux go build -o ai-meal-planner ./cmd/ai-meal-planner

# Runtime Stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests (needed for Ghost/Gemini APIs)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/ai-meal-planner .

# Create data directory for recipe storage
RUN mkdir -p data/recipes

# Set environment variable defaults (override these at runtime)
ENV GHOST_API_URL=""
ENV GHOST_CONTENT_KEY=""
ENV GEMINI_API_KEY=""

# Default command (can be overridden, e.g., "ingest" or "plan")
ENTRYPOINT ["./ai-meal-planner"]
