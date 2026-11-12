#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/.."
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
sed '$d' install-v2.sh > "$tmp/functions.sh"
source "$tmp/functions.sh"
parse_args --mode machine --panel https://panel.example --machine-id 7 --prompt-token --rebind
[[ "$PROMPT_TOKEN" == true && "$REBIND" == true ]]
INSTALL_ROOT="$tmp/etc"; CONFIG_FILE="$INSTALL_ROOT/config.yml"; CREDENTIALS_FILE="$INSTALL_ROOT/credentials.env"; CLI_PATH="$tmp/caddyctl"; BINARY_PATH="$tmp/caddy"
mkdir -p "$INSTALL_ROOT"
printf 'instances:\n  - machine:\n      machine_id: 7 # primary\n' > "$CONFIG_FILE"
printf 'ORIGINAL=config\n' > "$CREDENTIALS_FILE"
cat > "$CLI_PATH" <<'EOF'
#!/bin/sh
test "$1" = machine-token
test "$CADDY_MACHINE_TOKEN" = test-token
printf '%s\n' "$@" > "$TEST_CAPTURE"
while [ "$#" -gt 0 ]; do if [ "$1" = --machine-id ];then test "$2" = 7 || exit 1;fi;shift;done
EOF
chmod +x "$CLI_PATH";printf '#!/bin/sh\nexit 0\n' > "$BINARY_PATH";chmod +x "$BINARY_PATH"
export TEST_CAPTURE="$tmp/args"
TOKEN=test-token
start_service(){ touch "$tmp/restarted"; }
prepare_connection_cli(){ cp "$CLI_PATH" "$1";chmod +x "$1"; }
rebind_existing_machine
test -f "$tmp/restarted"
grep -q -- '--credentials' "$TEST_CAPTURE"
grep -q -- '--config' "$TEST_CAPTURE"
if (MACHINE_ID=8; rebind_existing_machine) >/dev/null 2>&1; then echo 'mismatched identity rebound';exit 1;fi
cmp "$CREDENTIALS_FILE" <(printf 'ORIGINAL=config\n')
echo 'PASS: explicit native machine rebind preserves input config and refuses a different identity'
