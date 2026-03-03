FROM golang:alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY axon/ axon/
COPY cmd/axonclaw/ cmd/axonclaw/

WORKDIR /build/cmd/axonclaw

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go mod download

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

COPY --from=builder /build/cmd/axonclaw/axonclaw /app/axonclaw

RUN mkdir -p /app/.axonclaw/logs /app/.axonclaw/threads

ENV AXONCLAW_INSTANCE_ID="" \
    AXONCLAW_BASE_URL="http://axonhub:8090" \
    AXONCLAW_API_KEY="" \
    AXONCLAW_NAME="" \
    DEBUG_MODE=""

ENTRYPOINT ["/app/axonclaw"]
