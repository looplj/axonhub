FROM node:20-alpine AS frontend-builder

WORKDIR /build
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile

COPY ./frontend .
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN pnpm build

FROM golang:alpine AS backend-builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /build/dist /build/internal/server/static

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

RUN go build -ldflags "-s -w -X 'github.com/looplj/axonhub/internal/build.Version=$(cat VERSION 2>/dev/null || echo dev)' -X 'github.com/looplj/axonhub/internal/build.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" -o axonhub ./cmd/axonhub

FROM alpine

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates

COPY --from=backend-builder /build/axonhub /
EXPOSE 8090
ENTRYPOINT ["/axonhub"]