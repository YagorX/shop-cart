FROM golang:1.25 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/cart-service ./cmd/cart

FROM alpine:3.20
WORKDIR /app

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /out/cart-service /app/cart-service
COPY config/config.docker.yaml /app/config/config.docker.yaml

USER app

EXPOSE 8084 9092

ENV CONFIG_PATH=/app/config/config.docker.yaml

ENTRYPOINT ["/app/cart-service"]
