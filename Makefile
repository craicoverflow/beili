.PHONY: generate build dev test lint clean build-linux-amd64 build-linux-arm64

BINARY := bin/server
CMD     := ./cmd/server

generate:
	templ generate ./internal/templates/...

build: generate
	go build -o $(BINARY) $(CMD)

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server-linux-amd64 $(CMD)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server-linux-arm64 $(CMD)

dev:
	@which air > /dev/null 2>&1 || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

test:
	go test ./...

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found, run: brew install golangci-lint" && exit 1)
	golangci-lint run

clean:
	rm -rf bin/ tmp/
	find . -name '*_templ.go' -delete
