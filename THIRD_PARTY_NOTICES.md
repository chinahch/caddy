# Third-party sources

- This is a customization/build repository for cedar2025/Xboard-Node, whose README declares MPL-2.0. It is not the caddyserver/caddy Web Server.
- Upstream revision and patch lineage are recorded in source.lock.json and docs/provenance.md. Source changes must keep applicable upstream notices.
- sing-box, Xray, certmagic and other transitive Go dependencies are resolved from the fixed upstream go.mod/go.sum; they retain their respective licenses. The MPL license for this repository does not replace their licenses.
- Before distributing binary releases, generate and ship the complete dependency license/notices inventory for that exact build. The initial repository intentionally provides no automatically published production binary release.
