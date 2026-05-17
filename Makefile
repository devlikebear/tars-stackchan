SHELL := /bin/sh

.DEFAULT_GOAL := help

REPO_ROOT := $(CURDIR)
MCP_DIR := $(REPO_ROOT)/mcp-server
DIST_DIR := $(REPO_ROOT)/dist

GO ?= go
NODE ?= node
GORELEASER ?= goreleaser

MCP_BIN := $(DIST_DIR)/tars-stackchan-mcp
CONTROL_BIN := $(DIST_DIR)/tars-stackchan-control

DEFAULT_STACKCHAN_BASE_URL := http://stackchan.local
TARS_STACKCHAN_BASE_URL ?= $(DEFAULT_STACKCHAN_BASE_URL)
ifeq ($(origin TARS_STACKCHAN_BASE_URL),file)
TARS_STACKCHAN_AUTO_BASE_URL ?= 1
else
TARS_STACKCHAN_AUTO_BASE_URL ?= 0
endif
TARS_STACKCHAN_BRIDGE ?= mock
TARS_STACKCHAN_CONTROL_ADDR ?= 127.0.0.1:8787
TARS_STACKCHAN_TTS_PORT ?= 18080
TARS_STACKCHAN_TTS_TOKEN ?=
TARS_STACKCHAN_GEMINI_API_KEY ?=
GEMINI_API_KEY ?=
TARS_STACKCHAN_PERCEIVE_CAMERA ?= off
TARS_STACKCHAN_TARS_BASE_URL ?=
TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL ?=
TARS_STACKCHAN_TARS_TOKEN ?=
TARS_STACKCHAN_DISCOVER_RESET ?= 1
TARS_STACKCHAN_DISCOVER_TIMEOUT ?= 20
OWNER_NAME ?= owner

export TARS_STACKCHAN_TOKEN
export TARS_STACKCHAN_TTS_TOKEN
export TARS_STACKCHAN_GEMINI_API_KEY
export GEMINI_API_KEY
export TARS_STACKCHAN_TARS_TOKEN
export TARS_STACKCHAN_DISCOVER_RESET
export TARS_STACKCHAN_DISCOVER_TIMEOUT

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo.Version=$(VERSION) \
	-X github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/devlikebear/tars-stackchan/mcp-server/internal/buildinfo.Date=$(DATE)

RESOLVE_STACKCHAN_BASE_URL = base_url="$(TARS_STACKCHAN_BASE_URL)"; if [ "$(TARS_STACKCHAN_AUTO_BASE_URL)" = "1" ]; then if discovered="$$(scripts/dev/discover-base-url.sh --url-only)"; then base_url="$$discovered"; echo "Using discovered Stack-chan base URL: $$base_url"; else echo "error: auto base URL discovery failed" >&2; echo "hint: set TARS_STACKCHAN_BASE_URL=http://<device-ip> or inspect USB serial discovery with 'make discover-base-url'" >&2; exit 1; fi; fi

.PHONY: help
help: ## Show available Makefile targets
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-24s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Quality

.PHONY: fmt
fmt: ## Format Go code
	cd "$(MCP_DIR)" && $(GO) fmt ./...

.PHONY: lint
lint: ## Run Go static checks
	cd "$(MCP_DIR)" && $(GO) vet ./...

.PHONY: test
test: test-go test-firmware test-contracts test-release ## Run all local verification that does not require hardware

.PHONY: test-go
test-go: ## Run all Go tests
	cd "$(MCP_DIR)" && $(GO) test ./...

.PHONY: test-firmware
test-firmware: ## Run firmware bridge Node contract tests
	scripts/test/firmware-bridge-contract.sh

.PHONY: test-contracts
test-contracts: ## Run shell-level project contract tests
	scripts/test/makefile-contract.sh
	scripts/test/discover-base-url-contract.sh
	scripts/test/firmware-upload-script-contract.sh
	scripts/test/hardware-smoke-contract.sh
	scripts/test/tts-relay-go-contract.sh
	scripts/test/perception-loop-contract.sh

.PHONY: test-release
test-release: ## Validate CI, GoReleaser, and Homebrew release config
	scripts/test/release-config-contract.sh

##@ Build

.PHONY: build
build: build-mcp build-control ## Build both local binaries into dist/

.PHONY: build-mcp
build-mcp: ## Build the MCP server binary
	mkdir -p "$(DIST_DIR)"
	cd "$(MCP_DIR)" && $(GO) build -ldflags "$(LDFLAGS)" -o "$(MCP_BIN)" ./cmd/tars-stackchan-mcp

.PHONY: build-control
build-control: ## Build the local control UI/TTS/perception binary
	mkdir -p "$(DIST_DIR)"
	cd "$(MCP_DIR)" && $(GO) build -ldflags "$(LDFLAGS)" -o "$(CONTROL_BIN)" ./cmd/tars-stackchan-control

.PHONY: clean
clean: ## Remove local build artifacts
	rm -rf "$(DIST_DIR)"

.PHONY: clean-work
clean-work: ## Remove prepared upstream firmware checkout under .work/
	rm -rf "$(REPO_ROOT)/.work"

##@ MCP server

.PHONY: run-mcp
run-mcp: ## Run the stdio MCP server with the mock bridge
	cd "$(MCP_DIR)" && TARS_STACKCHAN_BRIDGE=mock $(GO) run ./cmd/tars-stackchan-mcp

.PHONY: run-mcp-http
run-mcp-http: ## Run the stdio MCP server against real firmware over HTTP
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_BRIDGE=http \
		TARS_STACKCHAN_BASE_URL="$$base_url" \
		$(GO) run ./cmd/tars-stackchan-mcp

.PHONY: run-mcp-firmware
run-mcp-firmware: ## Run the MCP server with opt-in firmware upload tools enabled
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_ENABLE_FIRMWARE_TOOLS=1 \
		TARS_STACKCHAN_BRIDGE=http \
		TARS_STACKCHAN_BASE_URL="$$base_url" \
		$(GO) run ./cmd/tars-stackchan-mcp

.PHONY: mcp-tools-list
mcp-tools-list: ## List MCP tools with the mock bridge through JSON-RPC tools/list
	cd "$(MCP_DIR)" && \
		printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | \
		TARS_STACKCHAN_BRIDGE=mock $(GO) run ./cmd/tars-stackchan-mcp

.PHONY: doctor
doctor: ## Run offline doctor checks without hardware, TTS, or perception probes
	cd "$(MCP_DIR)" && TARS_STACKCHAN_BRIDGE=mock $(GO) run ./cmd/tars-stackchan-mcp doctor --skip-device --skip-tts --skip-perception

.PHONY: doctor-http
doctor-http: ## Run doctor against real hardware and local services
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_BRIDGE=http \
		TARS_STACKCHAN_BASE_URL="$$base_url" \
		$(GO) run ./cmd/tars-stackchan-mcp doctor

.PHONY: discover-base-url
discover-base-url: ## Discover a reachable firmware base URL from mDNS or USB serial logs
	scripts/dev/discover-base-url.sh

.PHONY: config-claude
config-claude: ## Print a Claude Code MCP config/install snippet
	cd "$(MCP_DIR)" && $(GO) run ./cmd/tars-stackchan-mcp config --target claude-code --firmware-tools

.PHONY: config-claude-desktop
config-claude-desktop: ## Print a Claude Desktop MCP config snippet
	cd "$(MCP_DIR)" && $(GO) run ./cmd/tars-stackchan-mcp config --target claude-desktop

.PHONY: config-tars
config-tars: ## Print a TARS MCP config snippet
	cd "$(MCP_DIR)" && $(GO) run ./cmd/tars-stackchan-mcp config --target tars

.PHONY: install-claude
install-claude: ## Install the MCP server into Claude Code with firmware tools enabled
	cd "$(MCP_DIR)" && $(GO) run ./cmd/tars-stackchan-mcp install --target claude-code --firmware-tools

##@ Control UI

.PHONY: run-control
run-control: ## Run the browser control UI against real firmware
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_CONTROL_ADDR="$(TARS_STACKCHAN_CONTROL_ADDR)" \
		TARS_STACKCHAN_BRIDGE=http \
		TARS_STACKCHAN_BASE_URL="$$base_url" \
		$(GO) run ./cmd/tars-stackchan-control

.PHONY: run-control-mock
run-control-mock: ## Run the browser control UI with the mock bridge
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_CONTROL_ADDR="$(TARS_STACKCHAN_CONTROL_ADDR)" \
		TARS_STACKCHAN_BRIDGE=mock \
		$(GO) run ./cmd/tars-stackchan-control

.PHONY: tts-serve
tts-serve: ## Run the Gemini TTS relay in the foreground
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_TTS_PORT="$(TARS_STACKCHAN_TTS_PORT)" \
		$(GO) run ./cmd/tars-stackchan-control tts serve

.PHONY: tts-status
tts-status: ## Check local TTS relay health and mDNS
	cd "$(MCP_DIR)" && $(GO) run ./cmd/tars-stackchan-control tts status --port "$(TARS_STACKCHAN_TTS_PORT)"

.PHONY: perceive-serve
perceive-serve: ## Run the perception loop
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_BRIDGE=http \
		TARS_STACKCHAN_BASE_URL="$$base_url" \
		TARS_STACKCHAN_PERCEIVE_CAMERA="$(TARS_STACKCHAN_PERCEIVE_CAMERA)" \
		TARS_STACKCHAN_TARS_BASE_URL="$(TARS_STACKCHAN_TARS_BASE_URL)" \
		TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL="$(TARS_STACKCHAN_TARS_WEBHOOK_CHANNEL)" \
		$(GO) run ./cmd/tars-stackchan-control perceive serve

.PHONY: perceive-enroll
perceive-enroll: ## Enroll the local owner fingerprint, e.g. make perceive-enroll OWNER_NAME=me
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	cd "$(MCP_DIR)" && \
		TARS_STACKCHAN_BRIDGE=http \
		TARS_STACKCHAN_BASE_URL="$$base_url" \
		$(GO) run ./cmd/tars-stackchan-control perceive enroll --name "$(OWNER_NAME)"

.PHONY: perceive-reset
perceive-reset: ## Remove the local owner fingerprint
	cd "$(MCP_DIR)" && $(GO) run ./cmd/tars-stackchan-control perceive enroll --reset

##@ Firmware

.PHONY: firmware-check
firmware-check: ## Check whether local firmware upload prerequisites are ready
	scripts/dev/check-firmware-upload-ready.sh

.PHONY: firmware-prepare
firmware-prepare: ## Prepare the pinned upstream firmware checkout and bridge MOD
	scripts/dev/upload-firmware.sh prepare

.PHONY: firmware-deps
firmware-deps: ## Prepare firmware checkout and install upstream dependencies
	scripts/dev/upload-firmware.sh deps

.PHONY: firmware-host
firmware-host: ## Flash the upstream host firmware to Stack-chan
	scripts/dev/upload-firmware.sh host

.PHONY: firmware-mod
firmware-mod: ## Build and direct-flash the TARS bridge MOD
	scripts/dev/upload-firmware.sh mod

.PHONY: firmware-smoke
firmware-smoke: ## Run firmware hardware smoke via upload helper
	scripts/dev/upload-firmware.sh smoke

.PHONY: firmware-all
firmware-all: ## Prepare, build, flash MOD, optionally flash host, and smoke-test hardware
	scripts/dev/upload-firmware.sh all

.PHONY: hardware-smoke
hardware-smoke: ## Run the direct HTTP/MCP hardware smoke test
	@$(RESOLVE_STACKCHAN_BASE_URL); \
	TARS_STACKCHAN_BASE_URL="$$base_url" \
	scripts/test/hardware-smoke.sh

##@ Release

.PHONY: release-check
release-check: test-release ## Validate release automation contracts

.PHONY: release-snapshot
release-snapshot: ## Build a local GoReleaser snapshot
	$(GORELEASER) release --snapshot --clean
