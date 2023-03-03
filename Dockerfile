FROM golang:1.19.5-alpine3.17 as go-builder

RUN apk add build-base

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY *.go ./
ARG GOOS=linux
ARG GOARCH=amd64
ARG VERSION 0.0.0
RUN go build -ldflags "-s -w -X main.Version=$VERSION -X main.Build=$(date +%s)"

FROM alpine:3.17

WORKDIR /app

COPY --from=go-builder /app/streams /app/streams

ENTRYPOINT [ "/app/streams" ]