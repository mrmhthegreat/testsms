# SMS Gateway

A high-performance SMS Gateway microservice built with **GoFiber**, **Redis**, **HTMX**, **Alpine.js**, and **Tailwind CSS**.

## Features

- 🚀 **GoFiber** — ultra-fast HTTP server
- 📦 **Redis Queue** — async job processing via `BLPOP`
- 🌍 **Bilingual UI** — English (LTR) & Arabic (RTL) via cookie-based i18n
- 📱 **Smart Segmentation** — GSM-7 (160 chars) vs Unicode/Arabic (70 chars)
- ⚡ **HTMX Polling** — real-time status updates without JavaScript frameworks
- 🐳 **Docker** — multi-stage build for minimal images

## Project Structure

```
testsms/
├── cmd/api/main.go             # Entry point & route handlers
├── internal/
│   ├── sms/
│   │   ├── service.go          # Segment calc, UUID, queue dispatch
│   │   ├── repository.go       # Redis state storage
│   │   └── worker.go           # Background delivery worker
│   └── i18n/
│       ├── loader.go           # YAML locale loader
│       └── middleware.go       # Fiber lang middleware (cookie/query)
├── pkg/queue/redis.go          # Redis client wrapper
├── locales/
│   ├── en.yaml                 # English translations
│   └── ar.yaml                 # Arabic translations
├── views/index.html            # HTMX + Alpine.js + Tailwind dashboard
├── Dockerfile                  # Multi-stage Docker build
└── README.md
```

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Redis 7+](https://redis.io/)
- Docker (optional)

## Quick Start

### 1. Clone & install

```bash
git clone <repo>
cd testsms
go mod tidy
```

### 2. Start Redis

```bash
redis-server
# or with Docker:
docker run -d -p 6379:6379 redis:7-alpine
```

### 3. Run the server

```bash
go run ./cmd/api/main.go
```

Open **http://localhost:3000**

### 4. Test via cURL

```bash
# Send an English SMS
curl -X POST http://localhost:3000/send-sms \
  -d "phone=%2B1234567890&message=Hello+World"

# Send an Arabic SMS
curl -X POST http://localhost:3000/send-sms \
  -d "phone=%2B966501234567&message=%D9%85%D8%B1%D8%AD%D8%A8%D8%A7"

# Poll status (replace <uuid> with the ID from the response)
curl http://localhost:3000/status/<uuid>
```

### 5. Switch language

```
http://localhost:3000/?lang=ar    # Arabic / RTL
http://localhost:3000/?lang=en    # English / LTR
```

## Docker

```bash
# Build
docker build -t smsgateway .

# Run (with Redis)
docker network create smsnet
docker run -d --name redis --network smsnet redis:7-alpine
docker run -d --name smsgateway \
  --network smsnet \
  -p 3000:3000 \
  -e REDIS_ADDR=redis:6379 \
  smsgateway
```

Or use Docker Compose:

```yaml
version: "3.9"
services:
  redis:
    image: redis:7-alpine
  smsgateway:
    build: .
    ports: ["3000:3000"]
    environment:
      REDIS_ADDR: redis:6379
    depends_on: [redis]
```

## SMS Segmentation Logic

| Encoding | 1 Segment | Multi-Part |
|----------|-----------|------------|
| GSM-7    | 160 chars | 153 chars  |
| Unicode  | 70 chars  | 67 chars   |

A message uses **Unicode** encoding if it contains any character with code point > 127 (Arabic, Chinese, emoji, etc.).

## Worker Flow

```
POST /send-sms
  └─> Redis RPUSH sms_queue <uuid>
        └─> Worker BLPOP
              ├─ sleep 1s ─> status: Sending
              └─ sleep 2s ─> status: Delivered
```

HTMX polls `GET /status/:id` every second and stops when the response contains `Delivered`.

## Environment Variables

| Variable     | Default         | Description          |
|-------------|-----------------|----------------------|
| `PORT`      | `3000`          | HTTP listen port     |
| `REDIS_ADDR`| `localhost:6379`| Redis address        |
