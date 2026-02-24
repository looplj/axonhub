FROM golang:alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go mod download

COPY . .

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go build \
    -ldflags "-s -w" \
    -o axonclaw \
    .

FROM alpine

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/axonclaw /app/axonclaw
COPY --from=builder /build/start.sh /app/start.sh
COPY --from=builder /build/stop.sh /app/stop.sh

RUN chmod +x /app/axonclaw /app/start.sh /app/stop.sh

RUN mkdir -p /app/.axonclaw/logs /app/.axonclaw/threads

ENV AXONCLAW_INSTANCE_ID="" \
    AXONHUB_BASE_URL="http://axonhub:8090" \
    AXONHUB_API_KEY="" \
    AXONCLAW_NAME="" \
    DEBUG_MODE=""

ENTRYPOINT ["/app/start.sh"]
