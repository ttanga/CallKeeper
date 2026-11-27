# Deployment (Ubuntu / Asterisk 20)

Development preview. Existing SIP accounts and dialplan are not modified by installer.

1. Check `asterisk -rx "dialplan show 2000@from-gateway"` and ensure the gateway sends to 2000, and the dialplan reads AstDB gateway/target. The existing dialplan must allow every configured target. Default fallback must match default_target.
2. Clone develop into /apps/callkeeper and run `sudo bash install.sh`. Edit /apps/callkeeper/config.json as root. Supply token, numeric chat_id, numeric user_ids array and existing SIP IDs with descriptions. Never commit secrets.
3. Verify `sudo -u asterisk /usr/sbin/asterisk -rx "pjsip show contacts"` works. Service runs as asterisk (access to PBX control socket, not root).
4. Run `systemctl enable --now callkeeper` and `journalctl -u callkeeper -f`. Send /start to bot in authorized chat. Test route switching and echo.
5. Monitoring defaults off to avoid duplicate notifications. After testing, stop old asterisk-monitor.timer and enable monitoring via bot. Do not run another getUpdates consumer or webhook for same token.
6. Rollback: stop callkeeper, restart old timer. AstDB route remains as last selected; change manually if needed.

Limitations: OPTIONS reachability is not an actual cellular call test. History records observed outages, including short ones. Asterisk CLI failure is treated as Asterisk unavailable; endpoint state is not sampled during that time. Experimental provisioning is disabled pending integration; no parallel ringing or web UI. No live PBX or Telegram integration test yet.
