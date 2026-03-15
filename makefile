# OCI ARM A1 Instance Provisioner - Makefile
APP_NAME  = provisioner
VERSION   = 1.0.0
BIN_DIR   = bin/$(VERSION)

# Target platform (Oracle Cloud AMD64 Linux)
GOOS      = linux
GOARCH    = amd64

.PHONY: all build clean upload help

all: build

## build: Compile binary for Linux AMD64 and place in bin/VERSION/
build:
	@echo "🔨 Building $(APP_NAME) v$(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BIN_DIR)/$(APP_NAME) main.go
	@echo "✅ Binary ready at $(BIN_DIR)/$(APP_NAME)"

## upload: Upload binary to Oracle VM (set VM_IP=your.server.ip)
upload:
	@if [ -z "$(VM_IP)" ]; then echo "❌ Set VM_IP: make upload VM_IP=1.2.3.4"; exit 1; fi
	@echo "📤 Uploading binary to $(VM_IP)..."
	scp $(BIN_DIR)/$(APP_NAME) ubuntu@$(VM_IP):~/oracle-provisioner/provisioner
	@echo "✅ Upload complete!"

## clean: Remove all binaries
clean:
	@echo "🧹 Cleaning bin/ directory..."
	@rm -rf bin/
	@echo "✅ Done"

## help: Show available commands
help:
	@echo "Available commands:"
	@grep -E '^## ' Makefile | sed 's/## /  make /'