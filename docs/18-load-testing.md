# 18 · Load and soak testing

Goal: know where the queue backs up and whether sandboxes leak, before a
customer finds out.

## Run it

```bash
LOAD_N=8  make loadtest          # smoke (about a minute)
LOAD_N=50 make loadtest          # soak; expect several minutes with one worker
KEEP=1 LOAD_N=5 make loadtest    # keep the stack for inspection
```

The script boots a real stack (Postgres, API, worker, Docker sandbox), registers
a throwaway org, creates `N` tasks, starts one development run per task and waits
until the queue drains. Each development run also enqueues its review run, so the
number of runs is `2N`.

## What it asserts

| Signal | Failure condition |
|---|---|
| Failed runs | any `failed` run |
| Queue drain | still `queued`/`running` after `N*60 + 120` s |
| Run duration p95 | above `LOAD_P95_LIMIT` (default 120 s) |
| Sandbox leaks | any `aip-sbx-*` container left behind |

## Baseline (single worker, 4 job slots, local Docker)

`LOAD_N=8`, sandbox image `aip/sandbox:go1.25-node22`, agent writes one file and
emits an empty review:

```
succeeded=16 failed=0
queue_wait_p50=0.0s p95=0.1s
run_p95=4.6s
8 tasks drained in 53s (~0.2 tasks/s)
sandbox_containers=0
```

Interpretation: with the echo agent the bottleneck is the sandbox lifecycle
(create, exec, export, destroy), not the model. Real agents will dominate run
time; this test tells you the platform overhead.

## Scaling rules of thumb

- Throughput scales with `MaxWorkers` (River queue config, currently 4) until
  the host runs out of CPU or memory; each sandbox reserves 2 CPU / 4 GiB.
- Watch `aip_run_queued_oldest_seconds` while ramping: if it grows linearly with
  `N`, add a worker; if it spikes then drains, the queue is fine.
- On Kubernetes the Job-per-run driver adds scheduler latency (~seconds per
  run); raise the concurrency before raising the job slots.

## Soak (longer run)

For a multi-hour soak, keep the stack (`KEEP=1`) and run the make target in a
loop; after each iteration check:

```sql
select status, count(*) from runs group by 1;
select count(*) from river_job where state = 'available';
```

and on the host:

```bash
docker ps -q --filter name=aip-sbx- | wc -l
```

A monotonically growing sandbox count means `Destroy` is not reached (check
worker shutdowns and `sandbox: destroy` logs).
