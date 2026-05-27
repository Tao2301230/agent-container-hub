IMAGE_NAME ?= toolbox
TAG ?= latest
CONTAINER_ENGINE ?= docker
PYTHON_BASE_IMAGE ?= python:3.12-slim-bookworm
CN_PYTHON_BASE_IMAGE ?= m.daocloud.io/docker.io/library/python:3.12-slim-bookworm
CN_APT_MIRROR ?= https://mirrors.tuna.tsinghua.edu.cn/debian
CN_APT_SECURITY_MIRROR ?= https://mirrors.tuna.tsinghua.edu.cn/debian-security
DBX_DOWNLOAD_URL ?=
HTTPX_DOWNLOAD_URL ?=
MOCK_DOWNLOAD_URL ?=

.PHONY: build build-cn

# Usage:
#   make build-cn  # builds the CN-friendly variant of toolbox:latest

build:
	$(CONTAINER_ENGINE) build \
		--tag $(IMAGE_NAME):$(TAG) \
		--build-arg PYTHON_BASE_IMAGE=$(PYTHON_BASE_IMAGE) \
		--build-arg DBX_DOWNLOAD_URL=$(DBX_DOWNLOAD_URL) \
		--build-arg HTTPX_DOWNLOAD_URL=$(HTTPX_DOWNLOAD_URL) \
		--build-arg MOCK_DOWNLOAD_URL=$(MOCK_DOWNLOAD_URL) \
		-f Dockerfile \
		.

build-cn:
	$(CONTAINER_ENGINE) build \
		--tag $(IMAGE_NAME):$(TAG) \
		--build-arg PYTHON_BASE_IMAGE=$(CN_PYTHON_BASE_IMAGE) \
		--build-arg APT_MIRROR=$(CN_APT_MIRROR) \
		--build-arg APT_SECURITY_MIRROR=$(CN_APT_SECURITY_MIRROR) \
		--build-arg DBX_DOWNLOAD_URL=$(DBX_DOWNLOAD_URL) \
		--build-arg HTTPX_DOWNLOAD_URL=$(HTTPX_DOWNLOAD_URL) \
		--build-arg MOCK_DOWNLOAD_URL=$(MOCK_DOWNLOAD_URL) \
		-f Dockerfile \
		.
