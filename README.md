# CallKeeper

Telegram-based monitoring and call routing management for an existing Asterisk PBX.

CallKeeper is a lightweight Python service for managing SIP phones and GSM-to-SIP gateways remotely. It uses Telegram long polling, SQLite and the Asterisk CLI. No Docker, external database, web server or additional inbound ports are required.

**Status: development preview.** Monitoring and routing code are present; live integration tests and automatic SIP provisioning installation are not complete. The `develop` branch contains the current implementation.

## Features

- Asterisk and SIP contact availability monitoring.
- Named devices and selection of one incoming-call recipient.
- Configurable outage notification delay, recovery alerts and outage history.
- Individual notification switches and global monitoring switch.
- Echo test call to the selected inbound route.
- Telegram private-chat authorization by numeric chat and user IDs.
- Experimental SIP account provisioning with TLS, mandatory SDES-SRTP, random passwords and best-effort deletion of protected Telegram password messages.

Parallel ringing, web UI, account deletion/password reset and automatic PBX migration are not implemented.

## Requirements

Ubuntu/Linux with systemd, Python 3, Git, a working Asterisk PJSIP installation (target: Asterisk 20), and outbound HTTPS access to Telegram. Existing SIP clients and routing must be configured separately. Create a Telegram bot using @BotFather and obtain its token and your numeric private chat/user IDs. Start the bot from your Telegram account. Do not share or commit the token.

## Installation

Run on the PBX server:

```bash
sudo mkdir -p /apps
cd /apps
sudo git clone --branch develop https://github.com/ttanga/CallKeeper.git callkeeper
cd callkeeper
sudo bash install.sh
sudoedit /apps/callkeeper/config.json
```

A private repository requires GitHub authentication; a public repository can be cloned without credentials. The installer runs directly in /apps/callkeeper, installs the systemd unit and management CLI, creates the state directory, and creates the configuration only if absent. It does not change Asterisk configuration or start the service.

Before starting, follow [DEPLOY.md](DEPLOY.md). Confirm that the GSM gateway calls extension 2000 in context from-gateway, that the route reads AstDB gateway/target, and that every configured target is allowed by the dialplan. Keep provisioning disabled.

```bash
sudo -u asterisk /usr/sbin/asterisk -rx "pjsip show contacts"
sudo systemctl enable --now callkeeper
sudo systemctl status callkeeper --no-pager
sudo journalctl -u callkeeper -f
```

Send /start in the authorized private Telegram chat. Check status, select an existing phone and confirm a test call. Only after verification, disable any previous monitoring service and enable CallKeeper monitoring in the bot. Do not run multiple getUpdates consumers or a webhook for the same bot token.

## Configuration

File: `/apps/callkeeper/config.json`. Start from [config.example.json](config.example.json).

```json
{
  "bot_token": "REPLACE",
  "chat_id": 123456789,
  "user_ids": [123456789],
  "poll_seconds": 3,
  "down_delay": 60,
  "monitor_enabled": false,
  "default_target": "1002",
  "phones": {
    "1001": {"name": "GSM Gateway", "target": false},
    "1002": {"name": "Primary phone", "target": true},
    "1003": {"name": "Second phone", "target": true}
  },
  "provision_enabled": false,
  "sip_host": "sip.example.com",
  "sip_port": 35061,
  "password_message_ttl": 60
}
```

All IDs and hostnames above are examples. Use actual numeric Telegram IDs and existing SIP IDs.

| Key | Meaning |
| --- | --- |
| bot_token | Telegram bot token; secret |
| chat_id | Authorized private chat and notification destination |
| user_ids | Allowed numeric Telegram user IDs |
| poll_seconds | Interval between monitoring iterations, seconds |
| down_delay | Default outage delay, seconds |
| monitor_enabled | Initial monitoring state; false avoids duplicate alerts |
| default_target | Fallback SIP recipient when AstDB target is absent; must match dialplan |
| phones | Existing SIP IDs mapped to device descriptions and target eligibility |
| phones.*.name | Human-readable device name |
| phones.*.target | Whether this device can be selected for incoming calls; not the current selection |
| provision_enabled | Experimental provisioning gate; leave false |
| sip_host | Hostname shown in new account instructions |
| sip_port | TLS port shown in new account instructions |
| password_message_ttl | Desired password message lifetime in seconds; deletion is best effort |

Bot changes to monitoring, notification switches and delay are persisted in SQLite and override initial defaults. The active recipient is stored in Asterisk AstDB, not in config.json. Restart CallKeeper after editing JSON. Keep configuration readable only by root and the service group.

## Telegram commands and controls

| Command | Action |
| --- | --- |
| /start | Open main menu |
| /menu | Open main menu |
| /status | Refresh status and menu |
| /cancel | Cancel pending device-name entry and return to menu |

The main menu shows Asterisk status, SIP contact availability and the selected recipient. Tap an eligible phone to select it. Test call requires confirmation and originates an Echo call through Local/2000@from-gateway. Notifications allows per-device switches, global monitoring and delays of 30, 60, 120 or 300 seconds. History shows the latest 15 completed observed outages. Add phone is experimental and disabled by default.

SIP OPTIONS availability does not prove cellular call delivery or audio quality. Monitoring intervals are approximate; CLI and Telegram latency can delay notifications. No recovery notification is expected for an outage that ended before its down alert.

## Service commands

These commands are available now via systemd:

| Operation | Command |
| --- | --- |
| Start | sudo systemctl start callkeeper |
| Stop | sudo systemctl stop callkeeper |
| Restart | sudo systemctl restart callkeeper |
| Status | sudo systemctl status callkeeper --no-pager |
| Enable at boot | sudo systemctl enable callkeeper |
| Disable at boot | sudo systemctl disable callkeeper |
| Recent logs | sudo journalctl -u callkeeper -n 100 --no-pager |
| Follow logs | sudo journalctl -u callkeeper -f |
| Edit settings | sudoedit /apps/callkeeper/config.json |

The installer provides `/usr/local/bin/callkeeper`. Run as root: `callkeeper start`, `stop`, `restart`, `enable`, `disable`, `status`, `logs`, `follow`, `edit`, `update`, `test`, or `version` (each as a separate subcommand). `test` runs unit tests, not a live call. `update` stops the service, pulls with --ff-only, installs and starts it; on failure it may remain stopped.

## Update

Back up configuration and state securely before updating. Stop the service for a consistent SQLite backup.

```bash
sudo systemctl stop callkeeper
cd /apps/callkeeper
sudo git pull --ff-only
sudo bash install.sh
sudo systemctl start callkeeper
sudo systemctl status callkeeper --no-pager
```

The installer preserves existing config.json and state. It does not automatically restart or roll back the service. Review changes before running the installer as root. Do not run an unattended update from an unreviewed development branch.

## Layout

```text
/apps/callkeeper/                 Git checkout and runtime
  callkeeper.py                   Application source
  config.json                     Local secrets and settings
  state/
    callkeeper.sqlite             Settings, devices, outage history
/etc/systemd/system/
  callkeeper.service              systemd unit
```

Repository files: callkeeper.py (bot and monitor), callkeeper-provision (experimental privileged helper), install.sh, callkeeper.service, config.example.json, test_callkeeper.py, DEPLOY.md, PROVISION.md and GitHub Actions Publish workflow.

## Experimental SIP provisioning

See [PROVISION.md](PROVISION.md). The helper is not installed by install.sh. Current systemd restrictions prevent its sudo invocation; a reviewed privilege boundary and PBX migration are required before enabling it. Do not weaken the service sandbox just to activate the button.

The intended flow is: Add phone → enter name → allocate SIP ID 1004–1999 → generate 64-character random password → update dedicated PJSIP include → reload and verify endpoint → authorize inbound routing in AstDB → send connection details. The dialplan must explicitly use the gateway_allowed list for new recipients. Existing hard-coded routes do not automatically support new phones.

Telegram password messages use protect_content and scheduled deletion, not end-to-end encryption. Deletion can fail. Passwords are stored in Asterisk authentication configuration; protect its files and backups. The service currently runs as asterisk and has broad PBX CLI access.

## Tests and release

```bash
python3 -m py_compile callkeeper.py callkeeper-provision
python3 -m unittest -v
bash -n install.sh && bash -n manage.sh
```

The Publish workflow is configured to run checks and build an archive; version tags can trigger prerelease publication. Successful CI and production deployment have not yet been confirmed. Git-based installation does not require a release archive.

## Troubleshooting and rollback

Check journalctl, Asterisk contact status and the inbound route:

```bash
sudo journalctl -u callkeeper -n 100 --no-pager
sudo asterisk -rx "pjsip show contacts"
sudo asterisk -rx "dialplan show 2000@from-gateway"
sudo asterisk -rx "database get gateway target"
```

To revert monitoring, stop CallKeeper and re-enable the previous monitor if present. The selected AstDB route remains unchanged. Do not delete the state database or SIP configuration without a backup. There is no automatic uninstall script yet.

Never commit real configuration, tokens, SIP credentials, private keys or SQLite state.
