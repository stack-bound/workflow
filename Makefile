BINARY := wf
COVERAGE := coverage.out
# Where `make install` puts the binary. Mirrors install.sh: defaults to
# ~/.local/bin and honours an INSTALL_DIR override, so a local dev build lands in
# the same place the README install does (no duplicate wf on PATH).
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: build install test test-coverage lint clean docs docs-build docs-deploy

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

docs: ## Run the documentation site locally (VitePress dev server)
	cd docs && npm install && npm run dev

docs-build: ## Build the static documentation site (output: docs/.vitepress/dist)
	cd docs && npm ci && npm run build

docs-deploy: ## Build and deploy the docs to Cloudflare Pages (needs wrangler auth)
	cd docs && npm run deploy
