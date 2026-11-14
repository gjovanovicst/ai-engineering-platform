package sandbox

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// TestKubernetesLiveDrill runs against a real cluster (kind in the drill
// script). It is skipped unless AIP_K8S_DRILL=1.
func TestKubernetesLiveDrill(t *testing.T) {
	if os.Getenv("AIP_K8S_DRILL") != "1" {
		t.Skip("set AIP_K8S_DRILL=1 with a reachable KUBECONFIG")
	}
	namespace := os.Getenv("AIP_K8S_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	image := os.Getenv("AIP_K8S_IMAGE")
	if image == "" {
		image = "alpine:latest"
	}
	k, err := NewKubernetes(namespace)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	h, err := k.Create(ctx, Spec{Image: image, NetworkDisabled: true, CPU: 0.2, MemoryMB: 128, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = k.Destroy(context.Background(), h) })

	res, err := k.Exec(ctx, h, []string{"sh", "-c", "echo k8s-ok"}, nil)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.Contains(res.Stdout, "k8s-ok") {
		t.Fatalf("exec stdout = %q", res.Stdout)
	}
	res, err = k.Exec(ctx, h, []string{"sh", "-c", "test -e /etc/aip-write && echo writable || echo readonly"}, nil)
	if err != nil {
		t.Fatalf("root probe: %v", err)
	}
	if !strings.Contains(res.Stdout, "readonly") {
		t.Fatalf("root filesystem is not read-only: %q", res.Stdout)
	}
	res, err = k.Exec(ctx, h, []string{"sh", "-c", "echo workspace-ok > /workspace/live && cat /workspace/live"}, nil)
	if err != nil {
		t.Fatalf("workspace write: %v", err)
	}
	if !strings.Contains(res.Stdout, "workspace-ok") {
		t.Fatalf("workspace not writable: %q", res.Stdout)
	}

	proc, err := k.StartProcess(ctx, h, []string{"cat"}, nil)
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	go func() {
		_, _ = proc.Stdin().Write([]byte("mcp-live\n"))
		_ = proc.Stdin().Close()
	}()
	out, err := io.ReadAll(proc.Stdout())
	if err != nil {
		t.Fatalf("read process: %v", err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("wait process: %v", err)
	}
	if string(out) != "mcp-live\n" {
		t.Fatalf("process stdout = %q", out)
	}
}
