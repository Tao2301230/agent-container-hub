#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

. "$SCRIPT_DIR/scripts/program-common.sh"

layout_args=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --state-dir)
      [[ $# -ge 2 ]] || program_die "missing value for --state-dir"
      layout_args+=(--state-dir "$2")
      shift 2
      ;;
    --config-dir|--data-dir|--log-dir|--bind-addr|--daemon)
      program_die "$1 is not a stop argument"
      ;;
    *)
      program_die "unsupported stop argument: $1"
      ;;
  esac
done

program_apply_layout_args "${layout_args[@]}"

cd "$SCRIPT_DIR"
program_validate_bundle
program_stop_backend
