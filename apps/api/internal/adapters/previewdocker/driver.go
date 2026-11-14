// Package previewdocker runs preview containers on the worker host.
package previewdocker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

// Driver implements previews.Driver with docker CLI.
type Driver struct{ Binary string }

// New returns a Docker-backed preview driver.
func New() *Driver { return &Driver{Binary: "docker"} }

func (d *Driver) bin() string {
	if d.Binary == "" {
		return "docker"
	}
	return d.Binary
}

// Start runs the image, publishes the container port on a random loopback port
// and returns the URL plus container id.
func (d *Driver) Start(ctx context.Context, image string, port int) (string, string, error) {
	name := "aip-prev-" + randomHex(6)
	args := []string{"run", "-d", "--rm", "--name", name, "-p", fmt.Sprintf("127.0.0.1::%d", port), image}
	//nolint:gosec // docker is the preview boundary; arguments are argv
	cmd := exec.CommandContext(ctx, d.bin(), args...)
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("preview: start: %w", err)
	}
	containerID := strings.TrimSpace(string(out))
	//nolint:gosec // fixed argv
	portCmd := exec.CommandContext(ctx, d.bin(), "port", name, fmt.Sprintf("%d/tcp", port))
	portOut, err := portCmd.Output()
	if err != nil {
		_ = d.Stop(ctx, containerID)
		return "", "", fmt.Errorf("preview: resolve port: %w", err)
	}
	addr := strings.TrimSpace(string(portOut))
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		addr = addr[i+1:]
	}
	if addr == "" {
		_ = d.Stop(ctx, containerID)
		return "", "", fmt.Errorf("preview: empty port")
	}
	return "http://127.0.0.1:" + addr, containerID, nil
}

// Stop removes the container (idempotent).
func (d *Driver) Stop(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}
	//nolint:gosec // container id from our own start call
	cmd := exec.CommandContext(ctx, d.bin(), "rm", "-f", containerID)
	if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "No such container") {
		return fmt.Errorf("preview: stop: %w", err)
	}
	return nil
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
