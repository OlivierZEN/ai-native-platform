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
AI_NATIVE_KEYCLOAK_CLIENT_IDS=semattice-cli,storefront-web,admin-web
AI_NATIVE_KEYCLOAK_SERVICE_BINDINGS=commerce-service=orgxxxxxxxxxxxxxxxxx@11111111-1111-4111-8111-111111111111
AI_NATIVE_OACT_ALLOWED_SCOPES=system.capability.read,tenant.status.read,tenant.lifecycle.write,tenant.entitlement.write,tenant.decommission,metadata.version.write,metadata.definition.write,metadata.publish,metadata.read,metadata.changeset.write,metadata.changeset.read,metadata.changeset.approve,metadata.changeset.publish,metadata.changeset.execute,metadata.changeset.purge,metadata.changeset.rollback,usage.read,usage.platform.read,runtime.record.create,runtime.record.read,runtime.record.update,runtime.record.delete,authorization.manage,record.share.manage,organization.manage,authorization.read
AI_NATIVE_OACT_TTL=10m
AI_NATIVE_IDENTITY_ISSUER=https://semattice.agentcici.com
AI_NATIVE_IDENTITY_AUDIENCE=semattice-api
AI_NATIVE_IDENTITY_ALGORITHM=HS256
AI_NATIVE_IDENTITY_HMAC_KEY=<independent 32+ character Semattice OACT key>
AI_NATIVE_CONSOLE_SESSION_HMAC_KEY=<independent 32+ character browser-session HMAC key>
AI_NATIVE_CONSOLE_OIDC_CLIENT_ID=semattice-web
AI_NATIVE_CONSOLE_OIDC_CLIENT_SECRET_FILE=/etc/semattice/secrets/semattice-web-client-secret
AI_NATIVE_CONSOLE_OIDC_REDIRECT_URI=https://semattice.agentcici.com/auth/oidc/callback
```

Human access tokens from `semattice-cli`, `storefront-web`, and `admin-web` must include audience `semattice-api` and exactly one Keycloak Organization Membership claim. Semattice maps the alias to an existing active `tenant_registry.company_id` and signs a short-lived HUMAN OACT. A controlled service client may be bound server-side as `client_id=company_id@owner_principal_id`; it receives a SERVICE OACT and cannot select tenant or owner through the request body. A missing, partial, or invalid access-context configuration makes `serve` fail closed.

`AI_NATIVE_CONSOLE_SESSION_HMAC_KEY` is separate from the identity signing key. It signs only the short-lived, HttpOnly Semattice management-console cookie after an OACT has been verified; never reuse an existing HMAC key for it.

The `semattice-web` Keycloak client is confidential and uses Authorization Code flow. Store its Client Secret only in `/etc/semattice/secrets/semattice-web-client-secret`, owned by `root:semattice` with mode `0640`; never put the value in `semattice.env`, Git, browser code, or a URL. The exact Keycloak redirect URI is `https://semattice.agentcici.com/auth/oidc/callback`. The web access token must include audience `semattice-api` and exactly one Organization membership claim. The callback exchanges the code with HTTP Basic client authentication, validates state, nonce, PKCE and both tokens, resolves an active Semattice tenant, then creates a short-lived signed `Secure; HttpOnly; SameSite=Lax` session cookie containing no Keycloak token.

The current executable source remains `cmd/ai-native-platform`; release packaging names the Linux binary `semattice` according to ADR-012.

The embedded migrations create the `ai_native_control` and `ai_native_runtime` roles. On a fresh database, the migrator therefore needs `CREATEROLE` only for the migration window. Revoke it immediately after a successful migration, then enable `LOGIN` on the two generated roles with independent passwords. Both roles must remain non-owner, non-superuser, non-CREATEROLE, and non-BYPASSRLS. Do not place the migrator URL in `semattice.env`.

The verified release layout is:

- `/opt/semattice/releases/<release>/semattice`: immutable release binary
- `/opt/semattice/current`: active-release symlink
- `/usr/local/bin/semattice`: CLI/MCP entry point
- `/var/www/semattice`: usage guide and downloadable binary
- `/etc/semattice/semattice.env`: root/service-group-only runtime configuration
- `/etc/semattice/tls`: root-only TLS private key and public certificate
