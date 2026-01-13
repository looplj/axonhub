FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder

WORKDIR /build
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

COPY ./frontend .
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN pnpm build

# Copy dist to a stage with the target platform to avoid architecture mismatch
FROM alpine AS frontend-dist
COPY --from=frontend-builder /build/dist /dist

FROM golang:alpine AS backend-builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go mod download

COPY . .
COPY --from=frontend-dist /dist /build/internal/server/static/dist

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go build \
    -tags=nomsgpack \
    -ldflags "-s -w -X 'github.com/looplj/axonhub/internal/build.Version=$(cat internal/build/VERSION 2>/dev/null || echo dev)' -X 'github.com/looplj/axonhub/internal/build.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
    -o axonhub \
    ./cmd/axonhub

FROM alpine

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -s /bin/sh axonhub

WORKDIR /app
COPY --from=backend-builder /build/axonhub /app/axonhub

USER axonhub
EXPOSE 8090
ENTRYPOINT ["/app/axonhub"]