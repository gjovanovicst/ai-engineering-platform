package runs

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/embeddings"
)

// Embedder turns texts into vectors.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

type semanticDoc struct {
	path string
	text string
}

// SemanticRepoMap ranks repository files by embedding similarity to the prompt.
// It returns nil when there are no files or the embedder fails.
func SemanticRepoMap(ctx context.Context, archive []byte, prompt string, embedder Embedder, limit int) []string {
	if embedder == nil || limit <= 0 {
		return nil
	}
	docs := semanticDocs(archive, 50)
	if len(docs) == 0 {
		return nil
	}
	inputs := []string{prompt}
	for _, doc := range docs {
		text := doc.text
		if len(text) > 4000 {
			text = text[:4000]
		}
		inputs = append(inputs, doc.path+"\n"+text)
	}
	vectors, err := embedder.Embed(ctx, inputs)
	if err != nil || len(vectors) != len(inputs) {
		return nil
	}
	order := embeddings.Rank(vectors[0], vectors[1:])
	out := make([]string, 0, limit)
	for _, index := range order {
		if len(out) >= limit || index >= len(docs) {
			break
		}
		out = append(out, docs[index].path)
	}
	return out
}

func semanticDocs(archive []byte, max int) []semanticDoc {
	var docs []semanticDoc
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF || err != nil || len(docs) >= max {
			return docs
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		lower := strings.ToLower(name)
		if hdr.Typeflag != tar.TypeReg || hdr.Size > 256<<10 || strings.Contains(lower, "vendor/") || strings.Contains(lower, "node_modules/") {
			continue
		}
		dot := strings.LastIndex(lower, ".")
		if dot < 0 || !textExt[lower[dot:]] {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(tr, 256<<10))
		docs = append(docs, semanticDoc{path: name, text: string(data)})
	}
}
