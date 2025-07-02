FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /wallet-api ./cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /wallet-api .
COPY --from=builder /app/migrations ./migrations

COPY config.env .

EXPOSE 8080

CMD ["./wallet-api"]