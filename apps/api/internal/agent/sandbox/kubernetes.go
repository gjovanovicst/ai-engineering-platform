package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

// Kubernetes runs one Job per sandbox. It is the production driver for managed
// clusters: the worker needs RBAC for jobs, pods and pods/exec.
type Kubernetes struct {
	Client          kubernetes.Interface
	Config          *rest.Config
	Namespace       string
	ServiceAccount  string
	PodReadyTimeout time.Duration

	// waitReady is replaceable in tests (a fake clientset has no controllers).
	waitReady func(ctx context.Context, name string) error
}

// NewKubernetes builds the driver from in-cluster config (or KUBECONFIG).
func NewKubernetes(namespace string) (*Kubernetes, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		cfg, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			return nil, fmt.Errorf("sandbox: kubernetes config: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("sandbox: kubernetes client: %w", err)
	}
	if namespace == "" {
		namespace = "aip"
	}
	return &Kubernetes{
		Client: client, Config: cfg, Namespace: namespace,
		PodReadyTimeout: 120 * time.Second,
	}, nil
}

var _ Sandbox = (*Kubernetes)(nil)

// Create starts a Job and waits for its pod to be running.
func (k *Kubernetes) Create(ctx context.Context, spec Spec) (Handle, error) {
	name := "aip-run-" + randomHex(6)
	image := spec.Image
	if image == "" {
		return Handle{}, fmt.Errorf("sandbox: image is required")
	}
	job := k.jobFor(name, image, spec)
	if _, err := k.Client.BatchV1().Jobs(k.Namespace).Create(ctx, job, metav1.CreateOptions{}); err != nil {
		return Handle{}, fmt.Errorf("sandbox: create job: %w", err)
	}
	if spec.NetworkDisabled {
		policy := k.networkPolicyFor(name)
		if _, err := k.Client.NetworkingV1().NetworkPolicies(k.Namespace).Create(ctx, policy, metav1.CreateOptions{}); err != nil {
			_ = k.destroy(context.WithoutCancel(ctx), name)
			return Handle{}, fmt.Errorf("sandbox: create network policy: %w", err)
		}
	}
	wait := k.waitReady
	if wait == nil {
		wait = k.waitForPod
	}
	if err := wait(ctx, name); err != nil {
		_ = k.destroy(context.WithoutCancel(ctx), name)
		return Handle{}, err
	}
	return Handle{ID: name}, nil
}

func quantityPtr(value string) *resource.Quantity {
	q := resource.MustParse(value)
	return &q
}

func (k *Kubernetes) jobFor(name, image string, spec Spec) *batchv1.Job {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = time.Hour
	}
	cpu := "1"
	if spec.CPU > 0 {
		cpu = fmt.Sprintf("%.1f", spec.CPU)
	}
	memory := "2Gi"
	if spec.MemoryMB > 0 {
		memory = fmt.Sprintf("%dMi", spec.MemoryMB)
	}
	env := make([]corev1.EnvVar, 0, len(spec.Env))
	for key, value := range spec.Env {
		env = append(env, corev1.EnvVar{Name: key, Value: value})
	}
	backoff := int32(0)
	deadline := int64(timeout.Seconds())
	runAs := int64(10001)
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	readOnlyRoot := true
	seccomp := &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: k.Namespace,
			Labels: map[string]string{"app": "aip-sandbox", "aip-run": name},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoff,
			ActiveDeadlineSeconds: &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "aip-sandbox", "aip-run": name}},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: k.ServiceAccount,
					SecurityContext:    &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot, RunAsUser: &runAs, FSGroup: &runAs, SeccompProfile: seccomp},
					Volumes: []corev1.Volume{
						{Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: quantityPtr("512Mi")}}},
						{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: quantityPtr("256Mi")}}},
						{Name: "home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: quantityPtr("64Mi")}}},
						{Name: "cache", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: quantityPtr("512Mi")}}},
					},
					Containers: []corev1.Container{{
						Name:       "agent",
						Image:      image,
						Command:    []string{"sh", "-c", "sleep infinity"},
						WorkingDir: "/workspace",
						Env:        env,
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &runAsNonRoot,
							RunAsUser:                &runAs,
							AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							ReadOnlyRootFilesystem:   &readOnlyRoot,
							SeccompProfile:           seccomp,
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "workspace", MountPath: "/workspace"},
							{Name: "tmp", MountPath: "/tmp"},
							{Name: "home", MountPath: "/home/agent"},
							{Name: "cache", MountPath: "/cache"},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(cpu),
								corev1.ResourceMemory: resource.MustParse(memory),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(cpu),
								corev1.ResourceMemory: resource.MustParse(memory),
							},
						},
					}},
				},
			},
		},
	}
}

func (k *Kubernetes) networkPolicyFor(name string) *networkingv1.NetworkPolicy {
	protocol := corev1.ProtocolTCP
	port := intstr.FromInt32(1)
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.Namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"aip-run": name}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			// A single impossible rule keeps both directions closed.
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: &protocol, Port: &port}},
			}},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: &protocol, Port: &port}},
			}},
		},
	}
}

func (k *Kubernetes) waitForPod(ctx context.Context, name string) error {
	deadline := time.Now().Add(k.PodReadyTimeout)
	for {
		pods, err := k.Client.CoreV1().Pods(k.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{"job-name": name}).String(),
		})
		if err != nil {
			return fmt.Errorf("sandbox: list pods: %w", err)
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}
			for _, status := range pod.Status.ContainerStatuses {
				if status.Ready {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("sandbox: job %s did not become ready", name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (k *Kubernetes) podFor(ctx context.Context, name string) (string, error) {
	pods, err := k.Client.CoreV1().Pods(k.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"job-name": name}).String(),
	})
	if err != nil {
		return "", fmt.Errorf("sandbox: list pods: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("sandbox: no running pod for job %s", name)
}

// Upload extracts the archive into /workspace inside the pod.
func (k *Kubernetes) Upload(ctx context.Context, h Handle, archive []byte) error {
	var stderr bytes.Buffer
	if err := k.execStream(ctx, h, []string{"tar", "-xf", "-", "-C", "/workspace"},
		bytes.NewReader(archive), io.Discard, &stderr); err != nil {
		return fmt.Errorf("sandbox: upload: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// WriteFile writes content through a shell with the path passed as $1.
func (k *Kubernetes) WriteFile(ctx context.Context, h Handle, path string, content []byte) error {
	var stderr bytes.Buffer
	err := k.execStream(ctx, h, []string{"sh", "-c", `mkdir -p "$(dirname "$1")" && cat > "$1"`, "sh", path},
		bytes.NewReader(content), io.Discard, &stderr)
	if err != nil {
		return fmt.Errorf("sandbox: write %s: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Exec runs a command and streams stdout lines.
func (k *Kubernetes) Exec(ctx context.Context, h Handle, cmd []string, onLine func(string) error) (Result, error) {
	var stdout, stderr bytes.Buffer
	var streamErr error
	err := k.execStream(ctx, h, cmd, nil, &streamWriter{buf: &stdout, onLine: onLine, onErr: func(e error) { streamErr = e }}, &stderr)
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if streamErr != nil {
		return result, streamErr
	}
	if err != nil {
		var codeErr utilexec.CodeExitError
		if errors.As(err, &codeErr) {
			result.ExitCode = codeErr.Code
			return result, nil
		}
		return result, err
	}
	return result, nil
}

// streamWriter captures stdout and forwards complete lines.
type streamWriter struct {
	buf     *bytes.Buffer
	onLine  func(string) error
	onErr   func(error)
	pending bytes.Buffer
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	w.pending.Write(p)
	for {
		line, err := w.pending.ReadString('\n')
		if err != nil {
			w.pending.WriteString(line)
			break
		}
		if w.onLine != nil {
			if err := w.onLine(strings.TrimRight(line, "\n")); err != nil {
				if w.onErr != nil {
					w.onErr(err)
				}
				return len(p), nil
			}
		}
	}
	return len(p), nil
}

// Export streams a tar of /workspace.
func (k *Kubernetes) Export(ctx context.Context, h Handle) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	if err := k.execStream(ctx, h, []string{"tar", "-cf", "-", "-C", "/workspace", "."}, nil, &stdout, &stderr); err != nil {
		return nil, fmt.Errorf("sandbox: export: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (k *Kubernetes) execStream(ctx context.Context, h Handle, cmd []string, stdin io.Reader, stdout, stderr io.Writer) error {
	pod, err := k.podFor(ctx, h.ID)
	if err != nil {
		return err
	}
	req := k.Client.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).Namespace(k.Namespace).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd, Stdin: stdin != nil, Stdout: stdout != nil, Stderr: stderr != nil, TTY: false,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(k.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin: stdin, Stdout: stdout, Stderr: stderr,
	})
}

// Destroy deletes the Job and its network policy. It is idempotent.
func (k *Kubernetes) Destroy(ctx context.Context, h Handle) error {
	if h.ID == "" {
		return nil
	}
	return k.destroy(ctx, h.ID)
}

func (k *Kubernetes) destroy(ctx context.Context, name string) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	background := metav1.DeletePropagationBackground
	err := k.Client.BatchV1().Jobs(k.Namespace).Delete(cleanup, name, metav1.DeleteOptions{PropagationPolicy: &background})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("sandbox: delete job: %w", err)
	}
	if err := k.Client.NetworkingV1().NetworkPolicies(k.Namespace).Delete(cleanup, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("sandbox: delete network policy: %w", err)
	}
	return nil
}

// k8sProcess exposes a remotecommand stream with the same Process contract as
// the Docker driver (used by MCP stdio servers).
type k8sProcess struct {
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	stderr *io.PipeReader
	cancel context.CancelFunc
	done   chan error
}

func (p *k8sProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *k8sProcess) Stdout() io.Reader     { return p.stdout }
func (p *k8sProcess) Stderr() io.Reader     { return p.stderr }
func (p *k8sProcess) Wait() error           { return <-p.done }
func (p *k8sProcess) Kill() error {
	p.cancel()
	_ = p.stdin.Close()
	return nil
}

func processCommand(cmdline []string, env map[string]string) []string {
	if len(env) == 0 {
		return cmdline
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(env)+1+len(cmdline))
	out = append(out, "env")
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return append(out, cmdline...)
}

// StartProcess runs an interactive command (MCP stdio) in the pod. Kubernetes
// exec cannot set environment variables, so the command is wrapped in `env`.
func (k *Kubernetes) StartProcess(ctx context.Context, h Handle, cmdline []string, env map[string]string) (Process, error) {
	if len(cmdline) == 0 {
		return nil, errors.New("sandbox: empty process command")
	}
	if k.Config == nil {
		return nil, errors.New("sandbox: kubernetes config is required")
	}
	pod, err := k.podFor(ctx, h.ID)
	if err != nil {
		return nil, err
	}
	cmd := processCommand(cmdline, env)
	req := k.Client.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).Namespace(k.Namespace).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd, Stdin: true, Stdout: true, Stderr: true, TTY: false,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(k.Config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("sandbox: kubernetes exec: %w", err)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	proc := &k8sProcess{stdin: stdinW, stdout: stdoutR, stderr: stderrR, cancel: cancel, done: make(chan error, 1)}
	go func() {
		defer close(proc.done)
		defer func() { _ = stdinR.Close() }()
		defer func() { _ = stdoutW.Close() }()
		defer func() { _ = stderrW.Close() }()
		proc.done <- executor.StreamWithContext(streamCtx, remotecommand.StreamOptions{
			Stdin: stdinR, Stdout: stdoutW, Stderr: stderrW,
		})
	}()
	return proc, nil
}
