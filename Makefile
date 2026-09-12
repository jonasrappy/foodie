GO ?= go
VERSION ?= 1.4.0

.PHONY: build test test-browser test-android check clean
build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w -X main.version=$(VERSION)' -o bin/foodie ./cmd/foodie

test:
	GO="$(GO)" python3 test/go/run.py test -race ./...

check:
	GO="$(GO)" python3 test/go/run.py vet ./...
	$(GO) mod verify

.PHONY: hooks
hooks:
	git config core.hooksPath .githooks

test-browser: build
	python3 test/browser/run.py

test-android:
	python3 test/android/run.py

# Remove reproducible build output. The running server and published APK stay in place.
clean:
	rm -rf android/build scripts/__pycache__ test/browser/__pycache__ test/go/__pycache__ test/android/__pycache__
