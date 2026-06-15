BINARY := wf
COVERAGE := coverage.out

.PHONY: build test test-coverage lint clean

build: ## Build the wf binary
	go build -o $(BINARY) ./cmd/wf

test: ## Run all tests
	go test ./...

test-coverage: ## Run all tests and display coverage
	go test -coverprofile=$(COVERAGE) ./...
	go tool cover -func=$(COVERAGE)

lint: ## Run golangci-lint
	golangci-lint run

clean: ## Remove build and coverage artifacts
	rm -f $(BINARY) $(COVERAGE)
