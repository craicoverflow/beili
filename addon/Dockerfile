ARG BUILD_FROM
FROM golang:1.25-alpine AS builder

ARG BUILD_VERSION=dev

RUN apk add --no-cache git

WORKDIR /build

RUN git clone --depth=1 --branch "v${BUILD_VERSION}" https://github.com/craicoverflow/beili.git .

RUN go install github.com/a-h/templ/cmd/templ@latest

RUN go mod download

RUN templ generate ./internal/templates/... && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/server ./cmd/server

# ----------------------------------------------------------------
FROM $BUILD_FROM

COPY --from=builder /build/bin/server /usr/bin/server
RUN mkdir -p /etc/services.d/beili
COPY run.sh /etc/services.d/beili/run
RUN chmod +x /etc/services.d/beili/run
