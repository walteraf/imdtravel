FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o exchange .

FROM scratch

WORKDIR /

COPY --from=builder /app/exchange .

USER 10001

EXPOSE 8082

CMD ["./exchange"]