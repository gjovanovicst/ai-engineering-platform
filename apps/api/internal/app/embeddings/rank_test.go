package embeddings

import "testing"

func TestCosineAndRank(t *testing.T) {
	if got := Cosine([]float32{1, 0}, []float32{1, 0}); got < 0.99 {
		t.Fatalf("identical = %v", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Fatalf("orthogonal = %v", got)
	}
	order := Rank([]float32{1, 0}, [][]float32{{0, 1}, {1, 0}, {0.9, 0.1}})
	if order[0] != 1 || order[2] != 0 {
		t.Fatalf("order = %v", order)
	}
}
