FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o airlineshub .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/airlineshub .

EXPOSE 8081

CMD ["./airlineshub"]