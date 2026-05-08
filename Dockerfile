FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /sidecar ./cmd/main.go

FROM alpine:3.20

RUN apk --no-cache add ca-certificates

COPY --from=builder /sidecar /usr/local/bin/sidecar

EXPOSE 4221/udp 4224/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
