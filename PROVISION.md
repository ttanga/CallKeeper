# Provisioning (manual preflight required)

Do not enable on an unverified PBX. As root inspect the current dialplan and TLS transport. The inbound extension 2000 must route to a numeric AstDB target including newly allocated IDs 1004–1999; validate the target against existing endpoints. Preserve existing outbound rules. The gateway must call 2000.

Create /etc/asterisk/pjsip_callkeeper.conf as root:asterisk mode 0640 (do not overwrite existing content), add #include "pjsip_callkeeper.conf" to pjsip.conf once, reload and verify. Install helper using install -o root -g root -m 0755 callkeeper-provision /usr/local/sbin/callkeeper-provision. Use visudo to grant only /usr/local/sbin/callkeeper-provision with no arguments to the service account. The current service NoNewPrivileges and ProtectSystem settings prevent sudo and writing /etc: an operator must configure a separate privileged helper service or explicitly relax those restrictions before enabling. Do not enable until this integration is reviewed. The helper itself accepts only a random hex password via stdin and never arbitrary CLI commands.

After successful preflight create root-owned /etc/asterisk/callkeeper-provision.enabled and set provision_enabled true. New phone descriptions persist in SQLite. Password delivery is protected against ordinary forwarding and deletion is retried after restart; Telegram is not E2EE. If sending fails after creation, reset credentials manually. Deletion is best effort.

This is a development preview, not a validated production installer.
