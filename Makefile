.PHONY: build test clean run install fmt lint deps e2e-test coverage release-dry

BINARY_NAME=kube-vap-test
KUBECTL_PLUGIN_NAME=kubectl-kube-vap-test
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.1.0")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands.Version=$(VERSION) -X github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands.Commit=$(COMMIT) -X github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands.BuildDate=$(DATE)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kube-vap-test

test:
	go test -v ./...

# E2Eテスト実行
e2e-test:
	./scripts/run-e2e-tests.sh

# Go版E2Eテスト実行
e2e-go-test:
	cd test/e2e && go mod tidy && go test -v

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	go clean

run:
	go run ./cmd/kube-vap-test

run-example:
	go run ./cmd/kube-vap-test run examples/tests/no-latest-tag-test.yaml

check-example:
	go run ./cmd/kube-vap-test check --rules examples/policies/no-latest-tag-policy.yaml examples/manifests/allowed-pod-specific-tag.yaml

check-examples:
	go run ./cmd/kube-vap-test check --rules examples/policies/no-latest-tag-policy.yaml examples/manifests/*.yaml

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

deps:
	go mod tidy

install:
	go install ./cmd/kube-vap-test

install-kubectl-plugin:
	go build $(LDFLAGS) -o $(KUBECTL_PLUGIN_NAME) ./cmd/kube-vap-test
	chmod +x $(KUBECTL_PLUGIN_NAME)
	@echo "Installing kubectl plugin to $$GOPATH/bin or /usr/local/bin"
	@if [ -n "$$GOPATH" ] && [ -d "$$GOPATH/bin" ]; then \
		mv $(KUBECTL_PLUGIN_NAME) $$GOPATH/bin/; \
		echo "Installed to $$GOPATH/bin/$(KUBECTL_PLUGIN_NAME)"; \
	else \
		sudo mv $(KUBECTL_PLUGIN_NAME) /usr/local/bin/; \
		echo "Installed to /usr/local/bin/$(KUBECTL_PLUGIN_NAME)"; \
	fi
	@echo "You can now use 'kubectl kube-vap-test' command"

# Coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Check for security vulnerabilities
sec:
	gosec ./...

# Run all checks before commit
check: fmt lint test

# Release (dry run)
release-dry:
	goreleaser release --snapshot --clean

all: clean build test 