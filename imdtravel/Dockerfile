FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o imdtravel .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/imdtravel .

EXPOSE 8080

CMD ["./imdtravel"]