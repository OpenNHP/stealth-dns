export GO111MODULE := on
export CGO_ENABLED=1

OS_NAME = $(shell uname -s | tr A-Z a-z)

# Go binary path
GOPATH := $(shell go env GOPATH 2>/dev/null || echo $$HOME/go)
GOBIN := $(GOPATH)/bin
PATH := $(GOBIN):$(PATH)
export PATH

# OpenNHP submodule directory
OPENNHP_DIR = third_party/opennhp

# Version management
# Use .version file to avoid conflict with version/ directory (case-insensitive filesystems)
BASE_VERSION := $(shell cat .version 2>/dev/null || echo "1.0.0")
BUILD_NUMBER := $(shell git rev-list --count HEAD 2>/dev/null || echo "0")
COMMIT_ID := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date "+%Y-%m-%d %H:%M:%S")
GOMODULE = github.com/OpenNHP/StealthDNS/version
VERSION_LDFLAGS = -X '$(GOMODULE).Version=$(BASE_VERSION)' -X '$(GOMODULE).BuildNumber=$(BUILD_NUMBER)' -X '$(GOMODULE).CommitID=$(COMMIT_ID)' -X '$(GOMODULE).BuildTime=$(BUILD_TIME)'

all: generate-version-and-build


generate-version-and-build:
	@echo "[StealthDNS] Start building..."
	@$(MAKE) init
	@$(MAKE) build-sdk
	@$(MAKE) build
	@echo "[StealthDNS] Build for platform ${OS_NAME} successfully done!"

init:
	@echo "[StealthDNS] Initializing..."
	@git clean -df release 2>/dev/null || true
	@git submodule update --init --recursive
	@go mod tidy

# Build OpenNHP SDK from submodule
build-sdk:
	@echo "[StealthDNS] Building OpenNHP SDK from submodule..."
ifeq ($(OS_NAME), linux)
	@$(MAKE) build-sdk-linux
else ifeq ($(OS_NAME), darwin)
	@$(MAKE) build-sdk-macos
else
	@echo "[StealthDNS] Skipping SDK build on ${OS_NAME}, use build.bat for Windows"
endif

build-sdk-linux:
	@echo "[StealthDNS] Building Linux SDK (nhp-agent.so)..."
	@cd $(OPENNHP_DIR)/nhp && go mod tidy
	@cd $(OPENNHP_DIR)/endpoints && go mod tidy
	@cd $(OPENNHP_DIR)/endpoints && \
		go build -a -trimpath -buildmode=c-shared -ldflags="-w -s" -v \
		-o ../../../sdk/nhp-agent.so ./agent/main/main.go ./agent/main/export.go
	@echo "[StealthDNS] Linux SDK built successfully!"
	@cd $(OPENNHP_DIR)/nhp && git restore go.mod go.sum 2>/dev/null || git checkout go.mod go.sum 2>/dev/null || true
	@cd $(OPENNHP_DIR)/endpoints && git restore go.mod go.sum 2>/dev/null || git checkout go.mod go.sum 2>/dev/null || true
	@cd $(OPENNHP_DIR) && git reset --hard HEAD 2>/dev/null || true

build-sdk-macos:
	@echo "[StealthDNS] Building macOS SDK (nhp-agent.dylib) - Universal Binary..."
	@cd $(OPENNHP_DIR)/nhp && go mod tidy
	@cd $(OPENNHP_DIR)/endpoints && go mod tidy
	@mkdir -p sdk
	@# Build for ARM64 (Apple Silicon)
	@echo "[StealthDNS] Building ARM64 variant..."
	@cd $(OPENNHP_DIR)/endpoints && \
		GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 \
		go build -a -trimpath -buildmode=c-shared -ldflags="-w -s" -v \
		-o ../../../sdk/nhp-agent-arm64.dylib ./agent/main/main.go ./agent/main/export.go
	@# Build for AMD64 (Intel)
	@echo "[StealthDNS] Building AMD64 variant..."
	@cd $(OPENNHP_DIR)/endpoints && \
		GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 \
		go build -a -trimpath -buildmode=c-shared -ldflags="-w -s" -v \
		-o ../../../sdk/nhp-agent-amd64.dylib ./agent/main/main.go ./agent/main/export.go
	@# Combine into universal binary using lipo
	@echo "[StealthDNS] Creating universal binary..."
	@lipo -create -output sdk/nhp-agent.dylib sdk/nhp-agent-arm64.dylib sdk/nhp-agent-amd64.dylib
	@# Fix the install name (lipo preserves the first input's install name which is nhp-agent-arm64.dylib)
	@install_name_tool -id nhp-agent.dylib sdk/nhp-agent.dylib
	@rm -f sdk/nhp-agent-arm64.dylib sdk/nhp-agent-amd64.dylib sdk/nhp-agent-arm64.h sdk/nhp-agent-amd64.h
	@echo "[StealthDNS] macOS SDK (universal) built successfully!"
	@cd $(OPENNHP_DIR)/nhp && git restore go.mod go.sum 2>/dev/null || git checkout go.mod go.sum 2>/dev/null || true
	@cd $(OPENNHP_DIR)/endpoints && git restore go.mod go.sum 2>/dev/null || git checkout go.mod go.sum 2>/dev/null || true
	@cd $(OPENNHP_DIR) && git reset --hard HEAD 2>/dev/null || true

build:
	@echo "[StealthDNS] Building package..."
	@echo "[StealthDNS] Version: $(BASE_VERSION) (Build: $(BUILD_NUMBER), Commit: $(COMMIT_ID))"
	@mkdir -p ./release/etc/cert ./release/sdk
	go build -trimpath -ldflags="-w -s $(VERSION_LDFLAGS)" -v -o ./release/stealth-dns ./main.go && \
	cp ./etc/*.toml ./release/etc/ && \
	cp ./sdk/nhp-agent.* ./release/sdk/ 2>/dev/null || true && \
	cp ./etc/cert/rootCA.pem ./release/etc/cert/ 2>/dev/null || true
ifeq ($(OS_NAME), darwin)
	install_name_tool -change nhp-agent.dylib ./sdk/nhp-agent.dylib ./release/stealth-dns
endif

# Windows SDK build (call from PowerShell/CMD)
build-sdk-windows:
	@echo "[StealthDNS] Building Windows SDK (nhp-agent.dll)..."
	cd $(OPENNHP_DIR)/nhp && go mod tidy
	cd $(OPENNHP_DIR)/endpoints && go mod tidy
	cd $(OPENNHP_DIR)/endpoints && \
		go build -a -trimpath -buildmode=c-shared -ldflags="-w -s" -v \
		-o ../../sdk/nhp-agent.dll ./agent/main/main.go ./agent/main/export.go
	@echo "[StealthDNS] Windows SDK built successfully!"

# Clean SDK binaries
clean-sdk:
	@echo "[StealthDNS] Cleaning SDK binaries..."
	rm -f sdk/nhp-agent.so sdk/nhp-agent.dylib sdk/nhp-agent.dll sdk/nhp-agent.h

# UI 构建相关目标
ui: ui-init ui-build
	@echo "[StealthDNS UI] Build for platform ${OS_NAME} successfully done!"

ui-init:
	@echo "[StealthDNS UI] Initializing..."
	@cd ui && go mod tidy
	@cd ui/frontend && npm ci || npm install
	@cd ui/frontend && git checkout package-lock.json 2>/dev/null || true

ui-build:
	@echo "[StealthDNS UI] Building UI package..."
	@echo "[StealthDNS UI] Version: $(BASE_VERSION) (Build: $(BUILD_NUMBER), Commit: $(COMMIT_ID))"
	@rm -rf ui/build/bin
	@# Update version in wails.json
	@sed -i.bak 's/"productVersion": "[^"]*"/"productVersion": "$(BASE_VERSION)"/' ui/wails.json 2>/dev/null || \
	 sed -i '' 's/"productVersion": "[^"]*"/"productVersion": "$(BASE_VERSION)"/' ui/wails.json 2>/dev/null || true
	@rm -f ui/wails.json.bak
	@# Update version in i18n files (simplified - just update the base version)
	@sed -i.bak 's/version: '\''版本 [^'\'']*'\''/version: '\''版本 $(BASE_VERSION)'\''/' ui/frontend/src/i18n/index.ts 2>/dev/null || \
	 sed -i '' 's/version: '\''版本 [^'\'']*'\''/version: '\''版本 $(BASE_VERSION)'\''/' ui/frontend/src/i18n/index.ts 2>/dev/null || true
	@sed -i.bak 's/version: '\''Version [^'\'']*'\''/version: '\''Version $(BASE_VERSION)'\''/' ui/frontend/src/i18n/index.ts 2>/dev/null || \
	 sed -i '' 's/version: '\''Version [^'\'']*'\''/version: '\''Version $(BASE_VERSION)'\''/' ui/frontend/src/i18n/index.ts 2>/dev/null || true
	@sed -i.bak 's/version: '\''バージョン [^'\'']*'\''/version: '\''バージョン $(BASE_VERSION)'\''/' ui/frontend/src/i18n/index.ts 2>/dev/null || \
	 sed -i '' 's/version: '\''バージョン [^'\'']*'\''/version: '\''バージョン $(BASE_VERSION)'\''/' ui/frontend/src/i18n/index.ts 2>/dev/null || true
	@rm -f ui/frontend/src/i18n/index.ts.bak
	@# Set version ldflags for UI build
	@echo "[StealthDNS UI] Injecting version info: Version=$(BASE_VERSION), Build=$(BUILD_NUMBER), Commit=$(COMMIT_ID), Time=$(BUILD_TIME)"
ifeq ($(OS_NAME), windows)
	@WAILS_CMD=$$(command -v wails 2>/dev/null || echo "$(GOBIN)/wails"); \
	if [ ! -f "$$WAILS_CMD" ] && [ ! -x "$$WAILS_CMD" ]; then \
		echo "[StealthDNS] Error: wails command not found. Please run 'make check-wails' first."; \
		exit 1; \
	fi; \
	cd ui && PATH="$(GOBIN):$$PATH" $$WAILS_CMD build -ldflags="-X 'stealthdns-ui/version.Version=$(BASE_VERSION)' -X 'stealthdns-ui/version.BuildNumber=$(BUILD_NUMBER)' -X 'stealthdns-ui/version.CommitID=$(COMMIT_ID)' -X 'stealthdns-ui/version.BuildTime=$(BUILD_TIME)'" -platform windows/amd64 -o stealthdns-ui.exe
	cp ./ui/build/bin/stealthdns-ui.exe ./release/
else ifeq ($(OS_NAME), darwin)
	@WAILS_CMD=$$(command -v wails 2>/dev/null); \
	if [ -z "$$WAILS_CMD" ]; then \
		WAILS_CMD="$(GOBIN)/wails"; \
	fi; \
	if [ ! -f "$$WAILS_CMD" ]; then \
		echo "[StealthDNS] Error: wails command not found at $$WAILS_CMD. Please run 'make check-wails' first."; \
		exit 1; \
	fi; \
	cd ui && PATH="$(GOBIN):$$PATH" $$WAILS_CMD build -ldflags="-X 'stealthdns-ui/version.Version=$(BASE_VERSION)' -X 'stealthdns-ui/version.BuildNumber=$(BUILD_NUMBER)' -X 'stealthdns-ui/version.CommitID=$(COMMIT_ID)' -X 'stealthdns-ui/version.BuildTime=$(BUILD_TIME)'" -platform darwin/universal
	rm -rf ./release/stealthdns-ui.app
	cp -r ./ui/build/bin/stealthdns-ui.app ./release/ 2>/dev/null || \
		cp ./ui/build/bin/stealthdns-ui ./release/
	@# Don't restore wailsjs/ - it contains auto-generated bindings including GetVersion
	@# cd ui/frontend && git checkout wailsjs/ 2>/dev/null || true
else
	@WAILS_CMD=$$(command -v wails 2>/dev/null || echo "$(GOBIN)/wails"); \
	if [ ! -f "$$WAILS_CMD" ] && [ ! -x "$$WAILS_CMD" ]; then \
		echo "[StealthDNS] Error: wails command not found. Please run 'make check-wails' first."; \
		exit 1; \
	fi; \
	cd ui && PATH="$(GOBIN):$$PATH" $$WAILS_CMD build -ldflags="-X 'stealthdns-ui/version.Version=$(BASE_VERSION)' -X 'stealthdns-ui/version.BuildNumber=$(BUILD_NUMBER)' -X 'stealthdns-ui/version.CommitID=$(COMMIT_ID)' -X 'stealthdns-ui/version.BuildTime=$(BUILD_TIME)'" -platform linux/amd64
	cp ./ui/build/bin/stealthdns-ui ./release/
endif

ui-dev:
	@echo "[StealthDNS UI] Starting development mode..."
	cd ui && wails dev


full: all ui
	@echo "[StealthDNS] Full build completed!"


clean:
	@echo "[StealthDNS] Cleaning..."
	git clean -df release
	rm -rf ui/build/bin
	rm -rf ui/frontend/dist
	rm -rf ui/frontend/node_modules

.PHONY: all generate-version-and-build init build build-sdk build-sdk-linux build-sdk-macos build-sdk-windows clean-sdk
