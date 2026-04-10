APP_NAME := agent-container-hub
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
LDFLAGS := -X main.buildVersion=$(VERSION)
BUILD_DIR := dist/release

.PHONY: build run test docker-build release release-program release-image clean

build:
	go build -ldflags "$(LDFLAGS)" -o ./$(APP_NAME) ./cmd/agent-container-hub

run:
	set -a; [ ! -f .env ] || . ./.env; set +a; go run -ldflags "$(LDFLAGS)" ./cmd/agent-container-hub

test:
	go test ./...

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t agent-container-hub:latest .

release:
	$(MAKE) release-program VERSION=$(VERSION) ARCH=$(ARCH)

release-program:
	VERSION=$(VERSION) ARCH=$(ARCH) bash scripts/release.sh

release-image:
	VERSION=$(VERSION) ARCH=$(ARCH) bash scripts/release-image.sh

clean:
	rm -f $(APP_NAME)
	rm -rf $(BUILD_DIR)
