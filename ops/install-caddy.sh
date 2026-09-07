#!/usr/bin/env bash
set -euo pipefail
test "$(id -u)" = 0
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
curl --fail --location --retry 3 -o "$tmp/caddy.deb" https://github.com/caddyserver/caddy/releases/download/v2.10.2/caddy_2.10.2_linux_amd64.deb
echo '1ecdbfe369c3fa052fc1918db5f6896aed6c9777dc1bae95c873f751bf6a7a71  '"$tmp/caddy.deb" | sha256sum --check --status
dpkg -i "$tmp/caddy.deb"
echo 'Install the reviewed Caddyfile, validate it and reload Caddy before issuing node sync.'
