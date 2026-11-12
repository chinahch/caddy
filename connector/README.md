# Luma connector worker

`luma-connect` supervises pinned cloudflared and mtg processes on Linux. It authenticates as one Luma Machine or one Node, polls desired state, writes private credential files, starts only these two allowlisted binary names, checks loopback health, and stops processes after their panel lease expires. Native Caddy remains a separate service.

MTG uses one assigned user/secret/port per instance. Only Telegram byte counters are sampled; domain-fronting traffic is excluded. Samples enter an fsynced immutable disk queue before submission to Luma's deduplicating receiver. Unacknowledged reports survive restarts. Unsampled bytes immediately before a process crash can still be lost. Do not run two workers for the same Machine; the state-directory lock only prevents duplicates on one filesystem.

CF uses a remotely managed named tunnel. Configure the public hostname and its local HTTP/WS origin in Cloudflare. `/ready` reports tunnel connectivity; an MTG `ready` status means its local metrics endpoint is available, not that an external Telegram client has completed a session.

## Install from source

Requires Linux, Go 1.26.10, Python 3.12+, and a Luma version with connector-v1 endpoints. Check out the intended Caddy commit. For ARM64, replace `amd64` below with `arm64`.

```sh
python3 scripts/fetch_connectors.py --arch amd64 --out dist/connectors
CGO_ENABLED=0 go -C connector build -trimpath -o ../dist/luma-connect ./cmd/luma-connect
# Official asset checksums/versions are pinned in connector/versions.lock.json.
sudo install -d -m 0755 /usr/local/lib/caddy-connect
sudo install -m 0755 dist/connectors/mtg dist/connectors/cloudflared /usr/local/lib/caddy-connect/
sudo install -m 0755 dist/luma-connect /usr/local/bin/luma-connect
sudo useradd --system --home-dir /var/lib/caddy-connect --shell /usr/sbin/nologin caddy-connect
sudo install -d -m 0750 -o root -g caddy-connect /etc/caddy-connect
sudo install -m 0640 -o root -g caddy-connect connector/connect.example.json /etc/caddy-connect/connect.json
sudo install -m 0644 connector/caddy-connect.service /etc/systemd/system/caddy-connect.service
```

Edit `connect.json`: use your HTTPS panel origin and actual Machine ID. For a standalone Node, remove `machine_id` and set `node_id` instead. Create `/etc/caddy-connect/worker.env`, owned by root:caddy-connect, mode 0640, containing `LUMA_NODE_TOKEN=YOUR_EXISTING_MACHINE_OR_NODE_TOKEN`. There is no default credential. Do not copy tokens into shell arguments or Git.

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now caddy-connect.service
sudo systemctl status caddy-connect.service
```

Give every connector a unique local metrics port. Keep metrics on loopback. CF also requires the native Caddy worker with the same Machine to serve its loopback WS origin. MTG listens on its configured TCP port; permit that port in the host/provider firewall. The worker caps one desired response at 64 connectors; per-user MTG uses additional processes and ports.

The lease is normally 90 seconds, checked during each bounded polling cycle (15-second tick; separate 5-second desired-state, 5-second sampling and 3-second status budgets). A panel outage stops connectors when the lease is no longer valid. Invalid desired state does not renew the lease. Reports run independently with a 10-second budget and at most eight concurrent requests; slow status calls cannot consume reporting time. Child output is discarded to prevent credential-bearing vendor errors entering panel logs; inspect vendor connectivity separately using private configs only when needed.

## Reports requiring action

Transient failures remain in `queue/` and retry fairly alongside other nodes. HTTP 400/403/404/409/422 responses preserve the original batch under `queue/rejected/<node-id>/`, with a separate `.reason` file. Only that node is paused and marked `queue_blocked`; unrelated users continue delivering reports. Malformed records whose node cannot be identified are preserved under `rejected/0` and produce an operator warning.

Stop the worker before recovery. Inspect the rejection status and reconcile the panel's node identity, enablement and accounting record first. For a resolved rejection, move the original `.json` batch back to `queue/`, preserving its contents/name, then restart. Keep the `.reason` file for diagnosis. Do not rewrite batch IDs or discard unacknowledged usage to clear an error; a conflicting report requires accounting investigation.

## Change servers

Stop this worker, migrate its entire private queue directory (including rejected reports) plus private worker configuration securely, and start exactly one replacement worker after restoring the panel database and master key. Preserve node IDs and Machine identity. The main Caddy node still uses its separately documented legacy traffic sender.

## Tests and limits

`go -C connector test ./...` covers real process start/lease termination, credential-free argv, port validation, Telegram-only exact counters, immutable disk queues and retry authentication. Linux CI also fetches checksum-verified official binaries and exercises their CLI/MTG loopback metrics. Actual CF account attachment and end-user Telegram/ISP connectivity require external account/network validation. ARM64 compilation does not establish runtime capacity on a 128 MiB machine.

Worker original code follows the repository MPL-2.0 license. Official mtg is MIT and cloudflared is Apache-2.0; download their corresponding upstream license notices when redistributing their binaries. This repository does not publish a combined binary release without that inventory.
