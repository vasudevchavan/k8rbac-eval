

BINARY_NAME=kubeaccess

.PHONY: all build build-all clean run fmt vet
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

