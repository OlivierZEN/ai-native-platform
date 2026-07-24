#!/usr/bin/env bash
set -euo pipefail

# Installs Keycloak behind the already managed Nginx on the authorized ECS.
# Secrets are generated only on the target host and never printed by this script.

keycloak_version="${KEYCLOAK_VERSION:-26.7.0}"
keycloak_root="/opt/keycloak"
keycloak_release="${keycloak_root}/releases/keycloak-${keycloak_version}"
java_root="/opt/java"
pg_bin="/opt/postgresql/16.13/bin/psql"

if [[ "${EUID}" -ne 0 ]]; then
  printf '%s\n' 'Run this installer as root.' >&2
  exit 1
fi

if [[ ! -x "${pg_bin}" ]]; then
  printf '%s\n' "PostgreSQL client not found at ${pg_bin}." >&2
  exit 1
fi

install -d -m 0755 "${keycloak_root}/releases" "${java_root}"
install -d -m 0750 /etc/keycloak

if ! id keycloak >/dev/null 2>&1; then
  useradd --system --home-dir "${keycloak_root}" --shell /sbin/nologin keycloak
fi
chown root:keycloak /etc/keycloak

if [[ ! -x "${java_root}/current/bin/java" ]]; then
  java_archive="/var/tmp/amazon-corretto-21-x64-linux-jdk.tar.gz"
  curl --fail --location --retry 3 --silent --show-error \
    --output "${java_archive}" \
    https://corretto.aws/downloads/latest/amazon-corretto-21-x64-linux-jdk.tar.gz
  # `sed` consumes the complete stream; unlike `head`, it does not make tar
  # fail under `pipefail` with SIGPIPE after the first archive entry.
  java_dir_name="$(tar -tzf "${java_archive}" | sed -n '1p' | cut -d/ -f1)"
  tar -xzf "${java_archive}" -C "${java_root}"
  ln -sfn "${java_root}/${java_dir_name}" "${java_root}/current"
  rm -f "${java_archive}"
fi

if [[ ! -d "${keycloak_release}" ]]; then
  keycloak_archive="/var/tmp/keycloak-${keycloak_version}.tar.gz"
  if [[ ! -f "${keycloak_archive}" ]]; then
    curl --fail --location --retry 3 --silent --show-error \
      --output "${keycloak_archive}" \
      "https://github.com/keycloak/keycloak/releases/download/${keycloak_version}/keycloak-${keycloak_version}.tar.gz"
  fi
  tar -tzf "${keycloak_archive}" >/dev/null
  tar -xzf "${keycloak_archive}" -C "${keycloak_root}/releases"
  rm -f "${keycloak_archive}"
fi
ln -sfn "${keycloak_release}" "${keycloak_root}/current"

if [[ ! -f /etc/keycloak/database.env ]]; then
  keycloak_db_password="$(openssl rand -base64 48 | tr -d '\n')"
  umask 077
  printf 'KEYCLOAK_DB_PASSWORD=%s\n' "${keycloak_db_password}" > /etc/keycloak/database.env
else
  # shellcheck disable=SC1091
  source /etc/keycloak/database.env
  keycloak_db_password="${KEYCLOAK_DB_PASSWORD:?KEYCLOAK_DB_PASSWORD is required}"
fi
if [[ ! "${keycloak_db_password}" =~ ^[A-Za-z0-9+/=]{32,}$ ]]; then
  printf '%s\n' 'KEYCLOAK_DB_PASSWORD has an unsafe format.' >&2
  exit 1
fi
chmod 0640 /etc/keycloak/database.env
chown root:keycloak /etc/keycloak/database.env

if ! sudo -u postgres "${pg_bin}" -h /run/postgresql -tAc "SELECT 1 FROM pg_roles WHERE rolname = 'keycloak'" | grep -qx 1; then
  sudo -u postgres "${pg_bin}" -h /run/postgresql \
    -c "CREATE ROLE keycloak LOGIN PASSWORD '${keycloak_db_password}' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT"
fi
if ! sudo -u postgres "${pg_bin}" -h /run/postgresql -tAc "SELECT 1 FROM pg_database WHERE datname = 'keycloak'" | grep -qx 1; then
  sudo -u postgres "${pg_bin}" -h /run/postgresql -c "CREATE DATABASE keycloak OWNER keycloak ENCODING 'UTF8' TEMPLATE template0"
fi

cat > /etc/keycloak/keycloak.conf <<EOF
db=postgres
db-url=jdbc:postgresql://127.0.0.1:5432/keycloak
db-username=keycloak
db-password=${keycloak_db_password}
http-enabled=true
http-host=127.0.0.1
http-port=8180
http-management-port=9000
hostname=https://sso.agentcici.com
hostname-strict=true
proxy-headers=xforwarded
proxy-trusted-addresses=127.0.0.1
health-enabled=true
metrics-enabled=true
cache=local
EOF
chmod 0640 /etc/keycloak/keycloak.conf
chown root:keycloak /etc/keycloak/keycloak.conf
ln -sfn /etc/keycloak/keycloak.conf "${keycloak_root}/current/conf/keycloak.conf"

if [[ ! -f /etc/keycloak/bootstrap-admin.env ]]; then
  keycloak_admin_password="$(openssl rand -base64 48 | tr -d '\n')"
  umask 077
  cat > /etc/keycloak/bootstrap-admin.env <<EOF
KC_BOOTSTRAP_ADMIN_USERNAME=sso-admin
KC_BOOTSTRAP_ADMIN_PASSWORD=${keycloak_admin_password}
EOF
  cat > /root/keycloak-initial-admin.txt <<EOF
Keycloak initial administrative account
URL: https://sso.agentcici.com/admin/
Username: sso-admin
Password: ${keycloak_admin_password}
EOF
  chmod 0600 /root/keycloak-initial-admin.txt
fi
chmod 0640 /etc/keycloak/bootstrap-admin.env
chown root:keycloak /etc/keycloak/bootstrap-admin.env

install -m 0644 "$(dirname "$0")/keycloak.service" /etc/systemd/system/keycloak.service
install -m 0644 "$(dirname "$0")/nginx-sso.agentcici.com.conf" /etc/nginx/conf.d/sso.agentcici.com.conf

chown -R root:root "${keycloak_release}"
install -d -o keycloak -g keycloak -m 0750 "${keycloak_root}/current/data" "${keycloak_root}/current/data/tmp"

# Quarkus augmentation writes generated artifacts below lib/quarkus. Build as
# root while installing, then keep the release root-owned and run the server
# as the unprivileged keycloak account.
env JAVA_HOME="${java_root}/current" \
  "${keycloak_root}/current/bin/kc.sh" build --db=postgres --health-enabled=true --metrics-enabled=true
chown -R root:root "${keycloak_release}"
chown -R keycloak:keycloak "${keycloak_root}/current/data"

systemctl daemon-reload
nginx -t
systemctl enable --now keycloak.service
systemctl is-active --quiet keycloak.service
nginx -s reload
