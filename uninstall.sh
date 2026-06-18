#!/usr/bin/env sh
set -eu

dry_run=0
for arg in "$@"; do
  case "$arg" in
    --dry-run)
      dry_run=1
      ;;
    *)
      echo "stead: unknown uninstall option: $arg" >&2
      echo "usage: ./uninstall.sh [--dry-run]" >&2
      exit 1
      ;;
  esac
done

dst_dir="${STEAD_INSTALL_DIR:-$HOME/.local/bin}"
dst="$dst_dir/stead"

if [ "$dry_run" -eq 1 ]; then
  echo "stead uninstall dry run"
  echo "target: $dst"
  if [ -e "$dst" ]; then
    echo "uninstall: would remove installed binary"
  else
    echo "uninstall: no installed binary found"
  fi
  echo "note: config, SSH keys, and SSH config are not removed by this script"
  exit 0
fi

if [ -e "$dst" ]; then
  rm "$dst"
  echo "removed $dst"
else
  echo "stead: no installed binary at $dst"
fi

echo "note: config, SSH keys, and SSH config were not removed"
