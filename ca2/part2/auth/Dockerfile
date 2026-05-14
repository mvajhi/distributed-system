FROM golang:alpine AS builder

WORKDIR /app

COPY grpc/go.mod grpc/go.sum ./grpc/
COPY auth/go.mod auth/go.sum ./auth/

WORKDIR /app/auth

RUN go mod download

WORKDIR /app

COPY grpc/ ./grpc/
COPY auth/ ./auth/

WORKDIR /app/auth

RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /auth-service .

EXPOSE 50051

CMD ["./auth-service"]