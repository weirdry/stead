#!/usr/bin/env sh
set -eu

out="${1:-bin/stead}"
module="github.com/ed/stead"

version="${STEAD_VERSION:-}"
commit="${STEAD_COMMIT:-}"
date="${STEAD_DATE:-}"

if [ -z "$version" ]; then
  version=$(git describe --tags --always --dirty 2>/dev/null || printf 'dev')
fi

if [ -z "$commit" ]; then
  commit=$(git rev-parse --short HEAD 2>/dev/null || printf 'unknown')
fi

if [ -z "$date" ]; then
  date=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
fi

ldflags="-X ${module}/internal/version.Version=${version} -X ${module}/internal/version.Commit=${commit} -X ${module}/internal/version.Date=${date}"

go build -ldflags "$ldflags" -o "$out" ./cmd/stead
