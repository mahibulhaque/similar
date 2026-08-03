GO ?= go

.PHONY: fmt lint test fuzz fuzz-myers fuzz-lcs cover ci

FUZZTIME ?= 60s

fmt:
	$(GO) run mvdan.cc/gofumpt@latest -l -w .

lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...

test:
	$(GO) test ./... -race -cover

# -fuzz takes one target, so each algorithm gets its own run.
fuzz: fuzz-myers fuzz-lcs

fuzz-myers:
	$(GO) test ./internal/algorithms/ -run=xxx -fuzz=FuzzMyersInvariants -fuzztime=$(FUZZTIME)

fuzz-lcs:
	$(GO) test ./internal/algorithms/ -run=xxx -fuzz=FuzzLCSInvariants -fuzztime=$(FUZZTIME)

cover:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

ci: test lint
	@echo "checking golden files are up to date (no -update)"
	$(GO) test . -run 'Golden|TestPublicAPI'
