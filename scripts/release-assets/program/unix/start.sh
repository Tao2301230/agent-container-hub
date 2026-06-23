#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

. "$SCRIPT_DIR/scripts/program-common.sh"

main() {
  local mode=""
  local layout_args=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --daemon)
        mode="--daemon"
        shift
        ;;
      --config-dir|--data-dir|--state-dir|--log-dir|--bind-addr)
        [[ $# -ge 2 ]] || program_die "missing value for $1"
        layout_args+=("$1" "$2")
        shift 2
        ;;
      *)
        program_die "unsupported argument: $1"
        ;;
    esac
  done
  if ((${#layout_args[@]} > 0)); then
    program_apply_layout_args "${layout_args[@]}"
  else
    program_apply_layout_args
  fi

  cd "$SCRIPT_DIR"
  program_validate_bundle
  program_load_env
  program_prepare_runtime_dirs
  program_check_engine

  if [[ "$mode" == "--daemon" ]]; then
    program_start_backend_daemon
    return
  fi

  program_exec_backend
}

main "$@"
