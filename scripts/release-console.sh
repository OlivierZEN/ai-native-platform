#!/usr/bin/env bash
# Pending production verification: deploys the Semattice read-only console only when
# an explicitly supplied, authorized SSH target is available. It never prints secrets.
set -euo pipefail

deploy_host="${SEMATTICE_DEPLOY_HOST:?set SEMATTICE_DEPLOY_HOST to the authorized target}"
deploy_user="${SEMATTICE_DEPLOY_USER:-root}"
deploy_domain="${SEMATTICE_DEPLOY_DOMAIN:-semattice.agentcici.com}"
identity_file="${SEMATTICE_DEPLOY_IDENTITY_FILE:-}"
release="$(date -u +%Y%m%dT%H%M%SZ)-console"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/semattice-console-release.XXXXXX")"
remote_stage="/tmp/semattice-console-${release}"

cleanup() { rm -rf "$work_dir"; }
trap cleanup EXIT

ssh_options=(-o BatchMode=yes -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new)
if [[ -n "$identity_file" ]]; then
  ssh_options+=(-i "$identity_file" -o IdentitiesOnly=yes)
fi
remote=(ssh "${ssh_options[@]}" "${deploy_user}@${deploy_host}")
copy_remote=(scp "${ssh_options[@]}")

cd "$repo_root"
GOTOOLCHAIN=go1.26.5 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' -o "$work_dir/semattice" ./cmd/ai-native-platform
tar -C deploy/semattice/www -czf "$work_dir/console-static.tgz" .
cp deploy/semattice/nginx.conf "$work_dir/semattice.conf"
binary_sha="$(shasum -a 256 "$work_dir/semattice" | awk '{print $1}')"

"${remote[@]}" "install -d -m 0700 '$remote_stage'"
"${copy_remote[@]}" "$work_dir/semattice" "$work_dir/console-static.tgz" "$work_dir/semattice.conf" "${deploy_user}@${deploy_host}:${remote_stage}/"

"${remote[@]}" bash -s -- "$release" "$remote_stage" "$binary_sha" "$deploy_domain" <<'REMOTE'
set -euo pipefail
release="$1"
stage="$2"
expected_sha="$3"
domain="$4"
release_path="/opt/semattice/releases/${release}"
static_root="/var/www/semattice"
static_backup="/var/www/semattice-backups/${release}"
nginx_config="/etc/nginx/conf.d/semattice.conf"
nginx_backup="${nginx_config}.backup.${release}"
previous_release="$(readlink -f /opt/semattice/current)"

test -n "$previous_release"
test -f /etc/semattice/semattice.env
test -f "$nginx_config"
test -d "$static_root"
test ! -e "$release_path"
actual_sha="$(sha256sum "$stage/semattice" | awk '{print $1}')"
test "$actual_sha" = "$expected_sha"
tar -tzf "$stage/console-static.tgz" | grep -Eq '(^|\./)console/index.html$'

if grep -q '^AI_NATIVE_CONSOLE_SESSION_HMAC_KEY=' /etc/semattice/semattice.env; then
  console_key="$(sed -n 's/^AI_NATIVE_CONSOLE_SESSION_HMAC_KEY=//p' /etc/semattice/semattice.env | tail -n 1)"
  test "${#console_key}" -ge 32
  unset console_key
else
  umask 077
  generated_console_key="$(openssl rand -hex 32)"
  printf '\nAI_NATIVE_CONSOLE_SESSION_HMAC_KEY=%s\n' "$generated_console_key" >> /etc/semattice/semattice.env
  unset generated_console_key
  chown root:semattice /etc/semattice/semattice.env
  chmod 0640 /etc/semattice/semattice.env
fi

cp -a "$nginx_config" "$nginx_backup"
install -m 0644 "$stage/semattice.conf" "$nginx_config"
if ! nginx -t; then
  cp -a "$nginx_backup" "$nginx_config"
  nginx -t
  exit 1
fi

install -d -m 0755 "$release_path"
install -m 0755 "$stage/semattice" "$release_path/semattice"
ln -sfn "$release_path" /opt/semattice/current
if ! systemctl restart semattice; then
  ln -sfn "$previous_release" /opt/semattice/current
  systemctl restart semattice
  exit 1
fi

install -d -m 0755 /var/www/semattice-backups
mv "$static_root" "$static_backup"
install -d -m 0755 "$static_root"
tar -C "$static_root" -xzf "$stage/console-static.tgz"
if ! systemctl reload nginx; then
  rm -rf "$static_root"
  mv "$static_backup" "$static_root"
  cp -a "$nginx_backup" "$nginx_config"
  nginx -t
  systemctl reload nginx
  ln -sfn "$previous_release" /opt/semattice/current
  systemctl restart semattice
  exit 1
fi

curl -fsS "https://${domain}/healthz" >/dev/null
curl -fsS "https://${domain}/console/" | grep -q '管理中心'
test "$(curl -sS -o /dev/null -w '%{http_code}' "https://${domain}/console/api/overview")" = '401'
rm -rf "$stage"
REMOTE

printf 'Released %s to %s@%s\n' "$release" "$deploy_user" "$deploy_host"
