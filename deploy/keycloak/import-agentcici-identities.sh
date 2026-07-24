#!/usr/bin/env bash
set -euo pipefail

# Imports the existing AgentCiCi global accounts into the agentcici Keycloak
# realm without exposing plaintext passwords.  Input is a root-readable TSV:
# account_id, username, PBKDF2 hash (base64), base64-encoded UTF-8 legacy
# salt, iterations, algorithm.  It writes only account_id -> Keycloak subject
# mappings.
#
# The importer is deliberately idempotent: a retry reuses an already-created
# Keycloak user with the same username, but it never overwrites credentials.

if [[ "${EUID}" -ne 0 ]]; then
  printf '%s\n' 'Run this importer as root.' >&2
  exit 1
fi
if [[ "$#" -ne 2 ]]; then
  printf '%s\n' "Usage: $0 INPUT_TSV OUTPUT_MAPPING_TSV" >&2
  exit 1
fi

input="$1"
output="$2"
keycloak_bin="/opt/keycloak/current/bin/kcadm.sh"
keycloak_config="/run/keycloak-agentcici-import-kcadm.config"
payload_dir="$(mktemp -d /run/keycloak-agentcici-import.XXXXXX)"

if [[ ! -r "${input}" || ! -x "${keycloak_bin}" || ! -r /etc/keycloak/bootstrap-admin.env ]]; then
  printf '%s\n' 'Input or Keycloak bootstrap prerequisites are unavailable.' >&2
  exit 1
fi

export JAVA_HOME=/opt/java/current
export PATH="${JAVA_HOME}/bin:${PATH}"
umask 077

cleanup() {
  rm -f "${keycloak_config}" "${output}.partial"
  rm -rf "${payload_dir}"
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

kcadm() {
  "${keycloak_bin}" "$@" --config "${keycloak_config}"
}

if ! kcadm get realms/agentcici >/dev/null 2>&1; then
  printf '%s\n' 'Keycloak realm agentcici is unavailable.' >&2
  exit 1
fi

: > "${output}.partial"
count=0
while IFS=$'\t' read -r account_id username password_hash salt_b64 iterations algorithm; do
  [[ -n "${account_id}" ]] || continue
  if [[ ! "${account_id}" =~ ^[A-Za-z0-9_-]{1,64}$ || -z "${username}" || -z "${password_hash}" || -z "${salt_b64}" || ! "${iterations}" =~ ^[1-9][0-9]*$ || "${algorithm}" != "PBKDF2WithHmacSHA256" ]] || ! printf '%s' "${salt_b64}" | base64 -d >/dev/null 2>&1; then
    printf '%s\n' 'Invalid protected identity-import row.' >&2
    exit 1
  fi

  user_json="${payload_dir}/${count}.json"
  existing="$(kcadm get users -r agentcici -q "username=${username}" --fields id,username | jq -r --arg username "${username}" '.[] | select(.username == $username) | .id' | head -n 1)"
  if [[ -z "${existing}" ]]; then
    jq -n \
      --arg username "${username}" \
      --arg email "agentcici-${account_id}@identity.invalid" \
      --arg hash "${password_hash}" \
      --arg salt "${salt_b64}" \
      --argjson iterations "${iterations}" \
      '{
        username: $username,
        enabled: true,
        email: $email,
        emailVerified: true,
        firstName: "AgentCiCi",
        lastName: "User",
        requiredActions: [],
        credentials: [{
          type: "password",
          temporary: false,
          secretData: ({value: $hash, salt: $salt, additionalParameters: {}} | tojson),
          credentialData: ({hashIterations: $iterations, algorithm: "pbkdf2-sha256", additionalParameters: {}} | tojson)
        }]
      }' > "${user_json}"
    kcadm create users -r agentcici -f "${user_json}" >/dev/null
    existing="$(kcadm get users -r agentcici -q "username=${username}" --fields id,username | jq -r --arg username "${username}" '.[] | select(.username == $username) | .id' | head -n 1)"
  fi
  if [[ -z "${existing}" ]]; then
    printf '%s\n' 'Could not resolve imported Keycloak user.' >&2
    exit 1
  fi
  printf '%s\t%s\n' "${account_id}" "${existing}" >> "${output}.partial"
  count=$((count + 1))
done < "${input}"

if [[ "${count}" -eq 0 || "$(cut -f1 "${output}.partial" | sort -u | wc -l | tr -d ' ')" -ne "${count}" || "$(cut -f2 "${output}.partial" | sort -u | wc -l | tr -d ' ')" -ne "${count}" ]]; then
  printf '%s\n' 'Identity import mapping failed uniqueness validation.' >&2
  exit 1
fi

mv "${output}.partial" "${output}"
printf 'keycloak-identities-imported:%s\n' "${count}"
