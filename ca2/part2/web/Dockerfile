FROM golang:alpine AS builder

WORKDIR /app

COPY grpc/go.mod grpc/go.sum ./grpc/
COPY web/go.mod web/go.sum ./web/

WORKDIR /app/web

RUN go mod download

WORKDIR /app

COPY grpc/ ./grpc/
COPY web/ ./web/

WORKDIR /app/web

RUN CGO_ENABLED=0 GOOS=linux go build -o /web-service main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /web-service .
COPY web/templates/ ./templates/

EXPOSE 8080

CMD ["./web-service"]