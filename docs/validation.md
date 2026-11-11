# Initial validation (2026-11-12)

Toolchain: Go 1.26.10. Go 1.26.3 hit an internal compiler panic in an AWS dependency, so results are not attributed to that compiler.

- Pinned source preparation and normalized package tests passed.
- Real CLI YAML round-trip passed for shared/per-instance `ip_family` and per-kernel custom config paths.
- Bash syntax, checksum corruption rejection, unknown service collision guard, existing Machine configuration/credentials preservation and blocked unsafe upgrade regressions passed in Windows Git Bash.
- Fixed-source config, tracker, sing-box, Xray and controlplane tests passed; Machine package compiled and contains no test files in this snapshot.
- Linux amd64 and arm64 cross compilation passed before the CLI serialization fix. The fix's CLI package test passed; final package rebuild evidence is reported with the exact final build artifacts rather than assuming old archives contain it.
- Fixed panel.Client Node/Machine REST contract passed against the local Luma API.

No Linux systemd/OpenRC startup, real proxy data-plane E2E, 128 MiB ARM64 run or production upgrade has been validated. Automatic upgrade is blocked because process /healthz does not establish logical-node readiness. Optional audit remains disabled by default.
