package sandbox

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestKubernetes() *Kubernetes {
	k := &Kubernetes{
		Client: fake.NewSimpleClientset(), Namespace: "aip",
		PodReadyTimeout: time.Second,
	}
	k.waitReady = func(context.Context, string) error { return nil }
	return k
}

func TestKubernetesCreateAndDestroy(t *testing.T) {
	ctx := context.Background()
	k := newTestKubernetes()
	handle, err := k.Create(ctx, Spec{Image: "aip/sandbox:test", NetworkDisabled: true, CPU: 1, MemoryMB: 256})
	if err != nil {
		t.Fatal(err)
	}
	if handle.ID == "" {
		t.Fatal("empty handle")
	}

	job, err := k.Client.BatchV1().Jobs("aip").Get(ctx, handle.ID, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != "aip/sandbox:test" || container.WorkingDir != "/workspace" {
		t.Fatalf("container = %+v", container)
	}
	if container.SecurityContext == nil || container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
		t.Fatal("container must not run as root")
	}
	if container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatal("container root filesystem must be read-only")
	}
	if container.SecurityContext.SeccompProfile == nil || container.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatal("container must use the runtime-default seccomp profile")
	}
	if job.Spec.Template.Spec.SecurityContext == nil || job.Spec.Template.Spec.SecurityContext.SeccompProfile == nil {
		t.Fatal("pod must use the runtime-default seccomp profile")
	}
	mounts := map[string]bool{}
	for _, mount := range container.VolumeMounts {
		mounts[mount.Name] = true
	}
	for _, name := range []string{"workspace", "tmp", "home", "cache"} {
		if !mounts[name] {
			t.Fatalf("missing writable %s mount", name)
		}
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Fatal("sandbox jobs must not retry")
	}
	if _, err := k.Client.NetworkingV1().NetworkPolicies("aip").Get(ctx, handle.ID, metav1.GetOptions{}); err != nil {
		t.Fatalf("network policy: %v", err)
	}

	if err := k.Destroy(ctx, handle); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Client.BatchV1().Jobs("aip").Get(ctx, handle.ID, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("job still exists: %v", err)
	}
	if _, err := k.Client.NetworkingV1().NetworkPolicies("aip").Get(ctx, handle.ID, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("network policy still exists: %v", err)
	}
	// Destroy is idempotent.
	if err := k.Destroy(ctx, handle); err != nil {
		t.Fatalf("second destroy: %v", err)
	}
}

func TestKubernetesCreateRequiresImage(t *testing.T) {
	k := newTestKubernetes()
	if _, err := k.Create(context.Background(), Spec{Image: ""}); err == nil {
		t.Fatal("image is required")
	}
}

func TestKubernetesJobDeadlineAndResources(t *testing.T) {
	k := newTestKubernetes()
	job := k.jobFor("aip-run-test", "img", Spec{Timeout: 30 * time.Minute, CPU: 2, MemoryMB: 512})
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 1800 {
		t.Fatalf("deadline = %v", job.Spec.ActiveDeadlineSeconds)
	}
	resources := job.Spec.Template.Spec.Containers[0].Resources
	if resources.Limits.Cpu().String() != "2" {
		t.Fatalf("cpu limit = %s", resources.Limits.Cpu().String())
	}
}

func TestKubernetesProcessCommand(t *testing.T) {
	got := processCommand([]string{"mcp-server", "--stdio"}, map[string]string{"Z": "last", "A": "first"})
	want := []string{"env", "A=first", "Z=last", "mcp-server", "--stdio"}
	if len(got) != len(want) {
		t.Fatalf("cmd = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cmd = %v, want %v", got, want)
		}
	}
	if plain := processCommand([]string{"true"}, nil); len(plain) != 1 || plain[0] != "true" {
		t.Fatalf("no-env cmd = %v", plain)
	}
}

func TestKubernetesStartProcessNeedsConfig(t *testing.T) {
	k := newTestKubernetes()
	if _, err := k.StartProcess(context.Background(), Handle{ID: "run"}, []string{"true"}, nil); err == nil {
		t.Fatal("expected config error")
	}
}
