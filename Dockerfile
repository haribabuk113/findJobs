##
## Build stage
##
FROM golang:1.27.3-alpine AS builder

WORKDIR /app

# git for modules fetched directly from VCS; no C toolchain (CGO is off)
RUN apk add --no-cache git

# Copy go mod files first (to leverage Docker cache)
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Static binary without local paths or debug symbols
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o findJobs ./cmd/main.go

##
## Run stage
##
FROM alpine:3.24.3

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/findJobs /app/findJobs

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup \
    && mkdir -p /app/uploads && chown appuser:appgroup /app/uploads
USER appuser

EXPOSE 8081

# Liveness only: a DB outage should mark the app not-ready (/readyz), not get it restarted.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${SERVER_PORT:-8081}/healthz" || exit 1

ENTRYPOINT ["/app/findJobs"]
