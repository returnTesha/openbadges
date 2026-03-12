FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /bin/server /app/server
COPY config/config.toml /app/config/config.toml
COPY migrations/ /app/migrations/

EXPOSE 8080
CMD ["/app/server"]
