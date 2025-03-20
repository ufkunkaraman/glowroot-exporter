# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o glowroot-exporter

# Final stage
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/glowroot-exporter .
EXPOSE 9101
ENTRYPOINT ["./glowroot-exporter"]
