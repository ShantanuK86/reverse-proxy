.PHONY: build test lint run certs demo-up demo-down loadtest clean

# Go parameters
BINARY_NAME=proxy
BUILD_DIR=bin
MAIN_PKG=./cmd/proxy

## build: Compile the proxy binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)
	@echo "Binary: $(BUILD_DIR)/$(BINARY_NAME)"

## test: Run all tests with race detection
test:
	go test -race -v ./...

## lint: Run go vet
lint:
	go vet ./...

## run: Build and run the proxy locally
run: build certs
	$(BUILD_DIR)/$(BINARY_NAME) --config config/proxy.yaml

## certs: Generate self-signed TLS certificates
certs:
	@chmod +x certs/generate.sh
	@certs/generate.sh

## demo-up: Start the full Docker Compose demo stack
demo-up:
	docker compose -f demo/docker-compose.yml up --build -d
	@echo ""
	@echo "Stack is running:"
	@echo "  Proxy (HTTP):   http://localhost:8080"
	@echo "  Proxy (HTTPS):  https://localhost:8443"
	@echo "  Prometheus:     http://localhost:9091"
	@echo ""
	@echo "Try: curl http://localhost:8080/"

## demo-down: Tear down the Docker Compose demo stack
demo-down:
	docker compose -f demo/docker-compose.yml down -v

## loadtest: Run k6 load test against localhost
loadtest:
	k6 run --env BASE_URL=http://localhost:8080 demo/loadtest/k6_script.js

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f certs/ca.* certs/server.*

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
