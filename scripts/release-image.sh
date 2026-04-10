#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_ASSETS_DIR="$SCRIPT_DIR/release-assets/image-bundle"

# shellcheck disable=SC1091
. "$SCRIPT_DIR/release-common.sh"

resolve_release_context
require_image_tools
command -v tar >/dev/null 2>&1 || die "tar is required"

cd "$REPO_ROOT"

IMAGE_TARGET_OS="$(image_target_os)"
IMAGE_BUNDLE="$RELEASE_DIR/$(image_bundle_name)"

echo "[release] image bundle VERSION=$VERSION TARGET_OS=$IMAGE_TARGET_OS ARCH=$ARCH"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agent-container-hub-image-release.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

BUNDLE_ROOT="$TMP_DIR/$APP_NAME"
mkdir -p \
  "$BUNDLE_ROOT/images" \
  "$BUNDLE_ROOT/configs"

cp "$REPO_ROOT/.env.example" "$BUNDLE_ROOT/.env.example"
cp "$RELEASE_ASSETS_DIR/README.txt" "$BUNDLE_ROOT/README.txt"
cp "$RELEASE_ASSETS_DIR/load-image.sh" "$BUNDLE_ROOT/load-image.sh"
chmod +x "$BUNDLE_ROOT/load-image.sh"

tar --exclude='.DS_Store' -C "$REPO_ROOT/configs" -cf - environments | tar -C "$BUNDLE_ROOT/configs" -xf -

IMAGE_ARCHIVE_PATH="$BUNDLE_ROOT/images/$(image_bundle_name)" bash "$SCRIPT_DIR/export-image.sh"

mkdir -p "$RELEASE_DIR"
tar -czf "$IMAGE_BUNDLE" -C "$TMP_DIR" "$APP_NAME"

echo "[release] done: $IMAGE_BUNDLE"
