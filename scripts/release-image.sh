#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# shellcheck disable=SC1091
. "$SCRIPT_DIR/release-common.sh"

resolve_release_context
command -v docker >/dev/null 2>&1 || die "docker is required"
command -v gzip >/dev/null 2>&1 || die "gzip is required"

cd "$REPO_ROOT"

IMAGE_TARGET_OS="linux"
IMAGE_TAG="${APP_NAME}:${VERSION}-${IMAGE_TARGET_OS}-${ARCH}"
IMAGE_BUNDLE="$RELEASE_DIR/${APP_NAME}-image-${VERSION}-${IMAGE_TARGET_OS}-${ARCH}.tar.gz"

echo "[release] image VERSION=$VERSION TARGET_OS=$IMAGE_TARGET_OS ARCH=$ARCH"

mkdir -p "$RELEASE_DIR"
docker build \
  --platform "${IMAGE_TARGET_OS}/${ARCH}" \
  --build-arg VERSION="$VERSION" \
  -t "$IMAGE_TAG" \
  .

docker save "$IMAGE_TAG" | gzip -c >"$IMAGE_BUNDLE"

echo "[release] done: $IMAGE_BUNDLE"
