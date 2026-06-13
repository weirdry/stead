#!/usr/bin/env sh
set -eu

dst_dir="${STEAD_INSTALL_DIR:-$HOME/.local/bin}"
dst="$dst_dir/stead"

if [ -e "$dst" ]; then
  rm "$dst"
  echo "removed $dst"
else
  echo "stead: no installed binary at $dst"
fi
