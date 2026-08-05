#!/usr/bin/env bash
set -euo pipefail

# Run on the Keycloak host as root. The theme changes presentation only;
# Keycloak remains the owner of credentials, CSRF state and login sessions.

theme_source="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/themes/agentcici" && pwd)}"
theme_target="/opt/keycloak/current/themes/agentcici"
backup_root="/opt/keycloak/backups"
keycloak_bin="/opt/keycloak/current/bin/kcadm.sh"
keycloak_config="$(mktemp /run/keycloak-kcadm.XXXXXX)"
stage_dir="$(mktemp -d /opt/keycloak/.agentcici-theme.XXXXXX)"

cleanup() {
  rm -rf "${stage_dir}"
  rm -f "${keycloak_config}"
}
trap cleanup EXIT

if [[ "${EUID}" -ne 0 ]]; then
  printf '%s\n' 'Run this theme deployment as root.' >&2
  exit 1
fi
theme_stylesheet=""
if [[ -d "${theme_source}/login/resources/css" ]]; then
  theme_stylesheet="$(find "${theme_source}/login/resources/css" -maxdepth 1 -type f -name 'agentcici-*.css' -print -quit)"
fi
if [[ ! -f "${theme_source}/login/theme.properties" || -z "${theme_stylesheet}" ]]; then
  printf '%s\n' 'Theme source is incomplete.' >&2
  exit 1
fi
if [[ ! -x "${keycloak_bin}" || ! -f /etc/keycloak/bootstrap-admin.env ]]; then
  printf '%s\n' 'Keycloak installation or protected bootstrap configuration is unavailable.' >&2
  exit 1
fi

release_stamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="${backup_root}/${release_stamp}-before-agentcici-login-theme"
install -d -m 0755 "${backup_root}" "${theme_target%/*}"
export JAVA_HOME=/opt/java/current
export PATH="${JAVA_HOME}/bin:${PATH}"
# shellcheck disable=SC1091
source /etc/keycloak/bootstrap-admin.env
"${keycloak_bin}" config credentials --server http://127.0.0.1:8180 --realm master \
  --user "${KC_BOOTSTRAP_ADMIN_USERNAME}" --password "${KC_BOOTSTRAP_ADMIN_PASSWORD}" \
  --config "${keycloak_config}" >/dev/null
install -d -m 0755 "${backup_dir}"
"${keycloak_bin}" get realms/agentcici --config "${keycloak_config}" --fields loginTheme \
  > "${backup_dir}/realm-login-theme.before.json"

cp -a "${theme_source}" "${stage_dir}/agentcici"
find "${stage_dir}/agentcici" -type f -name '._*' -delete
chown -R root:root "${stage_dir}/agentcici"
find "${stage_dir}/agentcici" -type d -exec chmod 0755 {} +
find "${stage_dir}/agentcici" -type f -exec chmod 0644 {} +
if [[ -e "${theme_target}" ]]; then
  mv "${theme_target}" "${backup_dir}/agentcici"
fi
mv "${stage_dir}/agentcici" "${theme_target}"

"${keycloak_bin}" update realms/agentcici --config "${keycloak_config}" \
  -s loginTheme=agentcici \
  -s internationalizationEnabled=true \
  -s defaultLocale=zh-CN \
  -s 'supportedLocales=["zh-CN"]' \
  -s registrationEmailAsUsername=false \
  -s loginWithEmailAllowed=true \
  -s duplicateEmailsAllowed=false \
  -s editUsernameAllowed=false >/dev/null

# Do not let a presentation-only theme rollout reset the realm's identity
# semantics. With registrationEmailAsUsername=true Keycloak forces username to
# email, which overwrites AgentCiCi's phone/public-ID usernames on later saves.
"${keycloak_bin}" get realms/agentcici --config "${keycloak_config}" \
  --fields registrationEmailAsUsername,loginWithEmailAllowed,duplicateEmailsAllowed,editUsernameAllowed \
  | jq -e '
      .registrationEmailAsUsername == false
      and .loginWithEmailAllowed == true
      and .duplicateEmailsAllowed == false
      and .editUsernameAllowed == false
    ' >/dev/null

systemctl restart keycloak.service
for attempt in $(seq 1 24); do
  if curl --fail --silent --show-error http://127.0.0.1:9000/health/ready >/dev/null; then
    printf 'agentcici-login-theme-ready backup=%s\n' "${backup_dir}"
    exit 0
  fi
  sleep 2
done

printf '%s\n' 'Keycloak did not become ready after theme deployment.' >&2
exit 1
