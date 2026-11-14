# Patroni host fencing

Status: **design/runbook only; preflight blocked on this host**. No
watchdog or STONITH is available and nothing here is claimed as implemented.

## Why

Patroni demotes a leader that loses the DCS (verified in
A partition drill is kept in the private operations repository. That is
*self-fencing by the database process*. It does not cover:

- a host that is alive enough to keep serving stale reads but too broken to run
  Patroni/Postgres correctly
- a leader whose Postgres is paused (`SIGSTOP`, stuck I/O) while HAProxy still
  sees a healthy endpoint
- a network partition where the old primary keeps a client path

## Options

| Option | Requirement | Strength |
|---|---|---|
| watchdog (`/dev/watchdog`, `watchdog_mode: required`) | host device + Patroni config | hard reboot if Patroni stops petting |
| cloud STONITH (AWS/GCP/Azure API) | credentials + instance IDs | hard stop of the old instance |
| power/SAN fencing | PDU / SAN | strongest, slowest |

The development containers cannot provide any of these; do not add a fake
watchdog.

## Production shape (watchdog)

1. Host: load `softdog` or use a hardware watchdog; confirm `/dev/watchdog`.
2. Patroni: `watchdog_mode: required`, `watchdog_device: /dev/watchdog`.
3. systemd: `WatchdogSec` on the Patroni unit; Patroni pets only while it holds
   the leader lock.
4. HAProxy: keep the `/primary` check; fencing is the backstop, not the router.
5. Alert on watchdog timeouts and leader changes.

## Acceptance tests before "done"

1. `SIGSTOP` Patroni on the leader; the host reboots/stopped within the watchdog
   interval; the replica promotes; clients reconnect through HAProxy.
2. Partition the leader from etcd but keep clients: it must demote (already
   verified) **and** the host fencing must not fire spuriously.
3. Kill the primary host: replica promotion completes within the RTO; no writes
   are served by the dead instance.
4. Fencing failure is loud: alert + audit, never silent.
5. A fenced old primary cannot rejoin as a second writer; Patroni/HAProxy keeps
   it out until it catches up.

## Current state

- self-demotion on DCS loss: verified
- host watchdog/STONITH: absent
- cloud instance IDs/credentials: not configured
- no reboot/fencing drill has been run
