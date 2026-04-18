ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

RUN git clone --depth=1 https://github.com/craicoverflow/beili.git .

RUN go install github.com/a-h/templ/cmd/templ@latest

RUN go mod download

RUN templ generate ./internal/templates/... && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/server ./cmd/server

# ----------------------------------------------------------------
FROM $BUILD_FROM

COPY --from=builder /build/bin/server /usr/bin/server

# Register as an s6 service (works with s6-overlay v2 and v3 compat layer)
COPY run.sh /etc/services.d/beili/run
RUN chmod +x /etc/services.d/beili/run
