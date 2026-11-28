#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
test "$(id -u)" = 0
test "$(pwd -P)" = /apps/callkeeper
id asterisk >/dev/null
command -v go >/dev/null || { echo 'Install golang-go first'; exit 1; }
exec 9>/run/lock/callkeeper-install.lock
flock -n 9
export CGO_ENABLED=0
stage=$(mktemp -d /apps/callkeeper/.install.XXXXXX)
trap 'rm -rf "$stage"' EXIT
go test ./...
go vet ./...
go build -trimpath -o "$stage/callkeeper-bin" .
chown root:asterisk /apps/callkeeper
chmod 750 /apps/callkeeper
install -d -o asterisk -g asterisk -m 700 state
fresh=0
if [[ ! -e config.json ]]; then
 install -o root -g asterisk -m 640 config.example.json config.json
 fresh=1
fi
chown root:asterisk config.json
chmod 640 config.json
if [[ "$fresh" = 0 ]]; then
 "$stage/callkeeper-bin" --check
fi
active=0
if systemctl is-active --quiet callkeeper; then active=1; fi
unit=/etc/systemd/system/callkeeper.service
for f in callkeeper-bin "$unit" /usr/local/bin/callkeeper /etc/systemd/system/callkeeper-provision.socket /etc/systemd/system/callkeeper-provision@.service; do
 if [[ -f "$f" ]]; then cp -p "$f" "$stage/old-$(basename "$f")"; fi
done
rollback(){
 trap - ERR
 systemctl stop callkeeper || true
 for f in callkeeper-bin "$unit" /usr/local/bin/callkeeper /etc/systemd/system/callkeeper-provision.socket /etc/systemd/system/callkeeper-provision@.service; do
  if [[ -f "$stage/old-$(basename "$f")" ]]; then cp -p "$stage/old-$(basename "$f")" "$f"; else rm -f "$f"; fi
 done
 systemctl daemon-reload || true
 if [[ "$active" = 1 ]]; then systemctl start callkeeper || echo 'Rollback start failed'; fi
 echo 'Installation failed; previous executable and unit restored where available'
 exit 1
}
trap rollback ERR
if [[ "$active" = 1 ]]; then systemctl stop callkeeper; fi
if systemctl is-active --quiet callkeeper-provision.socket; then
 echo 'Stop provisioning socket before upgrading'
 false
fi
install -o root -g root -m 755 "$stage/callkeeper-bin" callkeeper-bin
install -m 644 callkeeper.service "$unit"
install -m 644 callkeeper-provision.socket callkeeper-provision@.service /etc/systemd/system/
install -o root -g root -m 755 manage.sh /usr/local/bin/callkeeper
systemctl daemon-reload
if [[ "$active" = 1 ]]; then
 systemctl start callkeeper
 sleep 7
 systemctl is-active --quiet callkeeper
fi
trap - ERR
echo 'Installed. For a fresh installation: callkeeper edit; callkeeper enable; callkeeper start'
echo 'Provisioning requires separate operator setup; socket not enabled automatically.'
