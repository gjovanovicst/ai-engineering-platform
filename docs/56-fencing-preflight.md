# Fencing preflight — blocked on this host

Status: **blocked**, 2026-11-05.

`scripts/fencing-preflight.sh` checks the three viable fencing paths before any
reboot drill.

## Result

```
watchdog_device=absent
watchdog_sysfs=0
systemd=offline
cloud_stonith=absent
fencing_preflight=blocked: no watchdog, no systemd, no cloud STONITH
```

- no `/dev/watchdog`
- `/sys/class/watchdog` is empty
- systemd is not PID 1 (WSL `init(Ubuntu)`), so `WatchdogSec` cannot drive a host
  reboot
- no cloud instance id/STONITH provider is configured

A fencing drill here would be a simulated reboot, not fencing. We do not claim it.

## To unblock

1. a Linux VM/host with `softdog` or hardware watchdog and systemd as PID 1
2. Patroni `watchdog_mode: required` + `watchdog_device: /dev/watchdog`
3. run the five acceptance tests in docs/52 (SIGSTOP Patroni, partition, kill the
   primary, loud failure, old primary cannot rejoin as a writer)

Until then the verified safety net is Patroni self-demotion + HAProxy `/primary`,
not host fencing.
