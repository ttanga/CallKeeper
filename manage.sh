#!/bin/bash
set -euo pipefail
cd /apps/callkeeper
case "${1:-}" in
 start|stop|restart|enable|disable) systemctl "$1" callkeeper ;;
 status) systemctl status callkeeper --no-pager ;;
 logs) journalctl -u callkeeper -n 100 --no-pager ;;
 follow) journalctl -u callkeeper -f ;;
 edit) "${EDITOR:-vim}" config.json ;;
 version) git rev-parse --short HEAD ;;
 test) python3 -m unittest -v ;;
 update)
   test "$(id -u)" = 0
   systemctl stop callkeeper
   git pull --ff-only
   bash install.sh
   systemctl start callkeeper
   ;;
 *) echo 'Usage: callkeeper {start|stop|restart|enable|disable|status|logs|follow|edit|version|test|update}'; exit 1 ;;
esac
