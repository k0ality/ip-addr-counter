# Multi-stage build for minimal image size
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod ./

# Copy source code
COPY main.go ./

# Build the application
# CGO_ENABLED=0 for static binary
# -ldflags="-s -w" to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ip-counter main.go

# Final stage - minimal image
FROM alpine:latest

# Install ca-certificates for HTTPS (if needed in future)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ip-counter .

# Create a directory for input files
RUN mkdir -p /data

# Set the binary as entrypoint
ENTRYPOINT ["/app/ip-counter"]

# Default command shows help
CMD ["/data/input.txt", "14"]