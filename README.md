# CallKeeper

Telegram control and monitoring for Asterisk PJSIP. Go 1.22+, Linux, systemd. No external Go dependencies, CGO, Python or SQLite required by the Go runtime.

## Install

As root on Ubuntu 24.04:

    apt update
    apt install -y git golang-go
    mkdir -p /apps
    git clone -b develop https://github.com/ttanga/CallKeeper.git /apps/callkeeper
    cd /apps/callkeeper
    bash install.sh
    callkeeper edit
    callkeeper check
    callkeeper enable
    callkeeper start

Set bot_token, chat_id, user_ids, phones, default_target and sip_host in config.json. Keep provision_enabled false until the separate provisioning setup is verified. Never commit credentials.

## Upgrade from Python

Back up config.json and state/ securely. Do not delete the old checkout or stop the working service before compilation. Run as root:

    cd /apps/callkeeper
    git pull --ff-only
    bash install.sh
    callkeeper status
    callkeeper logs

Do not use the old installed update wrapper for this first upgrade. The installer builds and tests before stopping the service and attempts rollback on startup failure. Old SQLite state is not imported; settings and history start fresh. The current route remains in Asterisk AstDB. Do not run two Telegram polling consumers.

## Commands

    callkeeper update
    callkeeper start
    callkeeper stop
    callkeeper restart
    callkeeper status
    callkeeper logs
    callkeeper follow
    callkeeper edit
    callkeeper check
    callkeeper test
    callkeeper version

## Storage and monitoring

config.json is operator configuration. state/state.json contains notification settings, devices, history, polling offset and persistent notification/deletion queues. The state is atomically replaced with mode 0600. History is limited to 1000 entries. Notification delivery is FIFO with retry; duplicates are possible after an ambiguous network failure or crash. Ordinary Telegram requests have 15-second deadlines; long polling has a 35-second deadline.

## SIP provisioning

Optional. The socket worker runs as asterisk, not root. Requires operator-approved dynamic dialplan, a managed PJSIP include, TLS transport, and the systemd socket. Do not enable provisioning until these are configured and verified. Credentials are sent with protect_content and scheduled for deletion; Telegram is not end-to-end encrypted and deletion is best effort. Passwords remain in Asterisk configuration.

## Development

    go test -race ./...
    go vet ./...
    CGO_ENABLED=0 go build -o callkeeper-bin .
    bash -n install.sh manage.sh

GitHub Actions runs these checks. Live Asterisk and Telegram integration must be tested on the target server. Legacy Python files are retained temporarily for rollback.
