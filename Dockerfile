FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o sse-gateway .

FROM scratch

COPY --from=builder /app/sse-gateway /sse-gateway

EXPOSE 8080

ENTRYPOINT ["/sse-gateway"]
