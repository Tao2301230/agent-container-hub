#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

. "$SCRIPT_DIR/scripts/program-common.sh"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      [[ $# -ge 2 ]] || program_die "missing value for --output-dir"
      CONFIG_DIR="$2"
      program_refresh_layout_paths
      shift 2
      ;;
    --config-dir|--data-dir|--state-dir|--log-dir|--bind-addr|--daemon)
      program_die "$1 is not a deploy argument"
      ;;
    *)
      program_die "unsupported deploy argument: $1"
      ;;
  esac
done

cd "$SCRIPT_DIR"
program_validate_bundle
program_initialize_config

echo "[program-deploy] bundle validated"
echo "[program-deploy] backend binary: $BACKEND_BIN"
echo "[program-deploy] config initialized under $CONFIG_DIR"
