package embeddings

import (
	"math"
	"sort"
)

// Cosine returns the cosine similarity of two vectors (0 on a zero vector or a
// dimension mismatch).
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Rank returns document indexes ordered by descending similarity to the query.
func Rank(query []float32, docs [][]float32) []int {
	type scored struct {
		index int
		score float64
	}
	items := make([]scored, len(docs))
	for i, doc := range docs {
		items[i] = scored{index: i, score: Cosine(query, doc)}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	out := make([]int, len(items))
	for i, item := range items {
		out[i] = item.index
	}
	return out
}
