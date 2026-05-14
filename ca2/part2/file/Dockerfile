FROM golang:alpine AS builder

WORKDIR /app

COPY grpc/go.mod grpc/go.sum ./grpc/
COPY file/go.mod file/go.sum ./file/

WORKDIR /app/file

RUN go mod download

WORKDIR /app

COPY grpc/ ./grpc/
COPY file/ ./file/

WORKDIR /app/file

RUN CGO_ENABLED=0 GOOS=linux go build -o /file-service main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /file-service .
COPY file/files ./files

EXPOSE 50052

CMD ["./file-service"]