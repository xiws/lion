# lion Makefile

BINARY_NAME := lion
MAIN_PATH   := ./cmd/cli
DIST_DIR    := ./dist
VERSION     ?= $(shell git describe --tags --always --dirty 2>nul || echo dev)
BUILD_TIME  := $(shell date +%Y-%m-%d_%H:%M:%S)
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# 目标平台: OS/ARCH 组合
PLATFORMS := windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

# ============================================================
# 常用命令
# ============================================================

.PHONY: pub
pub:
	cp ./dist/lion-linux-amd64 /home/xiw/go/bin/lion
	chmod 777 /home/xiw/go/bin/lion

.PHONY: all build build-all clean help

## 构建当前平台
build:
	go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)$(if $(filter windows,$(OS)),.exe,) $(MAIN_PATH)

## 构建所有平台
build-all: clean $(PLATFORMS)

## 清理构建产物
clean:
	rm -rf $(DIST_DIR)

## 帮助信息
help:
	@echo "用法:"
	@echo "  make build        构建当前平台"
	@echo "  make build-all    构建所有平台 (win/mac/linux, amd64/arm64)"
	@echo "  make clean        清理构建产物"
	@echo ""
	@echo "单独构建某个平台:"
	@echo "  make windows/amd64"
	@echo "  make darwin/arm64"
	@echo "  make linux/amd64"

# ============================================================
# 各平台构建规则
# ============================================================

.PHONY: $(PLATFORMS)

windows/amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

windows/arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PATH)

darwin/amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)

darwin/arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

linux/amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

linux/arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)

