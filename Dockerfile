# Builds the standalone server image from the working tree. The Home
# Assistant add-on image lives in addon/Dockerfile and is unrelated.
# The builder runs on the host architecture and cross-compiles, so an amd64
# image built on an arm64 machine does not run Go under QEMU (segfaults).
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN templ generate ./internal/templates/... && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildvcs=false -ldflags="-s -w" -o bin/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
COPY --from=builder /build/bin/server /usr/bin/server
ENV PORT=8080 DATA_DIR=/data
VOLUME /data
EXPOSE 8080
CMD ["/usr/bin/server"]
