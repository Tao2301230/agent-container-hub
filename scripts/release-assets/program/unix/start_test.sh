#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
START_SCRIPT="$ROOT_DIR/start.sh"
STOP_SCRIPT="$ROOT_DIR/stop.sh"
PROGRAM_COMMON_SCRIPT="$ROOT_DIR/program-common.sh"
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
  mkdir -p "$bundle_dir/backend" "$bundle_dir/bin" "$bundle_dir/configs/environments/shell" "$bundle_dir/scripts"
  cat >"$bundle_dir/manifest.json" <<'EOF'
{"id":"agent-container-hub"}
EOF
  cat >"$bundle_dir/configs/environments/shell/environment.yml" <<'EOF'
name: shell
EOF
  cat >"$bundle_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
ENGINE=auto
EOF
  cp "$bundle_dir/.env" "$bundle_dir/.env.example"
  cat >"$bundle_dir/backend/agent-container-hub" <<'EOF'
#!/usr/bin/env bash
sleep 5
EOF
  chmod +x "$bundle_dir/backend/agent-container-hub"
  cp "$START_SCRIPT" "$bundle_dir/start.sh"
  cp "$STOP_SCRIPT" "$bundle_dir/stop.sh"
  cp "$PROGRAM_COMMON_SCRIPT" "$bundle_dir/scripts/program-common.sh"
  chmod +x "$bundle_dir/start.sh"
  chmod +x "$bundle_dir/stop.sh"
  chmod +x "$bundle_dir/scripts/program-common.sh"
}

make_fake_engine() {
  local bundle_dir="$1"
  cat >"$bundle_dir/bin/docker" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "info" ]]; then
  exit 0
fi
exit 1
EOF
  chmod +x "$bundle_dir/bin/docker"
}

test_missing_env_fails_fast() {
  local bundle_dir="$TMP_ROOT/missing-env"
  make_bundle "$bundle_dir"
  rm -f "$bundle_dir/.env"

  set +e
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  local status=$?
  set -e

  [[ $status -ne 0 ]] || {
    echo "expected startup to fail when .env is missing" >&2
    exit 1
  }
  assert_contains "$output" "missing .env"
}

test_explicit_engine_creates_runtime_dirs_and_stops_cleanly() {
  local bundle_dir="$TMP_ROOT/explicit-engine"
  make_bundle "$bundle_dir"
  make_fake_engine "$bundle_dir"
  cat >"$bundle_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
ENGINE=docker
EOF
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"
  [[ -d "$bundle_dir/data" ]] || { echo "expected data dir to be created" >&2; exit 1; }
  [[ -d "$bundle_dir/data/rootfs" ]] || { echo "expected rootfs dir to be created" >&2; exit 1; }
  [[ -d "$bundle_dir/data/builds" ]] || { echo "expected builds dir to be created" >&2; exit 1; }
  [[ -d "$bundle_dir/run" ]] || { echo "expected run dir to be created" >&2; exit 1; }
  [[ -f "$bundle_dir/run/agent-container-hub.pid" ]] || { echo "expected pid file to be created" >&2; exit 1; }
  [[ -f "$bundle_dir/run/agent-container-hub.log" ]] || { echo "expected log file to be created" >&2; exit 1; }

  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./stop.sh 2>&1
  )"
  assert_contains "$output" "stopped agent-container-hub"
}

test_invalid_pid_file_is_treated_as_stale() {
  local bundle_dir="$TMP_ROOT/invalid-pid"
  make_bundle "$bundle_dir"
  make_fake_engine "$bundle_dir"
  mkdir -p "$bundle_dir/run"
  printf 'not-a-pid\n' >"$bundle_dir/run/agent-container-hub.pid"

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"
  [[ -f "$bundle_dir/run/agent-container-hub.pid" ]] || { echo "expected pid file to be recreated" >&2; exit 1; }

  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./stop.sh 2>&1
  )"
  assert_contains "$output" "stopped agent-container-hub"
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

test_auto_detect_succeeds_with_fake_engine() {
  local bundle_dir="$TMP_ROOT/auto-detect-success"
  make_bundle "$bundle_dir"
  make_fake_engine "$bundle_dir"
  cat >"$bundle_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
EOF

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"

  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./stop.sh 2>&1
  )"
  assert_contains "$output" "stopped agent-container-hub"
}

test_auto_alias_succeeds_with_fake_engine() {
  local bundle_dir="$TMP_ROOT/auto-alias-success"
  make_bundle "$bundle_dir"
  make_fake_engine "$bundle_dir"

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"

  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" /bin/bash ./stop.sh 2>&1
  )"
  assert_contains "$output" "stopped agent-container-hub"
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

test_removed_local_engine_fails_fast() {
  local bundle_dir="$TMP_ROOT/removed-local-engine"
  make_bundle "$bundle_dir"
  printf 'BIND_ADDR=127.0.0.1:11960\nENGINE=%s\n' "local" >"$bundle_dir/.env"

  set +e
  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" /bin/bash ./start.sh --daemon 2>&1
  )"
  local status=$?
  set -e

  [[ $status -ne 0 ]] || {
    echo "expected removed local engine to fail" >&2
    exit 1
  }
  assert_contains "$output" "ENGINE=""local has been removed; use auto, docker, or podman"
}

test_external_config_initialization_is_idempotent() {
  local bundle_dir="$TMP_ROOT/external-config-idempotent"
  local config_dir="$TMP_ROOT/external-config"
  local data_dir="$TMP_ROOT/external-data"
  local state_dir="$TMP_ROOT/external-state"
  local log_dir="$TMP_ROOT/external-logs"
  make_bundle "$bundle_dir"
  make_fake_engine "$bundle_dir"
  mkdir -p "$config_dir/configs/environments/shell"
  cat >"$config_dir/.env" <<'EOF'
BIND_ADDR=127.0.0.1:11960
ENGINE=docker
EOF
  cat >"$config_dir/configs/environments/shell/environment.yml" <<'EOF'
name: custom-shell
EOF

  local output
  output="$(
    cd "$bundle_dir" &&
      SERVICE_CONFIG_DIR="$config_dir" \
      SERVICE_DATA_DIR="$data_dir" \
      SERVICE_STATE_DIR="$state_dir" \
      SERVICE_LOG_DIR="$log_dir" \
      PATH="$bundle_dir/bin:$SAFE_PATH" \
      /bin/bash ./start.sh --daemon 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"
  assert_contains "$(cat "$config_dir/configs/environments/shell/environment.yml")" "custom-shell"
  [[ -d "$data_dir/rootfs" ]] || { echo "expected external rootfs dir to be created" >&2; exit 1; }
  [[ -f "$state_dir/agent-container-hub.pid" ]] || { echo "expected external pid file to be created" >&2; exit 1; }
  [[ -f "$log_dir/agent-container-hub.log" ]] || { echo "expected external log file to be created" >&2; exit 1; }

  output="$(
    cd "$bundle_dir" &&
      SERVICE_STATE_DIR="$state_dir" \
      SERVICE_LOG_DIR="$log_dir" \
      PATH="$bundle_dir/bin:$SAFE_PATH" \
      /bin/bash ./stop.sh 2>&1
  )"
  assert_contains "$output" "stopped agent-container-hub"
}

test_missing_env_fails_fast
test_explicit_engine_creates_runtime_dirs_and_stops_cleanly
test_invalid_pid_file_is_treated_as_stale
test_auto_detect_requires_engine_in_path
test_auto_detect_succeeds_with_fake_engine
test_auto_alias_succeeds_with_fake_engine
test_invalid_explicit_engine_fails_fast
test_removed_local_engine_fails_fast
test_external_config_initialization_is_idempotent

echo "start.sh tests passed"
