#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
test "$(id -u)" = 0
test "$(pwd -P)" = /apps/callkeeper || { echo 'Clone into /apps/callkeeper first'; exit 1; }
id asterisk >/dev/null
command -v python3 >/dev/null
command -v git >/dev/null
python3 -m py_compile callkeeper.py callkeeper-provision
chown root:asterisk /apps/callkeeper
chmod 750 /apps/callkeeper
chown root:asterisk callkeeper.py
chmod 640 callkeeper.py
install -d -o asterisk -g asterisk -m 700 state
if [[ ! -e config.json ]]; then
    install -o root -g asterisk -m 640 config.example.json config.json
fi
chown root:asterisk config.json
chmod 640 config.json
install -m 644 callkeeper.service /etc/systemd/system/callkeeper.service
install -o root -g root -m 755 manage.sh /usr/local/bin/callkeeper
systemctl daemon-reload
echo 'Edit config with callkeeper edit, then systemctl enable --now callkeeper'
