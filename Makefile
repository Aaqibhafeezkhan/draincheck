.PHONY: build test vet fmt fuzz

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

# Each Fuzz target runs for FUZZTIME. `go test` alone only replays the seeds.
FUZZTIME ?= 30s
fuzz:
	go test ./cmd/draincheck -run '^$$' -fuzz '^FuzzOptionsValidate$$' -fuzztime $(FUZZTIME)
	go test ./cmd/draincheck -run '^$$' -fuzz '^FuzzDrainOptionsPodSelector$$' -fuzztime $(FUZZTIME)
	go test ./internal/findings -run '^$$' -fuzz '^FuzzSortIgnoresInputOrder$$' -fuzztime $(FUZZTIME)
