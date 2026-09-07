#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
test "$(id -u)" = 0
binary=${1:?Pass the built relaypilot-linux-amd64 binary path}
id relaypilot >/dev/null 2>&1 || useradd --system --home-dir /var/lib/relaypilot --shell /usr/sbin/nologin relaypilot
install -m755 "$binary" /usr/local/bin/relaypilot
install -d -m700 -o relaypilot -g relaypilot /var/lib/relaypilot /var/lib/relaypilot/backups /etc/relaypilot/ssh
install -m644 relaypilot-control.service relaypilot-backup.service relaypilot-backup.timer /etc/systemd/system/
sudo -u relaypilot /usr/local/bin/relaypilot init
systemctl daemon-reload
systemctl enable relaypilot-control.service
systemctl enable --now relaypilot-backup.timer
echo 'Install private inventory and SSH credentials before starting relaypilot-control.'
