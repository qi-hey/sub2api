#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')"
fi
if [ -z "$VERSION" ]; then
  echo "version is required" >&2
  exit 1
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if ROOT_DIR="$(git -C "$SCRIPT_DIR/.." rev-parse --show-toplevel 2>/dev/null)"; then
  :
else
  ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
fi

cd "$ROOT_DIR"

if ! command -v pnpm >/dev/null 2>&1; then
  if command -v corepack >/dev/null 2>&1; then
    corepack enable
    corepack prepare pnpm@9.15.9 --activate
  else
    echo "pnpm is required to build the embedded frontend" >&2
    exit 1
  fi
fi

cd frontend
pnpm install --frozen-lockfile
pnpm run build

cd ../backend
printf '%s\n' "$VERSION" > cmd/server/VERSION

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -tags embed \
  -ldflags="-s -w -X main.Version=$VERSION" \
  -trimpath \
  -o "../build/sub2api-privacyfilter-$VERSION-linux-amd64" \
  ./cmd/server
