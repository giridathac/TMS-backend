# ─── Stage 1: Build the Go binary ────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/main.go

# ─── Stage 2: Minimal runtime ───────────────────────────────────────────────
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/docs ./docs
COPY --from=builder /app/templates ./templates

EXPOSE 8080
CMD ["./server"]