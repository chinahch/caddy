#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
sed '$d' install-v2.sh > "$tmp/functions.sh"
source "$tmp/functions.sh"
INSTALL_ROOT="$tmp/etc"; CONFIG_FILE="$INSTALL_ROOT/config.yml"; CREDENTIALS_FILE="$INSTALL_ROOT/credentials.env"; META_FILE="$INSTALL_ROOT/install-meta.json"; CLI_PATH="$tmp/caddyctl"; BINARY_PATH="$tmp/caddy"
SYSTEMD_SERVICE_PATH="$tmp/caddy.service"; OPENRC_SERVICE_PATH="$tmp/caddy-openrc"; export CADDY_ALTERNATE_BINARY="$tmp/no-other-caddy" CADDY_DISTRIBUTION_UNIT="$tmp/no-other-unit"
export TEST_CAPTURE="$tmp/args" TEST_SECRET="$tmp/code"
cat > "$CLI_PATH" <<'CLI'
#!/bin/sh
test "$1" = machine-enroll || exit 4
test "${CADDY_ENROLLMENT_CODE:-}" = aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa || exit 5
printf '%s\n' "$@" > "$TEST_CAPTURE"
printf '%s' "$CADDY_ENROLLMENT_CODE" > "$TEST_SECRET"
CLI
chmod +x "$CLI_PATH"
printf '#!/bin/sh\necho dual-ip-v2\n' > "$BINARY_PATH";chmod +x "$BINARY_PATH"
# Isolate package/service operations; all assertions exercise the actual main path.
check_root(){ :; }; install_deps(){ :; }; read_connection_token(){ echo unexpected-prompt >&2;return 12; }
prepare_connection_cli(){ cp "$CLI_PATH" "$1";chmod +x "$1";printf '#!/bin/sh\necho dual-ip-v2\n' > "$2";chmod +x "$2"; }
start_service(){ touch "$tmp/restarted"; };cleanup_old_install(){ touch "$tmp/cleaned"; }
write_service(){ touch "$tmp/unit-written"; };write_uninstall_script(){ :; }
download_and_install_binary(){ touch "$tmp/downloaded"; }
detect_os_family(){ echo linux; };systemctl(){ return 1; }
# Fresh path: stage claim before installing binaries/services; no TTY or manual token.
rm -f "$BINARY_PATH"; mv "$CLI_PATH" "$tmp/fixture-cli"
prepare_connection_cli(){ cp "$tmp/fixture-cli" "$1";chmod +x "$1";printf '#!/bin/sh\necho dual-ip-v2\n' > "$2";chmod +x "$2"; }
main --mode machine --machine-id 7 --panel http://panel.example:8088 --enrollment-code aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa </dev/null
[[ -f "$tmp/restarted" && ! -f "$tmp/downloaded" && -f "$tmp/cleaned" ]]
! grep -q aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa "$TEST_CAPTURE"
grep -qx machine-enroll "$TEST_CAPTURE"
# Exact existing path delegates strict panel/ID/credential validation to native CLI.
printf '#!/bin/sh\necho dual-ip-v2\n' > "$BINARY_PATH";chmod +x "$BINARY_PATH"
mkdir -p "$INSTALL_ROOT";printf 'original-config\n' > "$CONFIG_FILE";printf 'original-credentials\n' > "$CREDENTIALS_FILE"
printf 'ExecStart=%s -c %s\nEnvironmentFile=-%s\n' "$BINARY_PATH" "$CONFIG_FILE" "$CREDENTIALS_FILE" > "$SYSTEMD_SERVICE_PATH"
rm -f "$tmp/restarted" "$tmp/downloaded" "$tmp/cleaned"
main --mode machine --machine-id 7 --panel http://panel.example:8088 --enrollment-code aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa </dev/null
[[ -f "$tmp/restarted" && ! -f "$tmp/downloaded" && ! -f "$tmp/cleaned" ]]
cmp "$CONFIG_FILE" <(printf 'original-config\n');cmp "$CREDENTIALS_FILE" <(printf 'original-credentials\n')
# Claim/local validation failure never restarts or cleans an existing runtime.
prepare_connection_cli(){ printf '#!/bin/sh\nexit 1\n' > "$1";chmod +x "$1"; }
rm -f "$tmp/restarted"
if (main --mode machine --machine-id 7 --panel http://panel.example:8088 --enrollment-code aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa) >/dev/null 2>&1;then echo 'failed enrollment accepted';exit 1;fi
[[ ! -f "$tmp/restarted" && ! -f "$tmp/cleaned" ]]
echo 'PASS: automatic enrollment is prompt-free, secretless argv, and preserves existing runtime on refusal'
