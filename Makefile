GO ?= go
BINARY ?= bin/ryxoui
WEB_DIR ?= web
DIST_DIR ?= internal/web/dist

.PHONY: all build dev test race vet fmt lint tidy web web-install clean migrate run

all: fmt vet test build

build:
	$(GO) build -trimpath -ldflags "-s -w -X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o $(BINARY) ./cmd/ryxoui

run:
	$(GO) run ./cmd/ryxoui serve --db ryxoui.db

test:
	$(GO) test ./...

race:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet
	@gofmt -l . | grep -v '^$$' && (echo "gofmt found unformatted files"; exit 1) || echo "gofmt clean"

tidy:
	$(GO) mod tidy

web-install:
	cd $(WEB_DIR) && npm ci || (cd $(WEB_DIR) && npm install)

web:
	cd $(WEB_DIR) && npm run build

migrate:
	$(GO) run ./cmd/ryxoui migrate --db ryxoui.db

clean:
	rm -rf $(BINARY) $(DIST_DIR)/assets
