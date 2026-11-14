// Package imageverify verifies container image signatures.
package imageverify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Verifier verifies an image reference.
type Verifier interface {
	Verify(ctx context.Context, image string) error
}

// Cosign runs `cosign verify` with a pinned identity and issuer.
type Cosign struct {
	Bin      string
	Identity string
	Issuer   string
	Key      string
	Timeout  time.Duration
}

// New returns nil when no identity is configured (verification disabled).
func New(bin, identity, issuer, key string) *Cosign {
	identity = strings.TrimSpace(identity)
	key = strings.TrimSpace(key)
	if identity == "" && key == "" {
		return nil
	}
	if strings.TrimSpace(bin) == "" {
		bin = "cosign"
	}
	return &Cosign{Bin: bin, Identity: identity, Issuer: strings.TrimSpace(issuer), Key: key, Timeout: 30 * time.Second}
}

// Verify implements Verifier.
func (c *Cosign) Verify(ctx context.Context, image string) error {
	if c == nil {
		return nil
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"verify"}
	if c.Key != "" {
		args = append(args, "--key", c.Key, "--insecure-ignore-tlog", "--allow-insecure-registry")
	} else {
		args = append(args, "--certificate-identity", c.Identity)
		if c.Issuer != "" {
			args = append(args, "--certificate-oidc-issuer", c.Issuer)
		}
	}
	args = append(args, image)
	out, err := exec.CommandContext(runCtx, c.Bin, args...).CombinedOutput() //nolint:gosec // binary and identity are operator config; image is digest-pinned
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if len(detail) > 300 {
			detail = detail[:300]
		}
		return fmt.Errorf("cosign verify %s: %w: %s", image, err, detail)
	}
	return nil
}
