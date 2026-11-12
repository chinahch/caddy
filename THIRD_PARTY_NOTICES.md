# Third-party sources

- This is a customization/build repository for cedar2025/Xboard-Node, whose README declares MPL-2.0. It is not the caddyserver/caddy Web Server.
- Upstream revision and patch lineage are recorded in source.lock.json and docs/provenance.md. Source changes must keep applicable upstream notices.
- sing-box, Xray, certmagic and other transitive Go dependencies are resolved from the fixed upstream go.mod/go.sum; they retain their respective licenses. The MPL license for this repository does not replace their licenses.
- Native binary preview releases ship the exact build's dependency inventory and full corresponding module/Go/customization sources, including their original license/notice files. `scripts/release_inventory.py` uses the same Linux target and build tags as the binary and refuses a module without a root license/notice. Source archives are separate release assets; no dependency is relabeled under this repository's license. These are preview builds, not production acceptance claims.
- The connector fetcher downloads official [mtg v2.3.0 (MIT)](https://github.com/9seconds/mtg/blob/v2.3.0/LICENSE) and [cloudflared 2026.10.7 (Apache-2.0)](https://github.com/cloudflare/cloudflared/blob/2026.10.7/LICENSE) using pinned SHA256 assets. They are separate executables, not relicensed by this repository. Preserve their notices when redistributing them.
