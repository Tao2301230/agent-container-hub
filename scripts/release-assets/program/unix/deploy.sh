#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

. "$SCRIPT_DIR/scripts/program-common.sh"

desktop_config_reset=0
desktop_config_backup_dir=""
desktop_version_from=""
desktop_version_to=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      [[ $# -ge 2 ]] || program_die "missing value for --output-dir"
      CONFIG_DIR="$2"
      program_refresh_layout_paths
      shift 2
      ;;
    --desktop-config-reset)
      desktop_config_reset=1
      shift
      ;;
    --desktop-config-backup-dir)
      [[ $# -ge 2 ]] || program_die "missing value for --desktop-config-backup-dir"
      desktop_config_backup_dir="$2"
      shift 2
      ;;
    --desktop-version-from)
      [[ $# -ge 2 ]] || program_die "missing value for --desktop-version-from"
      desktop_version_from="$2"
      shift 2
      ;;
    --desktop-version-to)
      [[ $# -ge 2 ]] || program_die "missing value for --desktop-version-to"
      desktop_version_to="$2"
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
auth_token=""
if [[ "$desktop_config_reset" == "1" ]]; then
  program_validate_desktop_config_reset_args \
    "$desktop_config_backup_dir" \
    "$desktop_version_from" \
    "$desktop_version_to"
  program_reset_desktop_config "$desktop_config_backup_dir"
  auth_token="$(program_read_env_literal_value "$desktop_config_backup_dir/.env" "AUTH_TOKEN" || true)"
fi
program_initialize_config
if [[ "$desktop_config_reset" == "1" && -n "$auth_token" ]]; then
  program_set_env_value "$ENV_FILE" "AUTH_TOKEN" "$auth_token"
fi
if [[ "$desktop_config_reset" == "1" ]]; then
  program_secure_config_tree "$CONFIG_DIR"
fi

echo "[program-deploy] bundle validated"
echo "[program-deploy] backend binary: $BACKEND_BIN"
echo "[program-deploy] config initialized under $CONFIG_DIR"
if [[ "$desktop_config_reset" == "1" ]]; then
  echo "[program-deploy] Desktop config rebuilt: $desktop_version_from -> $desktop_version_to"
fi
