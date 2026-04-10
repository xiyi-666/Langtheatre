FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o linguaquest-api ./cmd/server

FROM alpine:3.20
WORKDIR /srv
COPY --from=builder /app/linguaquest-api /srv/linguaquest-api
EXPOSE 8080
CMD ["/srv/linguaquest-api"]
