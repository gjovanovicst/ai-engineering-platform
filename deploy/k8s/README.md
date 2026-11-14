# Deploying on Kubernetes

## What these manifests assume

- Postgres is reachable at `AIP_DB_DSN` (managed database or the operator's
  own StatefulSet; not included here on purpose).
- The API image contains `aipd`; the worker image contains `aip-worker` and
  `aipctl`.
- TLS terminates at the ingress.

## Order

```bash
kubectl apply -f deploy/k8s/namespace.yaml
kubectl create secret generic aip-secrets -n aip --from-env-file=.env.production \
  --from-file=jwt.pem
# secret.yaml is intentionally not part of kustomize; the command above creates it.
kubectl apply -k deploy/k8s
kubectl wait -n aip --for=condition=complete job/aip-migrate --timeout=300s
kubectl rollout status -n aip deployment/aip-api deployment/aip-worker
```

Run the migration Job **before** rolling out a new image and delete the old Job
first (`kubectl delete job aip-migrate`), because Job specs are immutable.

## Sandbox driver

| Cluster | Driver | Notes |
|---|---|---|
| k3s / self-hosted with Docker | `docker` (today) | the worker mounts `/var/run/docker.sock` and creates one container per run; `--network none` keeps it isolated |
| managed (EKS/GKE/AKS) | `k8s` (implemented) | one Job per run: the worker creates the Job, execs into the pod, exports the workspace and deletes the Job; RBAC in `worker-rbac.yaml`, network denied with a NetworkPolicy |

Choose the driver with `AIP_SANDBOX_DRIVER` (`docker` or `k8s`). The configmap
uses `k8s`; switch it to `docker` and drop the socket mount for k3s/self-host.
The sandbox image must be pullable by the cluster.

## Scaling and failure behaviour

- API replicas are stateless; SSE polls Postgres, so no sticky sessions.
- One worker replica is enough to start; each worker runs up to 4 jobs.
  Scale with the queue depth (`select count(*) from river_job where state='available'`).
- The worker must have access to git over HTTPS (`GITHUB_TOKEN`/`GITLAB_TOKEN`)
  and to the git host for pushes; the sandbox itself stays offline.
- Graceful shutdown is 30s; in-flight runs are retried by River.

## Probes

- `/healthz` — process liveness.
- `/readyz` — database ping; use it for readiness only.

## Image verification

`kyverno-verify-image.yaml` is a Kyverno ClusterPolicy that requires a keyless
cosign signature from `OWNER/REPO` on `ghcr.io/OWNER/REPO/api*` images. Replace
the placeholders and apply it after the image-release workflow has published and
signed the digest. The workflow verifies with cosign before push; this policy
enforces the same contract at admission time.
