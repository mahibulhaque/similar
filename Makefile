GO ?= go

.PHONY: fmt lint test fuzz cover ci

fmt:
	$(GO) run mvdan.cc/gofumpt@latest -l -w .

lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...

test:
	$(GO) test ./... -race -cover

fuzz:
	$(GO) test ./internal/algorithms/ -run=xxx -fuzz=FuzzMyersInvariants -fuzztime=60s

cover:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

ci: test lint
	@echo "checking golden files are up to date (no -update)"
	$(GO) test . -run TestGolden
