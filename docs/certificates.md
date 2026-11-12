# Native TLS certificate lifecycle

The Luma panel sends native `cert_config` with `dns`, `http`, `file` or `content` mode through the existing authenticated machine configuration channel. The node is a native binary; no Docker, shell ACME client, global environment mutation or arbitrary provider script is required.

The DNS provider registry remains pinned to the upstream source lock. Luma's catalogue is generated from that registry. DNS-01 needs usable domain credentials, HTTP-01 needs reachable public TCP 80. REALITY does not use an ordinary TLS certificate. No automatic self-signed fallback is added.

`certificate-lifecycle-v1` adds explicit pending/ready/failed states, a gate preventing ordinary TLS listeners without valid certificate material covering the concrete SNI, retry after a failed configuration without committing its hash, cancellation and stopping of per-instance CertMagic caches, generation-safe renewal callbacks, and fixed error messages. Repeated unchanged configuration and restart reuse CertMagic storage. Each node retains its own existing configuration/certificate directory.

Diagnostics return only fixed certificate state classifications. Provider response bodies, DNS environment values and PEM private keys never enter diagnostic results. The certificate manager and issuer use non-output library loggers so provider errors containing credentials are not exposed; high-level service errors remain fixed.

`tests/certificate_lifecycle_test.go` uses an in-process issuer signing the real CertMagic CSR, with real temporary FileStorage. It verifies first issue, unchanged sync, restart cache reuse, renewal material publication, error recovery, generation cancellation and multiple independent node managers. `tests/certificate_service_test.go` verifies that failed TLS application cannot start a plain listener or suppress subsequent retries. No public CA orders or real DNS credentials are used by these tests. DNS-provider/ACME production network permissions remain deployment acceptance checks.
