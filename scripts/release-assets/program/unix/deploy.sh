#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

. "$SCRIPT_DIR/scripts/program-common.sh"

layout_args=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      [[ $# -ge 2 ]] || program_die "missing value for --output-dir"
      layout_args+=(--config-dir "$2")
      shift 2
      ;;
    --data-dir|--state-dir|--log-dir|--bind-addr)
      [[ $# -ge 2 ]] || program_die "missing value for $1"
      layout_args+=("$1" "$2")
      shift 2
      ;;
    --config-dir|--daemon)
      program_die "$1 is not a deploy argument"
      ;;
    *)
      program_die "unsupported deploy argument: $1"
      ;;
  esac
done

program_apply_layout_args "${layout_args[@]}"

cd "$SCRIPT_DIR"
program_validate_bundle
program_initialize_config
program_prepare_runtime_dirs

echo "[program-deploy] bundle validated"
echo "[program-deploy] backend binary: $BACKEND_BIN"
echo "[program-deploy] runtime directories prepared under $DATA_DIR and $RUN_DIR"
