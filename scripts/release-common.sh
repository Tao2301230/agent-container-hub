#!/usr/bin/env bash
set -euo pipefail

APP_NAME="agent-container-hub"

die() {
  echo "[release] $*" >&2
  exit 1
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) die "cannot detect ARCH from $(uname -m); pass ARCH=amd64|arm64" ;;
  esac
}

detect_host_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
    *) die "cannot detect TARGET_OS from $(uname -s); pass TARGET_OS=linux|darwin|windows" ;;
  esac
}

require_release_tools() {
  command -v go >/dev/null 2>&1 || die "go is required"
}

resolve_release_context() {
  VERSION="${VERSION:-$(cat "$REPO_ROOT/VERSION" 2>/dev/null || echo "dev")}"
  [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION must match vX.Y.Z (got: $VERSION)"

  ARCH="${ARCH:-$(detect_arch)}"
  case "$ARCH" in
    amd64|arm64) ;;
    *) die "ARCH must be amd64 or arm64 (got: $ARCH)" ;;
  esac

  RELEASE_DIR="$REPO_ROOT/dist/release"
}

validate_target_os() {
  case "$1" in
    linux|darwin|windows) ;;
    *) die "TARGET_OS must be linux, darwin, or windows (got: $1)" ;;
  esac
}

binary_name_for_os() {
  local target_os="$1"
  validate_target_os "$target_os"
  if [[ "$target_os" == "windows" ]]; then
    printf '%s.exe\n' "$APP_NAME"
    return
  fi
  printf '%s\n' "$APP_NAME"
}

parse_program_targets() {
  local raw="${PROGRAM_TARGETS:-darwin,windows}"
  raw="${raw//,/ }"
  for target in $raw; do
    validate_target_os "$target"
    printf '%s\n' "$target"
  done
}
