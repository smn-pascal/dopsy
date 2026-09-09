GO ?= go
PNPM ?= pnpm

.PHONY: dev dev-api dev-web build test fmt docs

dev:
	@echo "Run 'make dev-api' and 'make dev-web' in separate terminals."

dev-api:
	DOPSY_DEMO_MODE=true DOPSY_ADDR=127.0.0.1:3001 $(GO) run ./cmd/dopsy

dev-web:
	$(PNPM) dev:web

build:
	$(PNPM) build:web
	$(GO) build -o bin/dopsy ./cmd/dopsy
	$(GO) build -o bin/dopsy-proxy ./cmd/dopsy-proxy

test:
	$(GO) test ./...
	$(PNPM) test:web

fmt:
	$(GO) fmt ./...
	$(PNPM) -r format

docs:
	$(PNPM) docs:dev
