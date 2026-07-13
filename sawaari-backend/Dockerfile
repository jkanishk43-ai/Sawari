# ============================================
# Sawaari Backend - Multi-stage Go Build
# ============================================
# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build flags for optimization
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-s -w \
        -X main.Version=${VERSION} \
        -X main.CommitSHA=${COMMIT_SHA} \
        -X main.BuildDate=${BUILD_DATE}" \
    -o sawaari-backend \
    ./cmd/server

# ============================================
# Runtime stage
# ============================================
FROM alpine:3.19 AS runtime

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    wget \
    curl \
    && addgroup -S appgroup \
    && adduser -S appuser -G appgroup

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/sawaari-backend .

# Copy any additional assets if needed
COPY --from=builder /build/migrations /app/migrations

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment defaults
ENV GOMAXPROCS=2 \
    GOMEMLIMIT=800MiB

# Default command
ENTRYPOINT ["./sawaari-backend"]
CMD ["serve"]

# Build arguments documentation:
# VERSION=1.0.0
# COMMIT_SHA=$(git rev-parse --short HEAD)
# BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
