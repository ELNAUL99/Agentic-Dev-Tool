# Multi-stage build for go-agent
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o go-agent ./cmd/go-agent

# Runtime image with Docker-in-Docker support
FROM golang:1.23-alpine

RUN apk add --no-cache git docker-cli bash

WORKDIR /workspace
COPY --from=builder /build/go-agent /usr/local/bin/go-agent

ENTRYPOINT ["go-agent"]
