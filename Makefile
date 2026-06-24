BINARY_NAME=kubeaccess
SERVER_BIN=kubeaccess-server

.PHONY: all build build-server build-all clean run run-server fmt vet lint test test-e2e
.PHONY: build-linux-amd64 build-linux-arm64
.PHONY: build-darwin-amd64 build-darwin-arm64
.PHONY: build-windows-amd64
.PHONY: ui-install ui-dev ui-start ui-build

# ── CLI binary ───────────────────────────────────────────────────────────────

all: build

## build: build the kubeaccess CLI binary → bin/kubeaccess
build:
	go build -o bin/$(BINARY_NAME) main.go

## build-server: build the API server binary → bin/kubeaccess-server
build-server:
	go build -o bin/$(SERVER_BIN) ./cmd/server/main.go

## build-all: cross-compile CLI for all platforms
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 main.go

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64 main.go

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 main.go

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY_NAME)-darwin-arm64 main.go

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe main.go

# ── Quality ──────────────────────────────────────────────────────────────────

## test: run all unit tests
test:
	go test ./...

## test-e2e: run end-to-end tests against a live cluster (requires kubeconfig)
test-e2e:
	./test/e2e.sh

## fmt: format all Go source files
fmt:
	go fmt ./...

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint (requires golangci-lint in PATH)
lint:
	golangci-lint run ./...

# ── Run ─────────────────────────────────────────────────────────────────────

## run: run the CLI directly (no binary needed)
run:
	go run main.go

## run-server: run the API server directly (no binary needed)
run-server:
	go run ./cmd/server/main.go

# ── Cleanup ──────────────────────────────────────────────────────────────────

## clean: remove built binaries and Go build cache
clean:
	go clean
	rm -rf bin/

# ── UI ───────────────────────────────────────────────────────────────────────

## ui-install: install Node dependencies for the React UI
ui-install:
	cd ui && npm install

## ui-build: production build of the React UI → ui/dist/
ui-build:
	cd ui && npm run build

## ui-dev: start the UI with the mock API server (no cluster needed)
ui-dev:
	@[ -d ui/node_modules/.bin/vite ] || (echo "→ Installing UI dependencies..." && cd ui && npm install)
	cd ui && npm start

## ui-start: build CLI binary, then start Go API server + Vite dev server together
ui-start: build
	@[ -d ui/node_modules/.bin/vite ] || (echo "→ Installing UI dependencies..." && cd ui && npm install)
	@echo "Starting Go API server on :8080 and Vite UI on :3000"
	@trap 'kill 0' INT; \
	  KUBEACCESS_BIN=./bin/$(BINARY_NAME) go run ./cmd/server/main.go & \
	  (cd ui && npx vite) & \
	  wait
