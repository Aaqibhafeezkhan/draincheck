.PHONY: build test vet fmt

VERSION ?= dev
BIN     := bin/draincheck

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) ./cmd/draincheck

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
