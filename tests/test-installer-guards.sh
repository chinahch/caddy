#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
sed '$d' install-v2.sh > "$tmp/functions.sh"
source "$tmp/functions.sh"
INSTALL_ROOT="$tmp/etc-caddy"
CONFIG_FILE="$INSTALL_ROOT/config.yml"
CREDENTIALS_FILE="$INSTALL_ROOT/credentials.env"
BINARY_PATH="$tmp/bin/caddy"
CLI_PATH="$tmp/bin/caddyctl"
SYSTEMD_SERVICE_PATH="$tmp/service/caddy.service"
OPENRC_SERVICE_PATH="$tmp/init/caddy"
CADDY_ALTERNATE_BINARY="$tmp/alternate/caddy"
CADDY_DISTRIBUTION_UNIT="$tmp/distro/caddy.service"
systemctl(){ return 1; }
mkdir -p "$INSTALL_ROOT" "$(dirname "$SYSTEMD_SERVICE_PATH")" "$(dirname "$CADDY_ALTERNATE_BINARY")"
printf '[Service]\nExecStart=/usr/bin/caddy run\n' > "$SYSTEMD_SERVICE_PATH"
cp "$SYSTEMD_SERVICE_PATH" "$tmp/original-unit"
if ( if preserve_existing_install; then :; else touch "$tmp/fresh-install"; fi ); then :; fi
if [ -e "$tmp/fresh-install" ]; then echo 'FAIL: web server was treated as fresh install' >&2; exit 1; fi
cmp "$SYSTEMD_SERVICE_PATH" "$tmp/original-unit"
if ( upgrade_existing_machine ); then echo 'FAIL: unsafe automatic upgrade enabled' >&2; exit 1; fi
echo 'PASS: conflicting service refused and unverified auto-upgrade blocked'
