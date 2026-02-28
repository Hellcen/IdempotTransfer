FROM golang:1.24-alpine AS builder 

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o idempot-api ./cmd/api

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/idempot-api .
COPY --from=builder /app/internal/config ./internal/config
COPY --from=builder /app/internal/repository/migration ./internal/repository/migration

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

CMD ["./idempot-api"]