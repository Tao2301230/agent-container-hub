#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_SCRIPT="$ROOT_DIR/deploy.sh"
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
  cat >"$bundle_dir/configs/hub.example.yml" <<'EOF'
runtime:
  network_policy_helper_image: agent-container-hub/network-policy-helper:latest
EOF
  cat >"$bundle_dir/.env.example" <<'EOF'
# HTTP listen host and port.
# SERVER_HOST=127.0.0.1
# SERVER_PORT=8080

# Container engine: auto, docker, or podman.
# ENGINE=auto

# Required when SERVER_HOST is not localhost/loopback.
# AUTH_TOKEN=
EOF
  cp "$bundle_dir/.env.example" "$bundle_dir/.env"
  cat >"$bundle_dir/backend/agent-container-hub" <<'EOF'
#!/usr/bin/env bash
sleep 5
EOF
  chmod +x "$bundle_dir/backend/agent-container-hub"
  cp "$DEPLOY_SCRIPT" "$bundle_dir/deploy.sh"
  cp "$START_SCRIPT" "$bundle_dir/start.sh"
  cp "$STOP_SCRIPT" "$bundle_dir/stop.sh"
  cp "$PROGRAM_COMMON_SCRIPT" "$bundle_dir/scripts/program-common.sh"
  chmod +x "$bundle_dir/deploy.sh"
  chmod +x "$bundle_dir/start.sh"
  chmod +x "$bundle_dir/stop.sh"
  chmod +x "$bundle_dir/scripts/program-common.sh"
}

test_deploy_initializes_env_and_hub_config() {
  local bundle_dir="$TMP_ROOT/deploy-init"
  make_bundle "$bundle_dir"
  rm -f "$bundle_dir/.env" "$bundle_dir/configs/hub.yml"

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" /bin/bash ./deploy.sh 2>&1
  )"
  assert_contains "$output" "bundle validated"
  [[ -f "$bundle_dir/.env" ]] || { echo "expected .env to be created" >&2; exit 1; }
  [[ -f "$bundle_dir/configs/hub.yml" ]] || { echo "expected configs/hub.yml to be created" >&2; exit 1; }
  assert_contains "$(cat "$bundle_dir/.env")" "SERVER_PORT=8080"
  assert_contains "$(cat "$bundle_dir/.env")" "ENGINE=auto"
  if grep -q '^server:' "$bundle_dir/configs/hub.yml"; then
    echo "expected configs/hub.yml to omit server config" >&2
    exit 1
  fi
  if grep -qE '^  engine:' "$bundle_dir/configs/hub.yml"; then
    echo "expected configs/hub.yml to omit runtime.engine" >&2
    exit 1
  fi
  if ! grep -qE '^(# )?SERVER_HOST=' "$bundle_dir/.env.example" || ! grep -qE '^(# )?SERVER_PORT=' "$bundle_dir/.env.example" || ! grep -qE '^(# )?ENGINE=' "$bundle_dir/.env.example"; then
    echo "expected .env.example to contain SERVER_HOST, SERVER_PORT, and ENGINE" >&2
    exit 1
  fi
  if grep -qE '^(# )?(BIND_ADDR|HUB_CONFIG_PATH)=' "$bundle_dir/.env.example"; then
    echo "expected .env.example to omit BIND_ADDR and HUB_CONFIG_PATH" >&2
    exit 1
  fi
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
BIND_ADDR=127.0.0.1:8080
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
BIND_ADDR=127.0.0.1:8080
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
BIND_ADDR=127.0.0.1:8080
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
BIND_ADDR=127.0.0.1:8080
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
  printf 'BIND_ADDR=127.0.0.1:8080\nENGINE=%s\n' "local" >"$bundle_dir/.env"

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
BIND_ADDR=127.0.0.1:8080
ENGINE=docker
EOF
  cat >"$config_dir/configs/environments/shell/environment.yml" <<'EOF'
name: custom-shell
EOF

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" \
      /bin/bash ./start.sh --daemon \
        --config-dir "$config_dir" \
        --data-dir "$data_dir" \
        --state-dir "$state_dir" \
        --log-dir "$log_dir" \
        --bind-addr 127.0.0.1:8080 2>&1
  )"
  assert_contains "$output" "started agent-container-hub in daemon mode"
  assert_contains "$(cat "$config_dir/configs/environments/shell/environment.yml")" "custom-shell"
  [[ -d "$data_dir/rootfs" ]] || { echo "expected external rootfs dir to be created" >&2; exit 1; }
  [[ -f "$state_dir/agent-container-hub.pid" ]] || { echo "expected external pid file to be created" >&2; exit 1; }
  [[ -f "$log_dir/agent-container-hub.log" ]] || { echo "expected external log file to be created" >&2; exit 1; }

  output="$(
    cd "$bundle_dir" &&
      PATH="$bundle_dir/bin:$SAFE_PATH" \
      /bin/bash ./stop.sh \
        --state-dir "$state_dir" \
        --log-dir "$log_dir" 2>&1
  )"
  assert_contains "$output" "stopped agent-container-hub"
}

test_external_deploy_initialization_is_idempotent() {
  local bundle_dir="$TMP_ROOT/external-deploy-idempotent"
  local config_dir="$TMP_ROOT/external-deploy-config"
  local data_dir="$TMP_ROOT/external-deploy-data"
  local state_dir="$TMP_ROOT/external-deploy-state"
  local log_dir="$TMP_ROOT/external-deploy-logs"
  make_bundle "$bundle_dir"
  mkdir -p "$config_dir/configs/environments/shell"
  cat >"$config_dir/.env" <<'EOF'
AUTH_TOKEN=custom-token
EOF
  cat >"$config_dir/configs/hub.yml" <<'EOF'
server:
  port: 13000
EOF
  cat >"$config_dir/configs/environments/shell/environment.yml" <<'EOF'
name: custom-shell
EOF

  local output
  output="$(
    cd "$bundle_dir" &&
      PATH="$SAFE_PATH" \
      /bin/bash ./deploy.sh \
        --config-dir "$config_dir" \
        --data-dir "$data_dir" \
        --state-dir "$state_dir" \
        --log-dir "$log_dir" 2>&1
  )"
  assert_contains "$output" "bundle validated"
  assert_contains "$(cat "$config_dir/.env")" "custom-token"
  assert_contains "$(cat "$config_dir/configs/hub.yml")" "13000"
  assert_contains "$(cat "$config_dir/configs/environments/shell/environment.yml")" "custom-shell"
  [[ -d "$data_dir/rootfs" ]] || { echo "expected external rootfs dir to be created" >&2; exit 1; }
  [[ -d "$state_dir" ]] || { echo "expected external state dir to be created" >&2; exit 1; }
  [[ -d "$log_dir" ]] || { echo "expected external log dir to be created" >&2; exit 1; }
}

test_deploy_initializes_env_and_hub_config
test_missing_env_fails_fast
test_explicit_engine_creates_runtime_dirs_and_stops_cleanly
test_invalid_pid_file_is_treated_as_stale
test_auto_detect_requires_engine_in_path
test_auto_detect_succeeds_with_fake_engine
test_auto_alias_succeeds_with_fake_engine
test_invalid_explicit_engine_fails_fast
test_removed_local_engine_fails_fast
test_external_config_initialization_is_idempotent
test_external_deploy_initialization_is_idempotent

echo "start.sh tests passed"
