.PHONY: help test bench load-test load-baseline load-spike load-soak load-stress run build clean generate-protos install-proto-tools

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Testing
test: ## Run all unit tests
	go test -v -race -cover ./...

test-short: ## Run unit tests (short mode)
	go test -v -short ./...

bench: ## Run all benchmarks
	go test -bench=. -benchmem -benchtime=1s ./internal/exchange ./pkg/idr ./internal/fpd ./internal/metrics

bench-save: ## Run benchmarks and save results
	@mkdir -p benchmarks/results
	go test -bench=. -benchmem -benchtime=1s ./... > benchmarks/results/bench-$$(date +%Y%m%d-%H%M%S).txt

# Load Testing (requires k6)
load-test: ## Run all load tests sequentially
	./tests/load/run-load-tests.sh all

load-baseline: ## Run baseline load test (5 min, 1k-10k QPS)
	./tests/load/run-load-tests.sh baseline

load-spike: ## Run spike test (2 min, 50k QPS burst)
	./tests/load/run-load-tests.sh spike

load-soak: ## Run soak test (1 hour, 5k QPS sustained)
	./tests/load/run-load-tests.sh soak

load-soak-24h: ## Run 24-hour soak test
	./tests/load/run-load-tests.sh soak 24h

load-stress: ## Run stress test (find breaking point)
	./tests/load/run-load-tests.sh stress

load-go: ## Run Go-based load test (no k6 required)
	go test -v ./tests/load -tags=loadtest -timeout 30m -qps=1000 -duration=5m

# Development
run: ## Run the server locally
	go run cmd/server/main.go

build: ## Build the binary
	go build -o bin/catalyst cmd/server/main.go

clean: ## Clean build artifacts
	rm -rf bin/ benchmarks/results/ tests/load/results/

fmt: ## Format code
	go fmt ./...

lint: ## Run linters
	golangci-lint run

vet: ## Run go vet
	go vet ./...

# Docker
docker-build: ## Build Docker image
	docker build -t catalyst:latest .

docker-run: ## Run Docker container
	docker run -p 8080:8080 catalyst:latest

# Metrics and Monitoring
metrics: ## Show Prometheus metrics
	@curl -s http://localhost:8080/metrics | grep -v '^#' | head -50

circuit-breakers: ## Show circuit breaker status
	@curl -s http://localhost:8080/admin/circuit-breakers | jq

health: ## Check server health
	@curl -s http://localhost:8080/health | jq

# Installation
install-deps: ## Install Go dependencies
	go mod download

install-tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

install-k6: ## Install k6 (macOS only)
	@command -v k6 >/dev/null 2>&1 || brew install k6

# Documentation
docs: ## Generate documentation
	godoc -http=:6060 &
	@echo "Documentation server running at http://localhost:6060"

# Agentic / ARTF protos
install-proto-tools: ## Install protoc plugins for agentic codegen (devs only; CI does not run this)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

generate-protos: ## Regenerate agentic/gen/ from agentic/proto/ (devs only; CI does not run this)
	@command -v protoc >/dev/null 2>&1 || { echo "protoc v25+ required (edition 2023)"; exit 1; }
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo "run 'make install-proto-tools' first"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "run 'make install-proto-tools' first"; exit 1; }
	rm -rf agentic/gen
	mkdir -p agentic/gen
	protoc \
	  --proto_path=agentic/proto \
	  --go_out=agentic/gen --go_opt=paths=source_relative \
	  --go-grpc_out=agentic/gen --go-grpc_opt=paths=source_relative \
	  agentic/proto/iabtechlab/openrtb/v26/openrtb.proto \
	  agentic/proto/iabtechlab/bidstream/mutation/v1/agenticrtbframework.proto \
	  agentic/proto/iabtechlab/bidstream/mutation/v1/agenticrtbframeworkservices.proto
	@echo "✅ Protos regenerated. Commit agentic/gen/ with the proto changes."

# Pre-commit checks
pre-commit: fmt vet test ## Run pre-commit checks

# CI/CD simulation
ci: fmt vet test bench ## Simulate CI pipeline
	@echo "✅ All CI checks passed!"
