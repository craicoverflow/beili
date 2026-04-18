ARG BUILD_FROM=alpine:3.21
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

RUN apk add --no-cache bash ca-certificates tzdata

COPY --from=builder /build/bin/server /usr/bin/server
COPY run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]
