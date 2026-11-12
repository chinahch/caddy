# Protocol and landing validation (2026-11-12)

- Real PostgreSQL tests pass for encrypted exits, redacted lists, admin/user isolation, capability gates, disabled exits, per-user MTG discovery/quota and report replay.
- 23 Xray and 6 sing-box generated server/client pairs complete actual isolated loopback SOCKS→protocol→server→TCP-echo traffic, including six ENC combinations, AnyTLS Reality, SS2022, Hysteria2 and XHTTP H3. Certificates are local test certificates; Xray uses certificate SHA256 pins.
- Browser QA passes for multiline IPv6 landing import/dedup/edit/save/default routing and ML-KEM-768/random protocol generation and node creation. Frontend typecheck, production build and existing native tests pass.
- Worker process/lease tests pass. Official mtg v2.3.0 from the pinned Go module accepts generated TOML and serves loopback metrics. Immutable queue and retry tests preserve report identities; Telegram-only counters exclude domain fronting.
- Baseline Linux CI passed for node source/build and panel Compose/database backup→empty-instance restore. Feature Linux CI adds native data-plane, connector/vendor smoke and encrypted credential recovery; check the corresponding commit's Actions results.

Still unverified: actual CF account/hostname attachment, external Telegram client sessions, Linux systemd/OpenRC host operation, ARM64 runtime/128 MiB capacity and production load. Native node traffic still uses the documented best-effort legacy sender. Automatic node upgrade remains disabled until logical-node readiness can be established.

## Final review regression pass

Independent read-only review identified six Important defects. Each was reproduced before its fix, then verified with the full affected suites: arbitrary custom-outbound secret projection/encryption; native Xray private-key error/log leakage on Start/Reload; MTG database/route failure refusing direct leases; sustained 64-node reporting and independent status/report budgets; per-node rejected-report preservation/isolation; concurrent native/Telegram subscriptions with a one-connection PostgreSQL pool. No reviewer Minor findings were deferred.

The connector regression also stalls a real status HTTP request while durable reports continue delivering, and verifies that an identity with rejected reports does not execute a process. Mobile 390×844 landing layout was visually rechecked after stacking the heading/toolbar.

## Historical bootstrap checks


Toolchain: Go 1.26.10. Go 1.26.3 hit an internal compiler panic in an AWS dependency, so results are not attributed to that compiler.

- Pinned source preparation and normalized package tests passed.
- Real CLI YAML round-trip passed for shared/per-instance `ip_family` and per-kernel custom config paths.
- Bash syntax, checksum corruption rejection, unknown service collision guard, existing Machine configuration/credentials preservation and blocked unsafe upgrade regressions passed in Windows Git Bash.
- Fixed-source config, tracker, sing-box, Xray and controlplane tests passed; Machine package compiled and contains no test files in this snapshot.
- Linux amd64 and arm64 cross compilation passed before the CLI serialization fix. The fix's CLI package test passed; final package rebuild evidence is reported with the exact final build artifacts rather than assuming old archives contain it.
- Fixed panel.Client Node/Machine REST contract passed against the local Luma API.

At bootstrap, Linux service startup, data-plane E2E, ARM64 runtime and production upgrade had not been validated; current native loopback results are listed above. Automatic upgrade is blocked because process /healthz does not establish logical-node readiness. Optional audit remains disabled by default.
