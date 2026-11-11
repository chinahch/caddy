# Open-source node and panel design

## Intent
Two independently maintainable public repositories: chinahch/caddy for a traceable customized XBoard-compatible node, and chinahch/luma for the self-hosted Go/Vue panel. Preserve the original php-fpm-core repository and existing production installations.

## Architecture
Node: pinned upstream source plus auditable XHTTP, strict IP-family and CLI compatibility patches. Optional audit stays disabled. Build Linux amd64 and arm64 separately; distinguish compilation from actual low-memory Linux runtime validation.
Panel: Go/Gin API, PostgreSQL durable raw usage/ledger, Redis reconstructible snapshots, Vue3/TypeScript/Naive UI. Same-origin Nginx, Compose source builds, private environment configuration and explicit database migrations.

## Choices
Separate repositories allow node and panel upgrades independently; a monorepo couples these release processes. A full fork of the existing PHP XBoard panel would bring ready-made features but does not match the chosen Go/Vue implementation. Luma original code uses MIT; the node customization retains MPL-2.0 and applicable third-party notices.

## Accounting and access
Raw bytes, multiplier-adjusted billing and host interface counters are separate. UTC persistence with explicit query timezone. Reliable receipt needs a stable node/report ID and one transaction for raw records, ledger and account counters; legacy transport lacks a durable ID and must remain labeled best-effort. Users see their own records; node tokens are scoped to a node or Machine and its owned nodes.

## Portable operations
PostgreSQL backups plus private configuration and exact application version are required to change servers. Redis is reconstructible, not a unique accounting queue. Restore only into a new empty database, verify accounts/quotas/ledger, then switch the domain. No production restart, overwrite or release is implied by source publication.

## Additional requested capabilities
The next development branch targets the 15 protocol/transport combinations in protocol-roadmap.md, including Reality/ENC, Hysteria2, SS2022, AnyTLS, CF Tunnel and MTProxy. Landing means configurable outbound proxies/routes, with pasted single/multiline node-link import and editable preview; it does not mean SSH provisioning. Private proxy credentials must not enter Git, logs or unredacted list responses. Capability verification must include generated core configs, subscriptions and real connection tests where an environment is available.

## Verification
Publish actual command results and label unexecuted Linux service, ARM64 128 MiB, Docker and recovery checks. A source preview is not a production readiness claim. Unknown service/path ownership must stop installation; a process-only health endpoint cannot justify an automatic upgrade success claim.
