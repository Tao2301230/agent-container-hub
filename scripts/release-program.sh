#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_ASSETS_DIR="$SCRIPT_DIR/release-assets"
WINDOWS_RELEASE_SCRIPTS_DIR="$REPO_ROOT/release-scripts/windows"

# shellcheck disable=SC1091
. "$SCRIPT_DIR/release-common.sh"

require_release_tools
command -v tar >/dev/null 2>&1 || die "tar is required"
resolve_release_context

cd "$REPO_ROOT"

build_program_bundle() {
  local target_os="$1"
  local binary_name
  local bundle_name
  local bundle_tar
  local tmp_dir
  local bundle_root

  binary_name="$(binary_name_for_os "$target_os")"
  bundle_name="${APP_NAME}-program-${VERSION}-${target_os}-${ARCH}"
  bundle_tar="$RELEASE_DIR/${bundle_name}.tar.gz"

  echo "[release] program VERSION=$VERSION TARGET_OS=$target_os ARCH=$ARCH"

  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/agent-container-hub-program-release.XXXXXX")"
  trap 'rm -rf "$tmp_dir"' RETURN

  bundle_root="$tmp_dir/$APP_NAME"
  mkdir -p \
    "$bundle_root/configs" \
    "$bundle_root/data/rootfs" \
    "$bundle_root/data/builds"

  echo "[release] building program binary for $target_os..."
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$ARCH" \
    go build \
    -ldflags "-X main.buildVersion=$VERSION" \
    -o "$bundle_root/$binary_name" \
    ./cmd/agent-container-hub

  echo "[release] assembling program bundle for $target_os..."
  cp "$REPO_ROOT/.env.example" "$bundle_root/.env.example"
  cp "$RELEASE_ASSETS_DIR/README.txt" "$bundle_root/README.txt"

  if [[ "$target_os" == "windows" ]]; then
    mkdir -p "$bundle_root/release-scripts/windows"
    cp "$WINDOWS_RELEASE_SCRIPTS_DIR/start.ps1" "$bundle_root/release-scripts/windows/start.ps1"
    cp "$WINDOWS_RELEASE_SCRIPTS_DIR/stop.ps1" "$bundle_root/release-scripts/windows/stop.ps1"
    cp "$WINDOWS_RELEASE_SCRIPTS_DIR/start.cmd" "$bundle_root/release-scripts/windows/start.cmd"
    cp "$WINDOWS_RELEASE_SCRIPTS_DIR/stop.cmd" "$bundle_root/release-scripts/windows/stop.cmd"
  else
    cp "$RELEASE_ASSETS_DIR/start.sh" "$bundle_root/start.sh"
    cp "$RELEASE_ASSETS_DIR/stop.sh" "$bundle_root/stop.sh"
    chmod +x "$bundle_root/$binary_name" "$bundle_root/start.sh" "$bundle_root/stop.sh"
  fi

  if [[ "$target_os" == "linux" ]]; then
    mkdir -p "$bundle_root/systemd"
    cp "$RELEASE_ASSETS_DIR/systemd/agent-container-hub.service" "$bundle_root/systemd/agent-container-hub.service"
  fi

  tar --exclude='.DS_Store' -C "$REPO_ROOT/configs" -cf - environments | tar -C "$bundle_root/configs" -xf -

  mkdir -p "$RELEASE_DIR"
  tar -czf "$bundle_tar" -C "$tmp_dir" "$APP_NAME"

  echo "[release] done: $bundle_tar"
  rm -rf "$tmp_dir"
  trap - RETURN
}

while IFS= read -r target_os; do
  [[ -n "$target_os" ]] || continue
  build_program_bundle "$target_os"
done < <(parse_program_targets)
