# Source audit

Baseline: chinahch/php-fpm-core c8058a2f14060d8d2d12adaaef218ff558a1df98. Upstream: cedar2025/Xboard-Node 0a29338e1f102a462363ce3527417029f89bab28. Dual-IP lineage: baseline history 2a5ae50eed9bb400995601293a4ebd484920b0fd.

The baseline is a build/patch/installer repository, not a complete Go tree. Its current main workflow contains a duplicated source_ref suffix; the published dual-IP workflow relies on a historical patch not present in main. Root-level build-caddy.yml is not the same file as the active .github/workflows/build-caddy.yml. The migration uses the fixed upstream commit and restores the historical native/strict IP transform explicitly.

The audit build and ordinary XHTTP build historically applied different customization sets. New preparation combines the known patches in a fixed order and makes audit opt-in. Paths retain caddy/caddyctl and /etc/caddy for compatibility; this is not Caddy Web Server. Unknown existing service/path ownership is rejected before installation.

The kernel supplies cumulative counters; tracker.Process computes deltas and FlushTraffic drains pending increments. Failed report calls restore to memory. There is no durable stable batch identity in the old report. A lost response after a server commit may produce a duplicate retry; process failure may lose pending usage. Luma labels this legacy, distinct from its newer idempotent receiver.

Independent review found CLI YAML serialization omitted added IPFamily and per-core custom config fields; the preparation patch and real CLI round-trip test now preserve them. Process /healthz does not prove logical kernels ready, so automatic upgrade is disabled until a trustworthy readiness contract exists. Manual upgrades must preserve old binaries/configuration and verify real operation.

DOMAIN-AUDIT-README records an unresolved earlier ARM64 Alpine 128 MiB crash. Compilation, version output and unit tests do not close that runtime risk. Refer to compatibility-matrix.md and exact CI results.
