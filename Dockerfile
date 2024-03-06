FROM golang:1.22.1-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go generate ./...
RUN go build -o thea .

FROM alpine:latest

COPY ./tests/test-config.toml /config.toml
COPY --from=builder /thea /thea

EXPOSE 8080
ENTRYPOINT ["/thea"]
