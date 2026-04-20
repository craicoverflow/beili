.PHONY: generate build dev seed test lint clean build-linux-amd64 build-linux-arm64 docker-build release

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

seed:
	go run ./cmd/seed

test:
	go test ./...

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found, run: brew install golangci-lint" && exit 1)
	golangci-lint run

docker-build:
	docker build -f addon/Dockerfile -t my-recipe-manager:latest .

clean:
	rm -rf bin/ tmp/
	find . -name '*_templ.go' -delete

## Usage: make release VERSION=1.2.1
release:
	@[ "$(VERSION)" ] || ( echo "Usage: make release VERSION=x.y.z"; exit 1 )
	sed -i '' 's/version: "[^"]*"/version: "$(VERSION)"/' addon/config.yaml
	git add addon/config.yaml
	git commit -m "chore: bump version to $(VERSION)"
	git tag v$(VERSION)
	git push origin main
	git push origin v$(VERSION)
