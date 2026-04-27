# ═══════════════════════════════════════════════════════════════
# Stage 1 — Build the Go binary
# ═══════════════════════════════════════════════════════════════
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies (git needed for go modules with VCS)
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -extldflags '-static'" \
    -o /build/smsgateway ./cmd/api

# ═══════════════════════════════════════════════════════════════
# Stage 2 — Minimal final image
# ═══════════════════════════════════════════════════════════════
FROM alpine:3.19

WORKDIR /app

# TLS certificates (needed for outbound HTTPS / Redis TLS)
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary
COPY --from=builder /build/smsgateway ./smsgateway

# Copy runtime assets
COPY --from=builder /build/views    ./views
COPY --from=builder /build/locales  ./locales

# Non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Configuration via environment variables
ENV PORT=3000 \
    REDIS_ADDR=redis:6379

EXPOSE 3000

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:3000/ || exit 1

ENTRYPOINT ["./smsgateway"]
