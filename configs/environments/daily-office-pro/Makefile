IMAGE_NAME ?= daily-office-pro
TAG ?= latest
CONTAINER_ENGINE ?= docker
PYTHON_BASE_IMAGE ?= python:3.12-slim-bookworm
CN_PYTHON_BASE_IMAGE ?= m.daocloud.io/docker.io/library/python:3.12-slim-bookworm
CN_APT_MIRROR ?= https://mirrors.tuna.tsinghua.edu.cn/debian
CN_APT_SECURITY_MIRROR ?= https://mirrors.tuna.tsinghua.edu.cn/debian-security
CN_PIP_INDEX_URL ?= https://pypi.tuna.tsinghua.edu.cn/simple
CN_NPM_REGISTRY ?= https://registry.npmmirror.com
HIMALAYA_DOWNLOAD_URL ?=
DBX_DOWNLOAD_URL ?=
HTTPX_DOWNLOAD_URL ?=
DOTNET_SDK_PACKAGE ?=

.PHONY: build build-cn

# Usage:
#   make build-cn  # builds the CN-friendly variant of daily-office-pro:latest

build:
	$(CONTAINER_ENGINE) build \
		--tag $(IMAGE_NAME):$(TAG) \
		--build-arg PYTHON_BASE_IMAGE=$(PYTHON_BASE_IMAGE) \
		--build-arg HIMALAYA_DOWNLOAD_URL=$(HIMALAYA_DOWNLOAD_URL) \
		--build-arg DBX_DOWNLOAD_URL=$(DBX_DOWNLOAD_URL) \
		--build-arg HTTPX_DOWNLOAD_URL=$(HTTPX_DOWNLOAD_URL) \
		--build-arg DOTNET_SDK_PACKAGE=$(DOTNET_SDK_PACKAGE) \
		-f Dockerfile \
		.

build-cn:
	$(CONTAINER_ENGINE) build \
		--tag $(IMAGE_NAME):$(TAG) \
		--build-arg PYTHON_BASE_IMAGE=$(CN_PYTHON_BASE_IMAGE) \
		--build-arg APT_MIRROR=$(CN_APT_MIRROR) \
		--build-arg APT_SECURITY_MIRROR=$(CN_APT_SECURITY_MIRROR) \
		--build-arg PIP_INDEX_URL=$(CN_PIP_INDEX_URL) \
		--build-arg NPM_REGISTRY=$(CN_NPM_REGISTRY) \
		--build-arg HIMALAYA_DOWNLOAD_URL=$(HIMALAYA_DOWNLOAD_URL) \
		--build-arg DBX_DOWNLOAD_URL=$(DBX_DOWNLOAD_URL) \
		--build-arg HTTPX_DOWNLOAD_URL=$(HTTPX_DOWNLOAD_URL) \
		--build-arg DOTNET_SDK_PACKAGE=$(DOTNET_SDK_PACKAGE) \
		-f Dockerfile \
		.
