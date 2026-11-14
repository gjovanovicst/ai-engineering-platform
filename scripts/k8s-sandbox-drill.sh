#!/usr/bin/env bash
# Live Kubernetes sandbox drill: creates a kind cluster, runs the opt-in Go test
# (Job -> Exec -> MCP process -> cleanup), and removes the cluster unless KEEP=1.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT=$PWD
CLUSTER=${KIND_CLUSTER:-aip-sandbox}
KUBECONFIG_PATH=${KUBECONFIG_PATH:-$ROOT/.cache/kind-kubeconfig}
IMAGE=${AIP_K8S_IMAGE:-alpine:latest}
KIND_BIN=${KIND_BIN:-kind}
export GOCACHE=${GOCACHE:-$ROOT/.cache/go-build}
export GOMODCACHE=${GOMODCACHE:-$ROOT/.cache/gomod}
export GOPATH=${GOPATH:-$ROOT/.cache/gopath}

command -v "$KIND_BIN" >/dev/null || { echo "kind is required (https://kind.sigs.k8s.io/)" >&2; exit 1; }
mkdir -p "$(dirname "$KUBECONFIG_PATH")"

cleanup() {
  if [ "${KEEP:-0}" = "1" ]; then
    echo "[k8s] keeping cluster $CLUSTER (KEEP=1)"
    return
  fi
  "$KIND_BIN" delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

if ! "$KIND_BIN" get clusters | grep -qx "$CLUSTER"; then
  echo "[k8s] creating kind cluster $CLUSTER"
  "$KIND_BIN" create cluster --name "$CLUSTER" --kubeconfig "$KUBECONFIG_PATH" --wait 120s
else
  echo "[k8s] reusing kind cluster $CLUSTER"
  "$KIND_BIN" get kubeconfig --name "$CLUSTER" > "$KUBECONFIG_PATH"
fi
chmod 600 "$KUBECONFIG_PATH"
export KUBECONFIG=$KUBECONFIG_PATH

echo "[k8s] loading $IMAGE into the cluster"
if ! "$KIND_BIN" load docker-image "$IMAGE" --name "$CLUSTER"; then
  echo "[k8s] image load failed (multi-platform digest); the pod will pull it" >&2
fi

echo "[k8s] running TestKubernetesLiveDrill"
cd "$ROOT/apps/api"
AIP_K8S_DRILL=1 AIP_K8S_IMAGE="$IMAGE" go test ./internal/agent/sandbox -run TestKubernetesLiveDrill -v

echo "[k8s] drill passed"
