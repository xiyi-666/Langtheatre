# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOMAXPROCS=2

COPY go.mod go.sum* ./
RUN --mount=type=cache,id=linguaquest-go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,id=linguaquest-go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=linguaquest-go-build,target=/root/.cache/go-build,sharing=locked \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -v -p=1 -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/linguaquest-api ./cmd/server

FROM alpine:3.20
WORKDIR /srv
COPY --from=builder /out/linguaquest-api /srv/linguaquest-api
COPY --from=builder /app/migrations /srv/migrations
EXPOSE 8177
CMD ["/srv/linguaquest-api"]
