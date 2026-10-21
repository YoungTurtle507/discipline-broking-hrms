FROM golang:1-alpine AS builder
WORKDIR /app

# Trust a local network's TLS-inspecting cert if present (dev-only, gitignored).
# Silently no-ops on machines/environments without this file.
COPY corporate-ca.crt.loca[l] /usr/local/share/ca-certificates/corporate-ca.crt
RUN [ -s /usr/local/share/ca-certificates/corporate-ca.crt ] && update-ca-certificates || true

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/bin/server .

EXPOSE 8080
ENTRYPOINT ["./server"]
