package runs

import (
	"context"
	"strings"
	"testing"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	out := make([][]float32, len(inputs))
	for i, input := range inputs {
		if strings.Contains(strings.ToLower(input), "pagination") {
			out[i] = []float32{1, 0}
		} else {
			out[i] = []float32{0, 1}
		}
	}
	return out, nil
}

func TestSemanticRepoMapRanksByEmbedding(t *testing.T) {
	archive := tarFilesForMap(map[string]string{
		"pagination.go": "func paginate() {}",
		"billing.go":    "func charge() {}",
	})
	files := SemanticRepoMap(context.Background(), archive, "fix pagination", fakeEmbedder{}, 5)
	if len(files) == 0 || files[0] != "pagination.go" {
		t.Fatalf("files = %v", files)
	}
}
