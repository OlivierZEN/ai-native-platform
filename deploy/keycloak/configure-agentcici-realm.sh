#!/usr/bin/env bash
set -euo pipefail

# Idempotently creates the production business realm and non-secret client
# registrations. Client secrets are intentionally not read or printed here.

keycloak_bin="/opt/keycloak/current/bin/kcadm.sh"
keycloak_config="/run/keycloak-kcadm.config"
export JAVA_HOME=/opt/java/current
export PATH="${JAVA_HOME}/bin:${PATH}"

if [[ "${EUID}" -ne 0 ]]; then
  printf '%s\n' 'Run this configurator as root.' >&2
  exit 1
fi
if [[ ! -x "${keycloak_bin}" || ! -f /etc/keycloak/bootstrap-admin.env ]]; then
  printf '%s\n' 'Keycloak installation or bootstrap-admin configuration is unavailable.' >&2
  exit 1
fi

# shellcheck disable=SC1091
source /etc/keycloak/bootstrap-admin.env
trap 'rm -f "${keycloak_config}"' EXIT

"${keycloak_bin}" config credentials \
  --server http://127.0.0.1:8180 \
  --realm master \
  --user "${KC_BOOTSTRAP_ADMIN_USERNAME}" \
  --password "${KC_BOOTSTRAP_ADMIN_PASSWORD}" \
  --config "${keycloak_config}" >/dev/null

kcadm() {
  "${keycloak_bin}" "$@" --config "${keycloak_config}"
}

if ! kcadm get realms/agentcici >/dev/null 2>&1; then
  kcadm create realms \
    -s realm=agentcici \
    -s displayName='AgentCiCi Unified Identity' \
    -s enabled=true \
    -s registrationAllowed=false \
    -s loginWithEmailAllowed=true \
    -s duplicateEmailsAllowed=false \
    -s accessTokenLifespan=300 \
    -s ssoSessionIdleTimeout=1800 \
    -s ssoSessionMaxLifespan=28800 >/dev/null
fi

client_exists() {
  kcadm get clients -r agentcici -q "clientId=$1" | grep -q '"clientId"'
}

create_client() {
  local client_id="$1"
  local payload="$2"
  if ! client_exists "${client_id}"; then
    printf '%s' "${payload}" | kcadm create clients -r agentcici -f - >/dev/null
  fi
}

create_client agentcici-bff '{
  "clientId":"agentcici-bff",
  "name":"AgentCiCi BFF",
  "enabled":true,
  "protocol":"openid-connect",
  "publicClient":false,
  "standardFlowEnabled":true,
  "directAccessGrantsEnabled":false,
  "implicitFlowEnabled":false,
  "serviceAccountsEnabled":false,
  "redirectUris":[],
  "webOrigins":[]
}'

# Keep this update outside create_client so repeat runs reconcile the redirect URI
# rather than leaving the original intentionally empty bootstrap registration.
agentcici_bff_id="$(kcadm get clients -r agentcici -q 'clientId=agentcici-bff' --fields id | sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' | head -n 1)"
if [[ -z "${agentcici_bff_id}" ]]; then
  printf '%s\n' 'Unable to resolve agentcici-bff client.' >&2
  exit 1
fi
kcadm update "clients/${agentcici_bff_id}" -r agentcici \
  -s 'redirectUris=["https://x.agentcici.com/auth/oidc/callback"]' \
  -s 'webOrigins=["https://x.agentcici.com"]' \
  -s 'standardFlowEnabled=true' \
  -s 'directAccessGrantsEnabled=false' \
  -s 'implicitFlowEnabled=false' >/dev/null

# The theme changes only the Keycloak browser surface. It never forwards
# passwords to AgentCiCi or changes the authorization-code + PKCE flow.
# A fresh IdP can still be configured before the theme package is deployed.
if [[ -d /opt/keycloak/current/themes/agentcici ]]; then
  kcadm update realms/agentcici \
    -s loginTheme=agentcici \
    -s internationalizationEnabled=true \
    -s defaultLocale=zh-CN \
    -s 'supportedLocales=["zh-CN"]' \
    -s registrationEmailAsUsername=false \
    -s loginWithEmailAllowed=true \
    -s duplicateEmailsAllowed=false \
    -s editUsernameAllowed=false >/dev/null
fi

create_client semattice-cli '{
  "clientId":"semattice-cli",
  "name":"CloudCC Semattice CLI",
  "enabled":true,
  "protocol":"openid-connect",
  "publicClient":true,
  "standardFlowEnabled":true,
  "directAccessGrantsEnabled":false,
  "implicitFlowEnabled":false,
  "serviceAccountsEnabled":false,
  "redirectUris":["http://127.0.0.1"],
  "webOrigins":[],
  "attributes":{"pkce.code.challenge.method":"S256"}
}'
semattice_cli_id="$(kcadm get clients -r agentcici -q 'clientId=semattice-cli' --fields id | sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' | head -n 1)"
if [[ -z "${semattice_cli_id}" ]]; then
  printf '%s\n' 'Unable to resolve semattice-cli client.' >&2
  exit 1
fi
kcadm update "clients/${semattice_cli_id}" -r agentcici \
  -s 'publicClient=true' \
  -s 'standardFlowEnabled=true' \
  -s 'directAccessGrantsEnabled=false' \
  -s 'implicitFlowEnabled=false' \
  -s 'serviceAccountsEnabled=false' \
  -s 'redirectUris=["http://127.0.0.1"]' \
  -s 'webOrigins=[]' \
  -s 'attributes={"pkce.code.challenge.method":"S256"}' >/dev/null

create_client semattice-api '{
  "clientId":"semattice-api",
  "name":"CloudCC Semattice Resource Server",
  "enabled":true,
  "protocol":"openid-connect",
  "bearerOnly":true,
  "publicClient":false,
  "standardFlowEnabled":false,
  "directAccessGrantsEnabled":false,
  "implicitFlowEnabled":false,
  "serviceAccountsEnabled":false
}'

ensure_semattice_audience() {
  local client_id="$1"
  local internal_id
  internal_id="$(kcadm get clients -r agentcici -q "clientId=${client_id}" --fields id | sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' | head -n 1)"
  if [[ -z "${internal_id}" ]]; then
    printf 'Unable to resolve %s client.\n' "${client_id}" >&2
    exit 1
  fi
  if kcadm get "clients/${internal_id}/protocol-mappers/models" -r agentcici |
    grep -Eq '"name"[[:space:]]*:[[:space:]]*"semattice-api-audience"'; then
    return
  fi
  printf '%s' '{
    "name":"semattice-api-audience",
    "protocol":"openid-connect",
    "protocolMapper":"oidc-audience-mapper",
    "consentRequired":false,
    "config":{
      "included.client.audience":"semattice-api",
      "access.token.claim":"true",
      "id.token.claim":"false",
      "lightweight.claim":"false"
    }
  }' | kcadm create "clients/${internal_id}/protocol-mappers/models" -r agentcici -f - >/dev/null
}

# Semattice verifies Keycloak access tokens before issuing its own short-lived
# OACT. Human web clients and the controlled commerce service all need the
# resource-server audience; unrelated clients are left unchanged.
ensure_semattice_audience semattice-cli
for semattice_caller in storefront-web admin-web commerce-service; do
  if client_exists "${semattice_caller}"; then
    ensure_semattice_audience "${semattice_caller}"
  fi
done

create_client official-access-context '{
  "clientId":"official-access-context",
  "name":"Official Access Context Service",
  "enabled":true,
  "protocol":"openid-connect",
  "publicClient":false,
  "standardFlowEnabled":false,
  "directAccessGrantsEnabled":false,
  "implicitFlowEnabled":false,
  "serviceAccountsEnabled":true
}'

create_client followup-worker '{
  "clientId":"followup-worker",
  "name":"FollowUp Worker",
  "enabled":true,
  "protocol":"openid-connect",
  "publicClient":false,
  "standardFlowEnabled":false,
  "directAccessGrantsEnabled":false,
  "implicitFlowEnabled":false,
  "serviceAccountsEnabled":true
}'

printf '%s\n' 'realm-and-client-registrations-ready'
