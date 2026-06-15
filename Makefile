BINARY := wf
COVERAGE := coverage.out
# Where `make install` puts the binary. Mirrors install.sh: defaults to
# ~/.local/bin and honours an INSTALL_DIR override, so a local dev build lands in
# the same place the README install does (no duplicate wf on PATH).
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: build install test test-coverage lint clean

build: ## Build the wf binary (stamped with a -dev version + short commit)
	go build -o $(BINARY) ./cmd/wf

install: ## Build and install wf to INSTALL_DIR (default ~/.local/bin, as install.sh)
	mkdir -p "$(INSTALL_DIR)"
	go build -o "$(INSTALL_DIR)/$(BINARY)" ./cmd/wf
	@echo "$(BINARY) installed to $(INSTALL_DIR)/$(BINARY)"

test: ## Run all tests
	go test ./...

test-coverage: ## Run all tests and display coverage
	go test -coverprofile=$(COVERAGE) ./...
	go tool cover -func=$(COVERAGE)

lint: ## Run golangci-lint
	golangci-lint run

clean: ## Remove build and coverage artifacts
	rm -f $(BINARY) $(COVERAGE)
