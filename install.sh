#!/usr/bin/env sh
set -eu

dry_run=0
for arg in "$@"; do
  case "$arg" in
    --dry-run)
      dry_run=1
      ;;
    *)
      echo "stead: unknown install option: $arg" >&2
      echo "usage: ./install.sh [--dry-run]" >&2
      exit 1
      ;;
  esac
done

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
src="${STEAD_BIN:-$script_dir/bin/stead}"
dst_dir="${STEAD_INSTALL_DIR:-$HOME/.local/bin}"
dst="$dst_dir/stead"

if [ "$dry_run" -eq 1 ]; then
  echo "stead install dry run"
  echo "source: $src"
  echo "target: $dst"
  if [ -z "${STEAD_BIN:-}" ]; then
    echo "build: would run go build -o bin/stead ./cmd/stead"
  fi
  echo "install: would copy binary to target"
  exit 0
fi

if [ -z "${STEAD_BIN:-}" ]; then
  (cd "$script_dir" && go build -o bin/stead ./cmd/stead)
fi

if [ ! -x "$src" ]; then
  echo "stead: missing built binary at $src" >&2
  exit 1
fi

mkdir -p "$dst_dir"
cp "$src" "$dst"
chmod 755 "$dst"

echo "installed $dst"

case ":$PATH:" in
  *":$dst_dir:"*)
    ;;
  *)
    echo "stead: warning: $dst_dir is not on PATH" >&2
    echo "stead: add it to your shell profile to run 'stead' from anywhere" >&2
    ;;
esac
