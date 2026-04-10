#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
START_SCRIPT="$ROOT_DIR/start.sh"
TMP_ROOT="$(mktemp -d)"
SAFE_PATH="/usr/bin:/bin:/usr/sbin:/sbin"

cleanup() {
  if [[ -d "$TMP_ROOT" ]]; then
    find "$TMP_ROOT" -name agent-container-hub.pid -print0 2>/dev/null | while IFS= read -r -d '' pid_file; do
      if [[ -f "$pid_file" ]]; then
        pid="$(cat "$pid_file" 2>/dev/null || true)"
        if [[ -n "${pid:-}" ]]; then
          kill "$pid" >/dev/null 2>&1 || true
        fi
      fi
    done
    rm -rf "$TMP_ROOT"
  fi
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
  mkdir -p "$bundle_dir/configs/environments"
  cat >"$bundle_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
ENGINE=local
EOF
  cp "$bundle_dir/.env" "$bundle_dir/.env.example"
  cat >"$bundle_dir/agent-container-hub" <<'EOF'
#!/usr/bin/env bash
sleep 5
EOF
  chmod +x "$bundle_dir/agent-container-hub"
  cp "$START_SCRIPT" "$bundle_dir/start.sh"
  chmod +x "$bundle_dir/start.sh"
}

test_local_mode_skips_engine_check() {
  local bundle_dir="$TMP_ROOT/local-mode"
  make_bundle "$bundle_dir"
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"
}

test_auto_detect_requires_engine_in_path() {
  local bundle_dir="$TMP_ROOT/auto-detect"
  make_bundle "$bundle_dir"
  cat >"$bundle_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
EOF

  set +e
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  local status=$?
  set -e

  [[ $status -ne 0 ]] || {
    echo "expected auto-detect startup to fail without docker or podman" >&2
    exit 1
  }
  assert_contains "$output" "docker or podman is required in PATH"
}

test_invalid_explicit_engine_fails_fast() {
  local bundle_dir="$TMP_ROOT/invalid-engine"
  make_bundle "$bundle_dir"
  cat >"$bundle_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
ENGINE=missing-engine
EOF

  set +e
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  local status=$?
  set -e

  [[ $status -ne 0 ]] || {
    echo "expected explicit missing engine to fail" >&2
    exit 1
  }
  assert_contains "$output" "ENGINE=missing-engine is not available in PATH"
}

test_local_mode_skips_engine_check
test_auto_detect_requires_engine_in_path
test_invalid_explicit_engine_fails_fast

echo "start.sh tests passed"
