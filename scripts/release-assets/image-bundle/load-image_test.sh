#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOAD_SCRIPT="$ROOT_DIR/load-image.sh"
TMP_ROOT="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

assert_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "$haystack" == *"$needle"* ]] || {
    echo "expected output to contain: $needle" >&2
    echo "actual output:" >&2
    echo "$haystack" >&2
    exit 1
  }
}

make_bundle() {
  local bundle_dir="$1"
  mkdir -p "$bundle_dir/images"
  printf 'fake image payload\n' | gzip -c >"$bundle_dir/images/agent-container-hub-image-v0.1.0-linux-arm64.tar.gz"
}

make_engine_stub() {
  local bin_dir="$1"
  local name="$2"
  local log_file="$3"
  cat >"$bin_dir/$name" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s %s\n' "$name" "\$*" >>"$log_file"
cat >/dev/null
EOF
  chmod +x "$bin_dir/$name"
}

test_prefers_docker_when_available() {
  local bundle_dir="$TMP_ROOT/docker"
  local bin_dir="$bundle_dir/bin"
  local log_file="$bundle_dir/engine.log"
  make_bundle "$bundle_dir"
  mkdir -p "$bin_dir"
  make_engine_stub "$bin_dir" docker "$log_file"
  make_engine_stub "$bin_dir" podman "$log_file"

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bin_dir:/usr/bin:/bin" /bin/bash "$LOAD_SCRIPT" 2>&1
  )"
  assert_contains "$output" "image available as agent-container-hub:v0.1.0-linux-arm64"
  assert_contains "$(cat "$log_file")" "docker load"
}

test_falls_back_to_podman() {
  local bundle_dir="$TMP_ROOT/podman"
  local bin_dir="$bundle_dir/bin"
  local log_file="$bundle_dir/engine.log"
  make_bundle "$bundle_dir"
  mkdir -p "$bin_dir"
  make_engine_stub "$bin_dir" podman "$log_file"

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bin_dir:/usr/bin:/bin" /bin/bash "$LOAD_SCRIPT" 2>&1
  )"
  assert_contains "$output" "image available as agent-container-hub:v0.1.0-linux-arm64"
  assert_contains "$(cat "$log_file")" "podman load"
}

test_requires_engine_when_missing() {
  local bundle_dir="$TMP_ROOT/missing-engine"
  make_bundle "$bundle_dir"

  set +e
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="/usr/bin:/bin" /bin/bash "$LOAD_SCRIPT" 2>&1
  )"
  local status=$?
  set -e

  [[ $status -ne 0 ]] || {
    echo "expected load-image.sh to fail without docker or podman" >&2
    exit 1
  }
  assert_contains "$output" "docker or podman is required in PATH"
}

test_prefers_docker_when_available
test_falls_back_to_podman
test_requires_engine_when_missing

echo "load-image.sh tests passed"
