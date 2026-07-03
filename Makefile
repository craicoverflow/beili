.PHONY: generate css build dev seed test lint clean build-linux-amd64 build-linux-arm64 docker-build release

BINARY := bin/server
CMD     := ./cmd/server
TAILWIND_CLI := $(shell command -v tailwindcss 2> /dev/null)

generate:
	templ generate ./internal/templates/...

## Rebuilds static/app.css from tailwind/input.css + tailwind.config.js.
## Requires templ-generated *_templ.go files (content glob scans them) — run after generate.
## Uses the standalone Tailwind v3 CLI: https://github.com/tailwindlabs/tailwindcss/releases
css: generate
	@if [ -z "$(TAILWIND_CLI)" ]; then echo "tailwindcss CLI not found on PATH — install the standalone binary (see Makefile comment)"; exit 1; fi
	$(TAILWIND_CLI) -i tailwind/input.css -o static/app.css --minify

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
	@VERSION=$$(awk -F'"' '/^version:/ {print $$2}' addon/config.yaml); \
	echo "Building with BUILD_VERSION=$$VERSION"; \
	docker build --build-arg BUILD_VERSION=$$VERSION -f addon/Dockerfile -t my-recipe-manager:latest .

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
