.PHONY: build test vet fmt fmt-check lint check fuzz

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

# Fails and lists the files when gofmt would change any; rewrites nothing.
fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt would change:"; echo "$$out"; exit 1; fi

# Needs golangci-lint (config in .golangci.yml). Nothing is installed for you.
lint:
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not found: see https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

# Format check, vet and tests, in that order. Run `make lint` as well for what CI runs.
check: fmt-check vet test

# Each Fuzz target runs for FUZZTIME. `go test` alone only replays the seeds.
FUZZTIME ?= 30s
fuzz:
	go test ./cmd/draincheck -run '^$$' -fuzz '^FuzzOptionsValidate$$' -fuzztime $(FUZZTIME)
	go test ./cmd/draincheck -run '^$$' -fuzz '^FuzzDrainOptionsPodSelector$$' -fuzztime $(FUZZTIME)
	go test ./internal/findings -run '^$$' -fuzz '^FuzzSortIgnoresInputOrder$$' -fuzztime $(FUZZTIME)
