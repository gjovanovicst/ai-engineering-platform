package runs

import (
	"archive/tar"
	"bytes"
	"io"
	"sort"
	"strings"
)

var textExt = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".py": true,
	".md": true, ".sql": true, ".yaml": true, ".yml": true, ".json": true,
	".html": true, ".css": true, ".toml": true,
}

// RepoMap ranks repository files against the task prompt so the agent reads the
// relevant code first. It is a lightweight index, not a full symbol graph.
func RepoMap(archive []byte, prompt string, limit int) []string {
	if limit <= 0 {
		limit = 20
	}
	tokens := queryTokens(prompt)
	tr := tar.NewReader(bytes.NewReader(archive))
	type scored struct {
		path  string
		score int
	}
	var files []scored
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Size > 256<<10 {
			continue
		}
		path := strings.TrimPrefix(hdr.Name, "./")
		lower := strings.ToLower(path)
		dot := strings.LastIndex(lower, ".")
		if dot < 0 {
			// LICENSE, Makefile and Dockerfile have no extension.
			continue
		}
		ext := lower[dot:]
		if !textExt[ext] || strings.Contains(lower, "vendor/") || strings.Contains(lower, "node_modules/") {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(tr, 256<<10))
		content := strings.ToLower(string(data))
		score := 0
		for _, token := range tokens {
			if strings.Contains(lower, token) {
				score += 5
			}
			if strings.Contains(content, token) {
				score += 1 + strings.Count(content, token)
			}
		}
		if score > 0 {
			files = append(files, scored{path: path, score: score})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].score == files[j].score {
			return files[i].path < files[j].path
		}
		return files[i].score > files[j].score
	})
	out := make([]string, 0, limit)
	for _, file := range files {
		if len(out) >= limit {
			break
		}
		out = append(out, file.path)
	}
	return out
}

// RepoMapPrompt renders the map for the agent prompt.
func RepoMapPrompt(files []string) string {
	if len(files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("REPOSITORY MAP (read these before editing; search further with grep):\n")
	for _, file := range files {
		b.WriteString("- " + file + "\n")
	}
	return b.String()
}

func queryTokens(prompt string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range strings.FieldsFunc(prompt, func(r rune) bool { return !isQueryRune(r) }) {
		token := strings.ToLower(raw)
		if len(token) < 4 || seen[token] {
			continue
		}
		switch token {
		case "task", "with", "this", "that", "when", "then", "given", "from", "into", "your", "will", "should", "must":
			continue
		}
		seen[token] = true
		out = append(out, token)
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func isQueryRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
}
