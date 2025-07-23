# ───────────────────────────
# Build a small stateless API
# ───────────────────────────
FROM golang:1.24-alpine AS builder

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" \
        -o /out/service-io ./cmd/service-io

# ───────────────────────────
FROM gcr.io/distroless/static

LABEL org.opencontainers.image.title="service-io" \
      org.opencontainers.image.description="Device onboarding (NATS + Docker)" \
      org.opencontainers.image.licenses="Apache-2.0"

WORKDIR /
COPY --from=builder /out/service-io /app/service-io

ENV NATS_URL=nats://nats-1:4222 \
    LISTEN_ADDR=:9090 \
    DEV_BUCKET=devices \
    PUBLISH_TIMEOUT_SEC=5 \
    ADAPTER_MAP_JSON='{"random":"adapter-rand:latest"}' \
    DO_REGISTRY_TOKEN=""

#USER nonroot:nonroot
ENTRYPOINT ["/app/service-io"]
