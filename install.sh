#!/usr/bin/env sh
set -eu

src="${STEAD_BIN:-bin/stead}"
dst_dir="${STEAD_INSTALL_DIR:-$HOME/.local/bin}"
dst="$dst_dir/stead"

if [ ! -x "$src" ]; then
  echo "stead: missing built binary at $src" >&2
  echo "stead: run 'just build' first" >&2
  exit 1
fi

mkdir -p "$dst_dir"
cp "$src" "$dst"
chmod 755 "$dst"

echo "installed $dst"
