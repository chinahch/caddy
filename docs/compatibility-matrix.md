# Compatibility and verification matrix

| Capability | Code evidence | Initial verification |
|---|---|---|
| caddy/caddyctl paths | install-v2.sh; prepare_source.py CLI transforms | Source preparation tests and cross compilation passed; Linux CLI startup pending |
| Node/Machine | pinned upstream internal/service and internal/machine; XHTTP patch | Config/controlplane tests passed; live Machine process pending |
| sing-box/Xray | pinned core packages | Their unit tests passed on Windows; Linux binaries compiled |
| Native dual IP/strict-only | apply_ipfamily.py; strict_ipfamily.py | Included generated kernel regression tests passed |
| XHTTP/TLS | xhttp-v2-fixes.patch; xhttp_tls_v2_test.go | Xray tests passed; real client/proxy connection pending |
| Linux amd64 | scripts/build.py | Cross compilation passed; systemd/OpenRC runtime pending |
| Linux arm64 | scripts/build.py | Cross compilation passed; runtime pending |
| Alpine ARM64 128 MiB | DOMAIN-AUDIT-README.md | Known prior crash; not validated |
| Audit | opt-in overlay | Default off; Linux permission tests pending |
| Installer checksum | verify_package; test-checksum.sh | Exact bytes accepted, corruption rejected |
| Installer preservation | test-install-v2.sh; test-installer-guards.sh | Windows Git Bash regression passed after unsafe automatic upgrade was disabled; Linux service runtime pending |
| CLI YAML rewrite | cli_roundtrip_test.go | Shared and per-instance strict IP/custom kernel paths preserved |
| Luma REST compatibility | fixed upstream internal/panel/client.go | HTTP contract tests and subsequent integration results documented in Luma |
| Reliable accounting transport | old node reporting chain lacks durable batch IDs | Not implemented in this node build; legacy quality must remain visible |
