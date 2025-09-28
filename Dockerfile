# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o gogocoin ./cmd/gogocoin

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata sqlite

WORKDIR /app

# Create non-root user
RUN addgroup -g 1001 -S gogocoin && \
    adduser -u 1001 -S gogocoin -G gogocoin

# Create necessary directories
RUN mkdir -p /app/data /app/logs /app/configs /app/web && \
    chown -R gogocoin:gogocoin /app

# Copy binary from builder stage
COPY --from=builder /app/gogocoin .

# Copy configuration and web files
COPY --chown=gogocoin:gogocoin configs/ ./configs/
COPY --chown=gogocoin:gogocoin web/ ./web/

# Switch to non-root user
USER gogocoin

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/status || exit 1

# Default command
CMD ["./gogocoin", "run-paper"]
