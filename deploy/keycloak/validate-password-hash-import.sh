#!/usr/bin/env bash
set -euo pipefail

# Verifies, using a random disposable account, that AgentCiCi's legacy
# PBKDF2WithHmacSHA256 password representation can be imported by Keycloak.
# It never reads AgentCiCi data and deletes the test account before exit.

export JAVA_HOME=/opt/java/current
export PATH="${JAVA_HOME}/bin:${PATH}"
keycloak_bin=/opt/keycloak/current/bin/kcadm.sh
keycloak_config=/run/keycloak-password-import-kcadm.config
payload=/run/keycloak-password-import-user.json
test_user="credential-import-smoke-$(date +%s)"
test_client="credential-import-validator-$(date +%s)"
test_password="$(openssl rand -base64 30 | tr -d '\n')"
legacy_salt="legacy-agentcici-salt-$(openssl rand -hex 12)"
test_user_id=""
test_client_id=""

if [[ "${EUID}" -ne 0 ]]; then
  printf '%s\n' 'Run this validator as root.' >&2
  exit 1
fi

cleanup() {
  if [[ -n "${test_user_id}" ]]; then
    "${keycloak_bin}" delete "users/${test_user_id}" -r agentcici --config "${keycloak_config}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${test_client_id}" ]]; then
    "${keycloak_bin}" delete "clients/${test_client_id}" -r agentcici --config "${keycloak_config}" >/dev/null 2>&1 || true
  fi
  rm -f "${keycloak_config}" "${payload}"
}
trap cleanup EXIT

# shellcheck disable=SC1091
source /etc/keycloak/bootstrap-admin.env
"${keycloak_bin}" config credentials \
  --server http://127.0.0.1:8180 \
  --realm master \
  --user "${KC_BOOTSTRAP_ADMIN_USERNAME}" \
  --password "${KC_BOOTSTRAP_ADMIN_PASSWORD}" \
  --config "${keycloak_config}" >/dev/null

"${keycloak_bin}" create clients -r agentcici \
  -s "clientId=${test_client}" \
  -s enabled=true \
  -s publicClient=true \
  -s standardFlowEnabled=false \
  -s directAccessGrantsEnabled=true \
  -s implicitFlowEnabled=false \
  --config "${keycloak_config}" >/dev/null
test_client_id="$("${keycloak_bin}" get clients -r agentcici -q "clientId=${test_client}" --fields id --config "${keycloak_config}" | sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' | head -n 1)"
if [[ -z "${test_client_id}" ]]; then
  printf '%s\n' 'Could not locate disposable Keycloak client.' >&2
  exit 1
fi

hash_hex="$(openssl kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt "pass:${test_password}" -kdfopt "salt:${legacy_salt}" -kdfopt iter:120000 PBKDF2 | tr -d ':\n')"
hash_b64="$(printf '%s' "${hash_hex}" | xxd -r -p | base64 | tr -d '\n')"
# AgentCiCi feeds UTF-8 bytes of its stored salt string to PBKDF2. Keycloak
# stores a base64 representation of those exact bytes, not of a decoded salt.
salt_b64="$(printf '%s' "${legacy_salt}" | base64 | tr -d '\n')"

cat > "${payload}" <<EOF
{
  "username":"${test_user}",
  "enabled":true,
  "email":"${test_user}@example.test",
  "emailVerified":true,
  "firstName":"Credential",
  "lastName":"Smoke",
  "requiredActions":[],
  "credentials":[{
    "type":"password",
    "temporary":false,
    "secretData":"{\\"value\\":\\"${hash_b64}\\",\\"salt\\":\\"${salt_b64}\\",\\"additionalParameters\\":{}}",
    "credentialData":"{\\"hashIterations\\":120000,\\"algorithm\\":\\"pbkdf2-sha256\\",\\"additionalParameters\\":{}}"
  }]
}
EOF

"${keycloak_bin}" create users -r agentcici -f "${payload}" --config "${keycloak_config}" >/dev/null
test_user_id="$("${keycloak_bin}" get users -r agentcici -q "username=${test_user}" --fields id --config "${keycloak_config}" | sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' | head -n 1)"
if [[ -z "${test_user_id}" ]]; then
  printf '%s\n' 'Could not locate disposable Keycloak user.' >&2
  exit 1
fi

status_code="$(curl -sS -o /dev/null -w '%{http_code}' \
  --data-urlencode "client_id=${test_client}" \
  --data-urlencode 'grant_type=password' \
  --data-urlencode "username=${test_user}" \
  --data-urlencode "password=${test_password}" \
  https://sso.agentcici.com/realms/agentcici/protocol/openid-connect/token)"
if [[ "${status_code}" != '200' ]]; then
  printf '%s\n' 'Imported PBKDF2 credential was rejected.' >&2
  exit 1
fi

printf '%s\n' 'legacy-pbkdf2-import-and-login-verified'
