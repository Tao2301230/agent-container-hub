#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "[load-image] $*" >&2
  exit 1
}

IMAGE_DIR="./images"

[[ -d "$IMAGE_DIR" ]] || die "image directory not found: $IMAGE_DIR"
command -v docker >/dev/null 2>&1 || die "docker is required in PATH"

archives=("$IMAGE_DIR"/*.tar.gz)
[[ -e "${archives[0]}" ]] || die "no image archive found under $IMAGE_DIR"
[[ "${#archives[@]}" -eq 1 ]] || die "expected exactly one image archive under $IMAGE_DIR"

archive="${archives[0]}"
base_name="$(basename "$archive")"

if [[ ! "$base_name" =~ ^agent-container-hub-image-(v[0-9]+\.[0-9]+\.[0-9]+)-linux-(amd64|arm64)\.tar\.gz$ ]]; then
  die "unexpected image archive name: $base_name"
fi

version="${BASH_REMATCH[1]}"
arch="${BASH_REMATCH[2]}"
image_tag="agent-container-hub:${version}-linux-${arch}"

echo "[load-image] importing $archive"
gzip -dc "$archive" | docker load
echo "[load-image] image available as $image_tag"
