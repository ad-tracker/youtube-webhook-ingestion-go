# Build stage
FROM golang:1.25.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the migration tool
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" \
    -o migrate ./cmd/migrate

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/migrate /app/migrate

# Copy migrations directory
COPY --from=builder /build/migrations /app/migrations

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set environment variables
ENV PATH="/app:${PATH}"

# Expose any ports if needed (uncomment if your app needs it)
# EXPOSE 8080

# Default command
ENTRYPOINT ["/app/migrate"]
CMD ["--help"]
