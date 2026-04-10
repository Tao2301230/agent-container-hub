#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# shellcheck disable=SC1091
. "$SCRIPT_DIR/release-common.sh"

resolve_release_context
require_image_tools

cd "$REPO_ROOT"

IMAGE_ARCHIVE_PATH="${IMAGE_ARCHIVE_PATH:-$RELEASE_DIR/$(image_bundle_name)}"

build_and_export_image_archive "$IMAGE_ARCHIVE_PATH"

echo "[release] done: $IMAGE_ARCHIVE_PATH"
