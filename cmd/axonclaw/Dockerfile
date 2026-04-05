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

ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go build \
    -ldflags "-X github.com/looplj/axonhub/cmd/axonclaw/build.Version=${VERSION} \
              -X github.com/looplj/axonhub/cmd/axonclaw/build.BuildTime=${BUILD_TIME} \
              -X github.com/looplj/axonhub/cmd/axonclaw/build.GitCommit=${GIT_COMMIT} \
              -s -w" \
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
    AXONCLAW_AUTO_SYNC_CONFIG="" \
    DEBUG_MODE=""

ENTRYPOINT ["/app/axonclaw"]
