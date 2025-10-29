# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o stage ./cmd/stage

# Stage 2: Runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS requests (if needed)
RUN apk --no-cache add ca-certificates

# Create non-root user and group
RUN addgroup -g 1000 stage && \
    adduser -D -u 1000 -G stage stage

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/stage /app/stage

# Create directory for assets and set ownership
RUN mkdir -p /app/assets && \
    chown -R stage:stage /app

# Switch to non-root user
USER stage

# Expose default port
EXPOSE 8080

# Set default environment variables
ENV PORT=8080
ENV ASSET_DIR=/app/assets
ENV HOST=0.0.0.0
ENV GIN_MODE=release

# Run the server
ENTRYPOINT ["/app/stage"]
