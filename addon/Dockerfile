ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install templ code generator
RUN go install github.com/a-h/templ/cmd/templ@latest

# Fetch dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate templ files then compile
RUN templ generate ./internal/templates/... && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/server ./cmd/server

# ----------------------------------------------------------------
FROM $BUILD_FROM

# Copy the binary
COPY --from=builder /build/bin/server /usr/bin/server

# Copy the addon entrypoint
COPY addon/run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]
