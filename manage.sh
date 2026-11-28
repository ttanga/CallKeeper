#!/bin/bash
set -euo pipefail
cd /apps/callkeeper
case "${1:-}" in
 start|stop|restart|enable|disable) systemctl "$1" callkeeper ;;
 status) systemctl status callkeeper --no-pager ;;
 logs) journalctl -u callkeeper -n 100 --no-pager ;;
 follow) journalctl -u callkeeper -f ;;
 edit) "${EDITOR:-vim}" config.json ;;
 version) go version -m ./callkeeper-bin ;;
 test) CGO_ENABLED=0 go test ./... ;;
 check) ./callkeeper-bin --check ;;
 update)
   test "$(id -u)" = 0
   git pull --ff-only
   bash install.sh
   ;;
 *) echo 'Usage: callkeeper {start|stop|restart|enable|disable|status|logs|follow|edit|version|test|check|update}'; exit 1 ;;
esac
