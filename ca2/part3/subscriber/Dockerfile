FROM golang:1.25-alpine

WORKDIR /app


ENV GOPROXY=https://goproxy.io,direct
ENV GOSUMDB=off

COPY subscriber/go.mod ./
COPY subscriber/main.go ./

RUN go mod tidy
RUN go build -o main .

CMD ["./main"]

