GO ?= go
VERSION ?= 1.4.0

.PHONY: build test check
build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w -X main.version=$(VERSION)' -o bin/foodie ./cmd/foodie

test:
	$(GO) test -race ./...

check:
	$(GO) vet ./...
	$(GO) mod verify

.PHONY: hooks
hooks:
	git config core.hooksPath .githooks
