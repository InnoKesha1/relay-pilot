#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
test "$(id -u)" = 0
test "$(uname -m)" = x86_64
# Use only on the three new dedicated Debian 12/Ubuntu 24.04 pilot VPS.
apt-get update
apt-get install -y ca-certificates curl python3 sudo openssh-server openssl
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
curl --fail --location --retry 3 -o "$tmp/core.tar.gz" https://github.com/SagerNet/sing-box/releases/download/v1.13.0/sing-box-1.13.0-linux-amd64.tar.gz
echo '86db0f9df3f822ca2adccde2f7c1c9e21d64e646a77bf258274fde3be399025b  '"$tmp/core.tar.gz" | sha256sum --check --status
tar -xzf "$tmp/core.tar.gz" -C "$tmp"
install -m755 "$tmp/sing-box-1.13.0-linux-amd64/sing-box" /usr/local/bin/sing-box
install -d -m700 /etc/relaypilot/node
install -m755 relaypilot-node /usr/local/sbin/relaypilot-node
install -m644 relaypilot-node.service relaypilot-guard.service relaypilot-guard.timer /etc/systemd/system/
id relaydeploy >/dev/null 2>&1 || useradd --create-home --shell /bin/bash relaydeploy
printf '%s\n' 'relaydeploy ALL=(root) NOPASSWD: /usr/local/sbin/relaypilot-node apply' > /etc/sudoers.d/relaypilot
chmod 440 /etc/sudoers.d/relaypilot
visudo -cf /etc/sudoers.d/relaypilot
systemctl daemon-reload
systemctl enable relaypilot-node.service
systemctl enable --now relaypilot-guard.timer
echo 'Install the dedicated controller public SSH key for relaydeploy, then issue the first sync.'
