FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o fidelity .

FROM scratch

WORKDIR /

COPY --from=builder /app/fidelity .

USER 10001

EXPOSE 8083

CMD ["./fidelity"]