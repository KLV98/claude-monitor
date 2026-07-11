#!/usr/bin/env bash
# dev.sh — run a dev instance of claude-monitor alongside the installed
# menu-bar app.
#
# The menu-bar app ("Claude Monitor.app") runs its own daemon on
# 127.0.0.1:8788. Starting the CLI on the default ports (8788 / 3737)
# collides with it ("bind: address already in use"), so this script pins
# the dev daemon + web to alternate ports.
#
# Usage:
#   scripts/dev.sh            # run ./bin/claude-monitor on the dev ports
#   scripts/dev.sh --build    # rebuild the Go binary first (make build-go)
#   scripts/dev.sh --open     # also open the web UI in the browser
#
# Override the ports via env vars:
#   DEV_DAEMON_ADDR=127.0.0.1:8799 DEV_WEB_PORT=3738 scripts/dev.sh
set -euo pipefail

DEV_DAEMON_ADDR="${DEV_DAEMON_ADDR:-127.0.0.1:8799}"
DEV_WEB_PORT="${DEV_WEB_PORT:-3738}"

# Resolve the repo root from this script's location so it works from anywhere.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin/claude-monitor"

build=0
open_flag="--no-open"
for arg in "$@"; do
	case "$arg" in
		--build) build=1 ;;
		--open)  open_flag="" ;;
		-h|--help)
			awk 'NR==1{next} /^#/{sub(/^# ?/,""); print; next} {exit}' "${BASH_SOURCE[0]}"
			exit 0 ;;
		*) echo "dev.sh: unknown argument: $arg" >&2; exit 2 ;;
	esac
done

if [ "$build" = "1" ] || [ ! -x "$BIN" ]; then
	echo "dev.sh: building Go binary…"
	make -C "$ROOT" build-go
fi

# Bail early with a friendly message if a dev port is already taken —
# usually a previous dev instance still running.
# `|| true`: lsof exits non-zero when nothing matches (the common, healthy
# case here — the port is free), which would trip `set -e`/pipefail.
port_pid() { lsof -ti "tcp:$1" -sTCP:LISTEN 2>/dev/null | head -1 || true; }
daemon_port="${DEV_DAEMON_ADDR##*:}"
for p in "$daemon_port" "$DEV_WEB_PORT"; do
	pid="$(port_pid "$p" || true)"
	if [ -n "$pid" ]; then
		echo "dev.sh: port $p is already in use (pid $pid)." >&2
		echo "        stop it with:  kill $pid" >&2
		exit 1
	fi
done

echo "dev.sh: daemon → http://$DEV_DAEMON_ADDR   web → http://127.0.0.1:$DEV_WEB_PORT"
echo "        (menu-bar app keeps 127.0.0.1:8788 / 3737)"
exec "$BIN" --daemon-addr "$DEV_DAEMON_ADDR" --web-port "$DEV_WEB_PORT" $open_flag
