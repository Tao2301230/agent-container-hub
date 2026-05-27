IMAGE_NAME ?= shell
TAG ?= latest
CONTAINER_ENGINE ?= docker
BASE_IMAGE ?= busybox:latest
CN_BASE_IMAGE ?= m.daocloud.io/docker.io/library/busybox:1.37.0

.PHONY: build build-cn

# Usage:
#   make build-cn  # override CN_BASE_IMAGE=... if your preferred mirror differs

build:
	$(CONTAINER_ENGINE) build \
		--tag $(IMAGE_NAME):$(TAG) \
		--build-arg BASE_IMAGE=$(BASE_IMAGE) \
		-f Dockerfile \
		.

build-cn:
	$(CONTAINER_ENGINE) build \
		--tag $(IMAGE_NAME):$(TAG) \
		--build-arg BASE_IMAGE=$(CN_BASE_IMAGE) \
		-f Dockerfile \
		.
