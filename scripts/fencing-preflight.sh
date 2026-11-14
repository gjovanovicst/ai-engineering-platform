#!/usr/bin/env bash
# Report whether this host can run a real Patroni fencing drill.
set -euo pipefail
ok=0

if [ -c /dev/watchdog ]; then
    echo "watchdog_device=/dev/watchdog"
    ok=1
else
    echo "watchdog_device=absent"
fi
watchdogs=$(ls /sys/class/watchdog 2>/dev/null | wc -l | tr -d ' ')
echo "watchdog_sysfs=$watchdogs"

if command -v systemctl >/dev/null 2>&1; then
    state=$(systemctl is-system-running 2>/dev/null || true)
    echo "systemd=$state"
    [ "$state" = "running" ] || [ "$state" = "degraded" ] && ok=$((ok+1))
else
    echo "systemd=absent"
fi

if [ -n "${AWS_INSTANCE_ID:-}" ] || [ -n "${GCP_INSTANCE_ID:-}" ] || [ -n "${AZURE_INSTANCE_ID:-}" ]; then
    echo "cloud_stonith=configured"
    ok=$((ok+1))
else
    echo "cloud_stonith=absent"
fi

if [ "$ok" -eq 0 ]; then
    echo "fencing_preflight=blocked: no watchdog, no systemd, no cloud STONITH" >&2
    exit 1
fi
echo "fencing_preflight=ok"
