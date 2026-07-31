# Semattice single-node deployment assets

These assets support FEAT-022:

- `www/`: static usage guide served at `https://semattice.agentcici.com/`
- `nginx.conf`: TLS terminator, documentation server, download endpoint, and `/v1/` reverse proxy
- `semattice.service`: hardened systemd unit for the Go capability API
- `postgresql-16.service`: systemd unit for the exact PostgreSQL 16.13 source build used on Alibaba Cloud Linux 4

Secrets are intentionally absent. The deployment creates `/etc/semattice/semattice.env` on the target host with mode `0640`, owned by `root:semattice`. TLS material lives under `/etc/semattice/tls` and is never copied into the repository.

Keycloak Organization exchange is mandatory for `semattice serve`. The protected environment file must set all of the following (values are never stored in this repository):

```text
AI_NATIVE_KEYCLOAK_ISSUER=https://sso.agentcici.com/realms/agentcici
AI_NATIVE_KEYCLOAK_AUDIENCE=semattice-api
AI_NATIVE_KEYCLOAK_JWKS_URL=https://sso.agentcici.com/realms/agentcici/protocol/openid-connect/certs
AI_NATIVE_KEYCLOAK_CLIENT_ID=semattice-cli
AI_NATIVE_OACT_ALLOWED_SCOPES=system.capability.read,tenant.read,metadata.read,record.read
AI_NATIVE_OACT_TTL=10m
AI_NATIVE_IDENTITY_ISSUER=https://semattice.agentcici.com
AI_NATIVE_IDENTITY_AUDIENCE=semattice-api
AI_NATIVE_IDENTITY_ALGORITHM=HS256
AI_NATIVE_IDENTITY_HMAC_KEY=<independent 32+ character Semattice OACT key>
AI_NATIVE_CONSOLE_SESSION_HMAC_KEY=<independent 32+ character browser-session HMAC key>
```

The `semattice-cli` access token must include audience `semattice-api` and the Keycloak Organization Membership claim. Semattice accepts exactly one Organization alias, maps it to an existing active `tenant_registry.company_id`, and signs a short-lived OACT with the identity key. A missing, partial, or invalid access-context configuration makes `serve` fail closed.

`AI_NATIVE_CONSOLE_SESSION_HMAC_KEY` is separate from the identity signing key. It signs only the short-lived, HttpOnly Semattice management-console cookie after an OACT has been verified; never reuse an existing HMAC key for it.

The current executable source remains `cmd/ai-native-platform`; release packaging names the Linux binary `semattice` according to ADR-012.

The embedded migrations create the `ai_native_control` and `ai_native_runtime` roles. On a fresh database, the migrator therefore needs `CREATEROLE` only for the migration window. Revoke it immediately after a successful migration, then enable `LOGIN` on the two generated roles with independent passwords. Both roles must remain non-owner, non-superuser, non-CREATEROLE, and non-BYPASSRLS. Do not place the migrator URL in `semattice.env`.

The verified release layout is:

- `/opt/semattice/releases/<release>/semattice`: immutable release binary
- `/opt/semattice/current`: active-release symlink
- `/usr/local/bin/semattice`: CLI/MCP entry point
- `/var/www/semattice`: usage guide and downloadable binary
- `/etc/semattice/semattice.env`: root/service-group-only runtime configuration
- `/etc/semattice/tls`: root-only TLS private key and public certificate
