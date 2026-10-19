BINARY_NAME=tmaegis
VERSION=0.1.0
BUILD_DIR=bin
LDFLAGS=-buildid=$(VERSION) -X github.com/tmaegis/tmaegis/cmd.Version=$(VERSION)

.PHONY: all build clean test test-all build-all run-scan build-server validate-rules

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./main.go

build-all:
	@echo "Cross-compiling $(BINARY_NAME) for Linux, Windows, and macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./main.go
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./main.go
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./main.go

test:
	go test -race ./...

build-server:
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/tmaegis-server ./cmd/tmaegis-server

# Internal ai-pipeline tests (the directory only exists in the private repository).
ai-pipeline-test: build
	cd ai-pipeline && TMAEGIS_BIN=$(abspath $(BUILD_DIR))/tmaegis python3 -m pytest -q

validate-rules:
	go run . rules validate

# Go + Python pipeline (internal) + web portal
test-all: test validate-rules
	@if [ -d ai-pipeline ]; then cd ai-pipeline && python3 -m pytest -q; fi
	cd web-portal && npm test --silent

clean:
	rm -rf $(BUILD_DIR)
