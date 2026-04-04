# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev sqlite-dev

WORKDIR /app

# Copy root module files first (example/ depends on the parent module via replace)
COPY go.mod go.sum ./
RUN go mod download

# Copy example module files
COPY example/go.mod example/go.sum ./example/

# Copy all source code
COPY . .

# Build the example application
WORKDIR /app/example
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o /app/gogocoin ./cmd/

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata sqlite curl

# Set timezone to Asia/Tokyo
ENV TZ=Asia/Tokyo

WORKDIR /app

# Create non-root user
RUN addgroup -g 1001 -S gogocoin && \
    adduser -u 1001 -S gogocoin -G gogocoin

# Create necessary directories
RUN mkdir -p /app/data /app/logs /app/configs /app/web && \
    chown -R gogocoin:gogocoin /app

# Copy binary from builder stage
COPY --from=builder /app/gogocoin .

# Copy config and web assets from the root context
COPY --chown=gogocoin:gogocoin example/configs/ ./configs/
COPY --chown=gogocoin:gogocoin web/ ./web/

# Switch to non-root user
USER gogocoin

# Expose port
EXPOSE 8080
