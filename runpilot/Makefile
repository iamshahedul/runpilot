SHELL := /bin/bash
GO ?= $(shell which go 2>/dev/null || echo /Users/Shahed/.local/go/bin/go)

BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/runpilot-server
AGENT_BIN := $(BIN_DIR)/runpilot-agent
CLI_BIN := $(BIN_DIR)/runpilot

.PHONY: all build clean test test-gate test-unit run-server run-agent

all: build

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	@echo "Building RunPilot binaries using $(GO)..."
	$(GO) build -o $(SERVER_BIN) ./control-plane/cmd/runpilot-server
	$(GO) build -o $(AGENT_BIN) ./agent/cmd/runpilot-agent
	$(GO) build -o $(CLI_BIN) ./cli/cmd/runpilot
	@echo "Build complete: $(SERVER_BIN), $(AGENT_BIN), $(CLI_BIN)"

test: test-gate test-unit

test-gate:
	@echo "Running Gate 1 Test: Absolute No PHP or Composer Detection..."
	bash scripts/test-preflight-no-php.sh

test-unit:
	@echo "Running Go Unit & Integration Tests..."
	$(GO) test -v ./...

clean:
	rm -rf $(BIN_DIR)
