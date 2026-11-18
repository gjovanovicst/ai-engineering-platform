package git

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Cloner clones the project repository on the host and returns a tar archive
// that is uploaded into the sandbox before the agent runs. The sandbox itself
// never gets network access or the git token.
type Cloner struct {
	Repos     RepoLookup
	Tokens    map[string]string
	TokenFunc func(ctx context.Context, provider string) (string, error)
	// RepoTokenFunc supplies the per-repository credential; it wins over the
	// App and global tokens. An error or an empty token falls through.
	RepoTokenFunc func(ctx context.Context, repo domain.Repo) (string, error)
	// OrgTokenFunc supplies a per-org GitHub App token; errors fall back.
	OrgTokenFunc func(ctx context.Context, orgID, provider string) (string, error)
	CloneBases   map[string]string
	GitBin       string
	// NPMDeps installs the repository's locked web dependencies on the host
	// before the workspace archive is built, so the offline sandbox can run
	// pnpm test/build. PnpmBin selects the pnpm binary.
	NPMDeps bool
	PnpmBin string
	Log     *slog.Logger
}

// Clone implements runs.WorkspaceProvider. A project without a repo returns
// nil, meaning an empty workspace (the previous behaviour).
func (c *Cloner) Clone(ctx context.Context, orgID, projectID string) ([]byte, error) {
	return c.CloneRef(ctx, orgID, projectID, "")
}

// CloneRef clones a specific ref (a pull-request branch for a review-fix run).
// An empty ref means the repository's default branch.
func (c *Cloner) CloneRef(ctx context.Context, orgID, projectID, ref string) ([]byte, error) {
	repo, err := c.Repos.ByProject(ctx, orgID, projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	token := c.Tokens[string(repo.Provider)]
	if c.RepoTokenFunc != nil {
		if fresh, terr := c.RepoTokenFunc(ctx, repo); terr == nil && fresh != "" {
			token = fresh
		}
	}
	if c.OrgTokenFunc != nil && repo.Provider == domain.ProviderGitHub {
		if fresh, terr := c.OrgTokenFunc(ctx, orgID, string(repo.Provider)); terr == nil && fresh != "" {
			token = fresh
		}
	}
	if c.TokenFunc != nil {
		fresh, terr := c.TokenFunc(ctx, string(repo.Provider))
		if terr != nil {
			return nil, terr
		}
		token = fresh
	}
	base := ref
	if base == "" {
		base = repo.DefaultBranch
		if base == "" {
			base = "main"
		}
	}

	dir, err := os.MkdirTemp("", "aip-clone-*")
	if err != nil {
		return nil, fmt.Errorf("git: temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	url := remoteURL(repo, token, c.CloneBases)
	if err := c.runClone(ctx, url, dir, base); err != nil {
		if ref != "" {
			return nil, fmt.Errorf("git: clone %s: %w", ref, err)
		}
		// An empty repository has no branch yet; clone it without --branch so
		// the run can still create the first commit.
		if retryErr := c.runClone(ctx, url, dir, ""); retryErr != nil {
			return nil, fmt.Errorf("%s", redactCredentials(err.Error(), url))
		}
	}

	if c.NPMDeps {
		if installed, derr := prepareNodeModules(ctx, dir, c.PnpmBin, 5*time.Minute); derr != nil {
			c.log().Warn("git: web dependencies not prepared; the run continues without node_modules", slog.Any("err", derr))
		} else if installed {
			c.log().Info("git: web dependencies prepared for the offline sandbox")
		}
	}
	return tarDir(dir)
}

func (c *Cloner) runClone(ctx context.Context, url, dir, branch string) error {
	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, dir)
	//nolint:gosec // git is the purpose; argv only, the URL is built from config
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/true")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git: clone %s: %w: %s", url, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Cloner) log() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.New(slog.DiscardHandler)
}

func (c *Cloner) bin() string {
	if c.GitBin == "" {
		return "git"
	}
	return c.GitBin
}

// tarDir archives the directory including .git so the agent can branch/diff.
func tarDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path) //nolint:gosec // path comes from Walk inside our temp dir
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		_, err = io.Copy(tw, file)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
