#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
sed '$d' install-v2.sh > "$tmp/functions.sh"
source "$tmp/functions.sh"
printf abc > "$tmp/package"
CADDY_PACKAGE_SHA256='ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad'
verify_package "$tmp/package" caddy-test.tar.gz https://example.invalid/caddy-test.tar.gz
printf corrupted > "$tmp/package"
if verify_package "$tmp/package" caddy-test.tar.gz https://example.invalid/caddy-test.tar.gz; then
  echo 'FAIL: corrupted package accepted' >&2; exit 1
fi
echo 'PASS: checksum accepts exact bytes and rejects corruption'
