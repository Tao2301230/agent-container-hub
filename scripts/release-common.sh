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

require_image_tools() {
  command -v docker >/dev/null 2>&1 || die "docker is required"
  command -v gzip >/dev/null 2>&1 || die "gzip is required"
}

resolve_release_context() {
  VERSION="${VERSION:-$(cat "$REPO_ROOT/VERSION" 2>/dev/null || echo "dev")}"
  [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION must match vX.Y.Z (got: $VERSION)"

  ARCH="${ARCH:-$(detect_arch)}"
  validate_arch "$ARCH"

  RELEASE_DIR="$REPO_ROOT/dist/release"
}

validate_arch() {
  case "$1" in
    amd64|arm64) ;;
    *) die "ARCH must be amd64 or arm64 (got: $1)" ;;
  esac
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

parse_program_target_matrix() {
  local raw="${PROGRAM_TARGET_MATRIX:-}"
  local target_spec
  local target_os
  local target_arch

  if [[ -n "$raw" ]]; then
    raw="${raw//,/ }"
    for target_spec in $raw; do
      [[ "$target_spec" == */* ]] || die "PROGRAM_TARGET_MATRIX entries must look like <os>/<arch> (got: $target_spec)"
      target_os="${target_spec%%/*}"
      target_arch="${target_spec#*/}"
      validate_target_os "$target_os"
      validate_arch "$target_arch"
      printf '%s %s\n' "$target_os" "$target_arch"
    done
    return
  fi

  if [[ -n "${PROGRAM_TARGETS:-}" ]]; then
    while IFS= read -r target_os; do
      [[ -n "$target_os" ]] || continue
      printf '%s %s\n' "$target_os" "$ARCH"
    done < <(parse_program_targets)
    return
  fi

  printf 'darwin arm64\n'
  printf 'windows amd64\n'
}

image_target_os() {
  printf 'linux\n'
}

image_tag() {
  printf '%s:%s-%s-%s\n' "$APP_NAME" "$VERSION" "$(image_target_os)" "$ARCH"
}

image_archive_name() {
  printf '%s-image-%s-%s-%s.tar.gz\n' "$APP_NAME" "$VERSION" "$(image_target_os)" "$ARCH"
}

image_bundle_name() {
  printf '%s-image-bundle-%s-%s-%s.tar.gz\n' "$APP_NAME" "$VERSION" "$(image_target_os)" "$ARCH"
}

build_and_export_image_archive() {
  local output_path="$1"
  local target_os
  local tag

  target_os="$(image_target_os)"
  tag="$(image_tag)"

  mkdir -p "$(dirname "$output_path")"

  echo "[release] image VERSION=$VERSION TARGET_OS=$target_os ARCH=$ARCH"

  docker build \
    --platform "${target_os}/${ARCH}" \
    --build-arg VERSION="$VERSION" \
    -t "$tag" \
    .

  docker save "$tag" | gzip -c >"$output_path"
}
