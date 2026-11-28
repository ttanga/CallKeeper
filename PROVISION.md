# Go SIP provisioning

Keep provision_enabled false until operator setup is complete.

The gateway must call 2000@from-gateway. Configure the dialplan to validate numeric targets against AstDB gateway_allowed, preserving existing outbound rules. Seed existing permitted targets and verify routing before enabling new accounts.

Create /etc/asterisk/callkeeper owned by asterisk with mode 0700 and an empty phones.conf owned by asterisk with mode 0600 if absent. Never overwrite existing accounts. Add #include "callkeeper/phones.conf" to pjsip.conf once. Reload and verify TLS transport and existing endpoints. Create the root-owned /etc/asterisk/callkeeper-provision.enabled marker only after review.

The installer installs callkeeper-provision.socket and callkeeper-provision@.service. Enable the socket with systemctl enable --now callkeeper-provision.socket after preflight. Set provision_enabled true and restart callkeeper. Stop the socket before upgrades and restart it afterwards. The helper runs as asterisk with restricted filesystem access; no sudo rule is needed.

New device names persist in state/state.json. SIP passwords remain in the managed Asterisk configuration. Telegram credential delivery is protected against ordinary forwarding; deletion is best effort and Telegram is not E2EE. If delivery fails after account creation, reset credentials manually.

Live provisioning and dynamic routing require verification on the target PBX.
