#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
test "$(id -u)" = 0
id asterisk >/dev/null
install -d -o root -g asterisk -m 750 /apps/callkeeper
install -d -o asterisk -g asterisk -m 700 /apps/callkeeper/state
install -o root -g asterisk -m 640 callkeeper.py /apps/callkeeper/callkeeper.py
if [[ ! -e /apps/callkeeper/config.json ]]; then
    install -o root -g asterisk -m 640 config.example.json /apps/callkeeper/config.json
fi
install -m 644 callkeeper.service /etc/systemd/system/callkeeper.service
systemctl daemon-reload
echo 'Edit /apps/callkeeper/config.json, then systemctl enable --now callkeeper'
