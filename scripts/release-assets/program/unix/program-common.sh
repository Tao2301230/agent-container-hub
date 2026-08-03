#!/usr/bin/env bash
set -euo pipefail

PROGRAM_COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_ROOT="$(cd "$PROGRAM_COMMON_DIR/.." && pwd)"
APP_NAME="agent-container-hub"
MANIFEST_FILE="$BUNDLE_ROOT/manifest.json"
ENV_EXAMPLE_FILE="$BUNDLE_ROOT/.env.example"
HUB_EXAMPLE_FILE="$BUNDLE_ROOT/configs/hub.example.yml"
BACKEND_BIN="$BUNDLE_ROOT/backend/$APP_NAME"
CONFIG_DIR="$BUNDLE_ROOT"
DATA_DIR="$BUNDLE_ROOT/data"
RUN_DIR="$BUNDLE_ROOT/run"
LOG_DIR="$RUN_DIR"
PROGRAM_BIND_ADDR=""
PROGRAM_CONFIG_DIR_EXPLICIT=0
PROGRAM_DATA_DIR_EXPLICIT=0
PROGRAM_STATE_DIR_EXPLICIT=0
PROGRAM_LOG_DIR_EXPLICIT=0
ENV_FILE=""
HUB_CONFIG_FILE=""
CONFIG_ENV_DIR=""
ROOTFS_DIR=""
BUILD_DIR=""
PID_FILE=""
LOG_FILE=""

program_refresh_layout_paths() {
  ENV_FILE="$CONFIG_DIR/.env"
  HUB_CONFIG_FILE="$CONFIG_DIR/configs/hub.yml"
  CONFIG_ENV_DIR="$CONFIG_DIR/configs/environments"
  ROOTFS_DIR="$DATA_DIR/rootfs"
  BUILD_DIR="$DATA_DIR/builds"
  PID_FILE="$RUN_DIR/$APP_NAME.pid"
  LOG_FILE="$LOG_DIR/$APP_NAME.log"
}

program_apply_layout_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --config-dir)
        [[ $# -ge 2 ]] || program_die "missing value for --config-dir"
        CONFIG_DIR="$2"
        PROGRAM_CONFIG_DIR_EXPLICIT=1
        shift 2
        ;;
      --data-dir)
        [[ $# -ge 2 ]] || program_die "missing value for --data-dir"
        DATA_DIR="$2"
        PROGRAM_DATA_DIR_EXPLICIT=1
        shift 2
        ;;
      --state-dir)
        [[ $# -ge 2 ]] || program_die "missing value for --state-dir"
        RUN_DIR="$2"
        PROGRAM_STATE_DIR_EXPLICIT=1
        if [[ "$LOG_DIR" == "$BUNDLE_ROOT/run" ]]; then
          LOG_DIR="$RUN_DIR"
        fi
        shift 2
        ;;
      --log-dir)
        [[ $# -ge 2 ]] || program_die "missing value for --log-dir"
        LOG_DIR="$2"
        PROGRAM_LOG_DIR_EXPLICIT=1
        shift 2
        ;;
      --bind-addr)
        [[ $# -ge 2 ]] || program_die "missing value for --bind-addr"
        PROGRAM_BIND_ADDR="$2"
        shift 2
        ;;
      *)
        program_die "unsupported argument: $1"
        ;;
    esac
  done
  program_refresh_layout_paths
}

program_refresh_layout_paths

program_die() {
  echo "[program] $*" >&2
  exit 1
}

program_require_file() {
  local path="$1"
  [[ -f "$path" ]] || program_die "required file not found: $path"
}

program_require_dir() {
  local path="$1"
  [[ -d "$path" ]] || program_die "required directory not found: $path"
}

program_validate_bundle() {
  program_require_file "$MANIFEST_FILE"
  program_require_file "$ENV_EXAMPLE_FILE"
  program_require_file "$HUB_EXAMPLE_FILE"
  [[ -x "$BACKEND_BIN" ]] || program_die "backend binary is not executable: $BACKEND_BIN"
}

program_initialize_config() {
  mkdir -p "$(dirname "$ENV_FILE")" "$(dirname "$HUB_CONFIG_FILE")" "$CONFIG_ENV_DIR"
  if [[ ! -f "$ENV_FILE" ]]; then
    cp "$ENV_EXAMPLE_FILE" "$ENV_FILE"
  fi
  if [[ ! -f "$HUB_CONFIG_FILE" ]]; then
    cp "$HUB_EXAMPLE_FILE" "$HUB_CONFIG_FILE"
  fi
  local source_env_dir="$BUNDLE_ROOT/configs/environments"
  if [[ -d "$source_env_dir" ]]; then
    local entry
    for entry in "$source_env_dir"/*; do
      [[ -e "$entry" ]] || continue
      local target="$CONFIG_ENV_DIR/$(basename "$entry")"
      if [[ ! -e "$target" ]]; then
        cp -R "$entry" "$target"
      fi
    done
  fi
}

program_validate_desktop_config_reset_args() {
  local backup_dir="$1"
  local version_from="$2"
  local version_to="$3"
  [[ "$backup_dir" == /* ]] || program_die "--desktop-config-backup-dir must be absolute"
  [[ -n "${version_from//[[:space:]]/}" ]] || program_die "missing value for --desktop-version-from"
  [[ -n "${version_to//[[:space:]]/}" ]] || program_die "missing value for --desktop-version-to"
  [[ "$backup_dir" != "$CONFIG_DIR" && "$backup_dir" != "$CONFIG_DIR/"* ]] || \
    program_die "Desktop config backup directory must be outside the service config directory"
}

program_secure_config_tree() {
  local target="$1"
  [[ -e "$target" ]] || return
  find "$target" -type d -exec chmod 700 {} +
  find "$target" -type f -exec chmod 600 {} +
}

program_reset_desktop_config() {
  local backup_dir="$1"
  local backup_parent
  local failed_dir="${backup_dir}.failed"
  backup_parent="$(dirname "$backup_dir")"
  mkdir -p "$backup_parent"
  chmod 700 "$backup_parent"

  if [[ -e "$backup_dir" ]]; then
    rm -rf "$failed_dir"
    if [[ -e "$CONFIG_DIR" ]]; then
      mv "$CONFIG_DIR" "$failed_dir"
      program_secure_config_tree "$failed_dir"
    fi
  elif [[ -e "$CONFIG_DIR" ]]; then
    mv "$CONFIG_DIR" "$backup_dir"
    program_secure_config_tree "$backup_dir"
  fi
  mkdir -p "$CONFIG_DIR"
  chmod 700 "$CONFIG_DIR"
}

program_read_env_literal_value() {
  local file="$1"
  local name="$2"
  [[ -f "$file" ]] || return 1
  awk -v name="$name" '
    $0 ~ "^[[:space:]]*(export[[:space:]]+)?" name "[[:space:]]*=" {
      line = $0
      sub("^[[:space:]]*(export[[:space:]]+)?" name "[[:space:]]*=", "", line)
      print line
      exit
    }
  ' "$file"
}

program_set_env_value() {
  local file="$1"
  local name="$2"
  local value="$3"
  local tmp="$file.tmp.$$"
  if ! awk -v name="$name" -v value="$value" '
    BEGIN { found = 0 }
    $0 ~ "^[[:space:]]*#?[[:space:]]*" name "[[:space:]]*=" {
      print name "=" value
      found = 1
      next
    }
    { print }
    END { if (!found) print name "=" value }
  ' "$file" >"$tmp"; then
    rm -f "$tmp"
    program_die "failed to update $name in $file"
  fi
  mv "$tmp" "$file"
}

program_load_env() {
  [[ -f "$ENV_FILE" ]] || program_die "missing .env (copy from .env.example first)"
  set -a
  # shellcheck disable=SC1091
  . "$ENV_FILE"
  set +a
}

program_probe_engine() {
  local engine="$1"
  "$engine" info >/dev/null 2>&1
}

program_check_engine() {
  if [[ -n "${ENGINE:-}" ]]; then
    if [[ "$ENGINE" == "loc""al" ]]; then
      program_die "ENGINE=""local has been removed; use auto, docker, or podman"
    fi
    if [[ "$ENGINE" != "auto" ]]; then
      command -v "$ENGINE" >/dev/null 2>&1 || program_die "ENGINE=$ENGINE is not available in PATH"
      program_probe_engine "$ENGINE" || program_die "ENGINE=$ENGINE daemon is not reachable"
      return
    fi
  fi
  for candidate in docker podman; do
    if command -v "$candidate" >/dev/null 2>&1 && program_probe_engine "$candidate"; then
      return
    fi
  done
  program_die "docker or podman is required in PATH and its daemon must be reachable"
}

program_prepare_runtime_dirs() {
  mkdir -p "$DATA_DIR" "$ROOTFS_DIR" "$BUILD_DIR" "$RUN_DIR" "$LOG_DIR"
}

program_read_pid() {
  [[ -f "$PID_FILE" ]] || return 1
  local pid
  pid="$(cat "$PID_FILE")"
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  printf '%s\n' "$pid"
}

program_backend_running() {
  local pid
  pid="$(program_read_pid)" || return 1
  kill -0 "$pid" >/dev/null 2>&1
}

program_clear_stale_pid() {
  if [[ ! -f "$PID_FILE" ]]; then
    return
  fi
  local pid
  pid="$(program_read_pid || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
    program_die "$APP_NAME is already running with pid $pid"
  fi
  rm -f "$PID_FILE"
}

program_start_backend_daemon() {
  local pid
  local backend_args=()
  if [[ "$PROGRAM_CONFIG_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--config-dir "$CONFIG_DIR")
  fi
  if [[ "$PROGRAM_DATA_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--data-dir "$DATA_DIR")
  fi
  if [[ "$PROGRAM_STATE_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--state-dir "$RUN_DIR")
  fi
  if [[ "$PROGRAM_LOG_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--log-dir "$LOG_DIR")
  fi
  if [[ -n "$PROGRAM_BIND_ADDR" ]]; then
    backend_args+=(--bind-addr "$PROGRAM_BIND_ADDR")
  elif [[ -n "${BIND_ADDR:-}" ]]; then
    backend_args+=(--bind-addr "$BIND_ADDR")
  fi

  program_clear_stale_pid
  : >"$LOG_FILE"
  if ((${#backend_args[@]} > 0)); then
    nohup "$BACKEND_BIN" "${backend_args[@]}" >>"$LOG_FILE" 2>&1 &
  else
    nohup "$BACKEND_BIN" >>"$LOG_FILE" 2>&1 &
  fi
  pid=$!
  printf '%s\n' "$pid" >"$PID_FILE"
  sleep 1
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    rm -f "$PID_FILE"
    program_die "backend failed to start; see $LOG_FILE"
  fi
  echo "[program-start] started $APP_NAME in daemon mode (pid=$pid)"
  echo "[program-start] log file: $LOG_FILE"
}

program_exec_backend() {
  local backend_args=()
  if [[ "$PROGRAM_CONFIG_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--config-dir "$CONFIG_DIR")
  fi
  if [[ "$PROGRAM_DATA_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--data-dir "$DATA_DIR")
  fi
  if [[ "$PROGRAM_STATE_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--state-dir "$RUN_DIR")
  fi
  if [[ "$PROGRAM_LOG_DIR_EXPLICIT" == "1" ]]; then
    backend_args+=(--log-dir "$LOG_DIR")
  fi
  if [[ -n "$PROGRAM_BIND_ADDR" ]]; then
    backend_args+=(--bind-addr "$PROGRAM_BIND_ADDR")
  elif [[ -n "${BIND_ADDR:-}" ]]; then
    backend_args+=(--bind-addr "$BIND_ADDR")
  fi
  if ((${#backend_args[@]} > 0)); then
    exec "$BACKEND_BIN" "${backend_args[@]}"
  fi
  exec "$BACKEND_BIN"
}

program_stop_backend() {
  local pid

  if [[ ! -f "$PID_FILE" ]]; then
    echo "[program-stop] pid file not found: $PID_FILE"
    return
  fi

  pid="$(program_read_pid || true)"
  [[ -n "$pid" ]] || program_die "pid file must contain a numeric pid: $PID_FILE"

  if ! kill -0 "$pid" >/dev/null 2>&1; then
    rm -f "$PID_FILE"
    echo "[program-stop] process $pid is not running; removed stale pid file"
    return
  fi

  kill "$pid"

  for _ in $(seq 1 30); do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      rm -f "$PID_FILE"
      echo "[program-stop] stopped $APP_NAME (pid=$pid)"
      return
    fi
    sleep 1
  done

  program_die "process $pid did not stop within 30s"
}
