

BINARY_NAME=kubeaccess

.PHONY: all build build-all clean run fmt vet ui ui-install ui-dev
.PHONY: build-linux-amd64 build-linux-arm64
.PHONY: build-darwin-amd64 build-darwin-arm64
.PHONY: build-windows-amd64

all: build

build:
	go build -o bin/$(BINARY_NAME) main.go

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

test:
	go test ./...

test-e2e:
	./test/e2e.sh

clean:
	go clean
	rm -rf bin/

run:
	go run main.go

fmt:
	go fmt ./...

vet:
	go vet ./...

# ── UI ────────────────────────────────────────────────────────────────────────

## ui-install: install Node dependencies for the React UI
ui-install:
	cd ui && npm install

## ui-dev (mock): start UI + mock API in one terminal (no cluster needed)
ui-dev:
	@[ -d ui/node_modules/vite ] || (echo "→ Installing UI dependencies..." && cd ui && npm install)
	cd ui && npm start

## ui-start (real): build binary then run real Go API server + Vite together
ui-start: build
	@[ -d ui/node_modules/vite ] || (echo "→ Installing UI dependencies..." && cd ui && npm install)
	@echo "Starting Go API server on :8080 and Vite UI on :3000"
	@trap 'kill 0' INT; \
	  KUBEACCESS_BIN=./bin/$(BINARY_NAME) go run ./cmd/server/main.go & \
	  (cd ui && npx vite) & \
	  wait

