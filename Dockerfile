# {{CODE-Cycle-Integration:
#   Task_ID: #T001
#   Timestamp: 2025-12-29T20:35:00Z
#   Phase: D-Develop
#   Context-Analysis: "优化 Dockerfile,添加健康检查,优化缓存层,改进安全性"
#   Principle_Applied: "Aether-Engineering-SOLID, KISS, Security-First"
# }}
# {{START_MODIFICATIONS}}
FROM node:20-alpine AS frontend-builder

WORKDIR /build
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile

COPY ./frontend .
ENV NODE_OPTIONS="--max-old-space-size=8192"
RUN pnpm build

FROM golang:1.25-alpine AS backend-builder

ARG TARGETARCH=amd64

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /build/dist /build/internal/server/static/dist

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH}

ARG VERSION=dev
ARG BUILD_TIME
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    go build -tags=nomsgpack -ldflags "-s -w -X 'github.com/looplj/axonhub/internal/build.Version=${VERSION}' -X 'github.com/looplj/axonhub/internal/build.BuildTime=${BUILD_TIME}'" -o axonhub ./cmd/axonhub

FROM alpine:3.19

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata wget curl \
    && update-ca-certificates \
    && adduser -D -s /bin/sh -u 1000 axonhub

WORKDIR /app
COPY --from=backend-builder /build/axonhub /app/axonhub

RUN chown -R axonhub:axonhub /app

USER axonhub
EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

ENTRYPOINT ["/app/axonhub"]
# {{END_MODIFICATIONS}}
