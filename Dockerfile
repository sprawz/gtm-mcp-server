# Build stage (using Google mirror to avoid Docker Hub rate limits)
FROM mirror.gcr.io/library/golang:1.25-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gtm-mcp-server .

# Runtime stage (using Google mirror)
FROM mirror.gcr.io/library/alpine:3.21

WORKDIR /app

# Install ca-certificates for HTTPS requests to Google APIs
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/gtm-mcp-server .

# Create non-root user and a writable data dir for the token store.
# A named volume mounted at /data inherits this ownership, so the app can
# persist tokens while still running as a non-root user.
RUN adduser -D -g '' appuser && mkdir -p /data && chown appuser:appuser /data
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the server
CMD ["./gtm-mcp-server"]
