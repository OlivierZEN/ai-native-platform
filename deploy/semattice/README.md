# Semattice single-node deployment assets

These assets support FEAT-022:

- `www/`: static usage guide served at `https://semattice.agentcici.com/`
- `nginx.conf`: TLS terminator, documentation server, download endpoint, and `/v1/` reverse proxy
- `semattice.service`: hardened systemd unit for the Go capability API
- `postgresql-16.service`: systemd unit for the exact PostgreSQL 16.13 source build used on Alibaba Cloud Linux 4

Secrets are intentionally absent. The deployment creates `/etc/semattice/semattice.env` on the target host with mode `0640`, owned by `root:semattice`. TLS material lives under `/etc/semattice/tls` and is never copied into the repository.

Controlled provisioning is mandatory for `semattice serve`. The same protected environment file must set all of the following (values are never stored in this repository):

```text
AI_NATIVE_AGENTCICI_BASE_URL=https://onechat.agentcici.com
AI_NATIVE_AGENTCICI_HMAC_KEY=<Semattice-to-AgentCiCi HMAC key>
AI_NATIVE_PROVISIONING_CALLER_KEYS=agentcici=<AgentCiCi-to-Semattice HMAC key>[;trusted-system=<key>]
```

The first key must equal AgentCiCi's `APP_NATIVE_AGENTCICI_INTERNAL_HMAC_KEY`; the `agentcici` caller key must equal AgentCiCi's `APP_SEMATTICE_INTERNAL_HMAC_KEY`. Use independently generated 32+ character secrets. A missing, partial, or invalid configuration makes `serve` fail closed; there is no public/JWT/CLI fallback for company provisioning.

The current executable source remains `cmd/ai-native-platform`; release packaging names the Linux binary `semattice` according to ADR-012.

The embedded migrations create the `ai_native_control` and `ai_native_runtime` roles. On a fresh database, the migrator therefore needs `CREATEROLE` only for the migration window. Revoke it immediately after a successful migration, then enable `LOGIN` on the two generated roles with independent passwords. Both roles must remain non-owner, non-superuser, non-CREATEROLE, and non-BYPASSRLS. Do not place the migrator URL in `semattice.env`.

The verified release layout is:

- `/opt/semattice/releases/<release>/semattice`: immutable release binary
- `/opt/semattice/current`: active-release symlink
- `/usr/local/bin/semattice`: CLI/MCP entry point
- `/var/www/semattice`: usage guide and downloadable binary
- `/etc/semattice/semattice.env`: root/service-group-only runtime configuration
- `/etc/semattice/tls`: root-only TLS private key and public certificate
