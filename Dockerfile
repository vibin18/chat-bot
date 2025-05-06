############
# Build stage
############
FROM golang:1.24.2-alpine AS builder

# Install git and certificates for dependency fetching
RUN apk add --no-cache git ca-certificates tzdata curl && update-ca-certificates

# Create appuser
ENV USER=appuser
ENV UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

# Set Go environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Fix dependency issues and download dependencies
RUN go mod tidy && \
    go mod download -x

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /go/bin/chat-bot ./cmd/server

############
# Run stage
############
FROM alpine:3.17

# Import user and group from builder
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy built executable
COPY --from=builder /go/bin/chat-bot /app/chat-bot

# Create directory for config and data
RUN mkdir -p /app/config /app/data /app/web && \
    chown -R appuser:appuser /app

# Copy web files (templates, static assets, etc.)
COPY --chown=appuser:appuser web/ /app/web/

# Copy config sample (will be overridden by volume mount in production)
COPY --chown=appuser:appuser config/config.sample.json /app/config/config.json

# Set working directory
WORKDIR /app

# Use non-root user
USER appuser

# Expose HTTP port
EXPOSE 8080

# Run application
ENTRYPOINT ["/app/chat-bot"]
CMD ["--config=/app/config/config.json"]
