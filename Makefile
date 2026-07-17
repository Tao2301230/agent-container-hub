APP_NAME := agent-container-hub
ifeq ($(OS),Windows_NT)
VERSION :=
ARCH ?= amd64
else
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
endif
LDFLAGS := -X main.buildVersion=$(VERSION)
BUILD_DIR := dist/release
CONTAINER_ENGINE ?= docker
NETWORK_POLICY_HELPER_IMAGE ?= agent-container-hub/network-policy-helper:latest

.PHONY: build run test docker-build network-policy-helper-image release release-program release-image clean

build:
	go build -ldflags "$(LDFLAGS)" -o ./$(APP_NAME) ./cmd/agent-container-hub

run:
	set -a; [ ! -f .env ] || . ./.env; set +a; go run -ldflags "$(LDFLAGS)" ./cmd/agent-container-hub

test:
	go test ./...

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t agent-container-hub:latest .

network-policy-helper-image:
	$(CONTAINER_ENGINE) build -t $(NETWORK_POLICY_HELPER_IMAGE) configs/network-policy-helper

release:
	$(MAKE) release-program VERSION=$(VERSION) ARCH=$(ARCH)

ifeq ($(OS),Windows_NT)
release-program:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/release-program.ps1 -Version "$(VERSION)" -Arch "$(ARCH)"
else
release-program:
	VERSION=$(VERSION) ARCH=$(ARCH) bash scripts/release.sh
endif

release-image:
	VERSION=$(VERSION) ARCH=$(ARCH) bash scripts/release-image.sh

clean:
	rm -f $(APP_NAME)
	rm -rf $(BUILD_DIR)
