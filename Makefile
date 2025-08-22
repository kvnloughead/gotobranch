SHELL := /bin/bash

GO        ?= go
BIN_NAME  ?= gotobranch
CMD_PKG   ?= ./cmd/gotobranch
BUILD_DIR ?= bin

# System install prefix and bin dir (macOS-friendly defaults)
PREFIX  ?= /usr/local
BIN_DIR  ?= $(PREFIX)/bin

# User-local bin dir (no sudo)
DEV_BIN_DIR ?= $(HOME)/.local/bin

.PHONY: all build install dev-install clean

all: build

build:
	@mkdir -p "$(BUILD_DIR)"
	$(GO) build -o "$(BUILD_DIR)/$(BIN_NAME)" $(CMD_PKG)
	@echo "Built $(BUILD_DIR)/$(BIN_NAME)"

install: build
	@echo "Installing to $(BIN_DIR) (may require sudo if not writable)..."
	install -d "$(BIN_DIR)"
	install -m 0755 "$(BUILD_DIR)/$(BIN_NAME)" "$(BIN_DIR)/$(BIN_NAME)"
	@echo "Installed to $(BIN_DIR). Ensure it's on your PATH."

dev-install: build
	@echo "Installing to $(DEV_BIN_DIR)..."
	install -d "$(DEV_BIN_DIR)"
	install -m 0755 "$(BUILD_DIR)/$(BIN_NAME)" "$(DEV_BIN_DIR)/$(BIN_NAME)"
	@echo "Installed to $(DEV_BIN_DIR). If needed, add to PATH:"
	@echo '  echo '\''export PATH="$$HOME/.local/bin:$$PATH"'\'' >> $$HOME/.zshrc && source $$HOME/.zshrc'

clean:
	@rm -rf "$(BUILD_DIR)"
	@echo "Cleaned $(BUILD_DIR)"