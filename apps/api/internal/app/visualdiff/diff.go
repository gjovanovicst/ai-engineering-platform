// Package visualdiff compares two screenshots with masks and a threshold.
package visualdiff

import (
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"math"
)

// Rect masks a region (pixels) that must be ignored.
type Rect struct {
	X, Y, W, H int
}

// Result is a deterministic comparison result.
type Result struct {
	Width, Height int
	ChangedPixels int
	TotalPixels   int
	DiffPercent   float64
	Threshold     float64
	MaskedPixels  int
	Passed        bool
}

// ComparePNG decodes both images and fails when the changed ratio exceeds
// threshold (0..1). Masks are applied to both images before the comparison.
func ComparePNG(baseline, actual io.Reader, threshold float64, masks []Rect) (Result, error) {
	base, err := png.Decode(baseline)
	if err != nil {
		return Result{}, fmt.Errorf("visualdiff: baseline: %w", err)
	}
	got, err := png.Decode(actual)
	if err != nil {
		return Result{}, fmt.Errorf("visualdiff: actual: %w", err)
	}
	if base.Bounds().Dx() != got.Bounds().Dx() || base.Bounds().Dy() != got.Bounds().Dy() {
		return Result{}, errors.New("visualdiff: image sizes differ")
	}
	if threshold < 0 || threshold > 1 {
		return Result{}, errors.New("visualdiff: threshold must be between 0 and 1")
	}
	w, h := base.Bounds().Dx(), base.Bounds().Dy()
	masked := make([]bool, w*h)
	maskedCount := 0
	for _, r := range masks {
		for y := max(0, r.Y); y < min(h, r.Y+r.H); y++ {
			for x := max(0, r.X); x < min(w, r.X+r.W); x++ {
				if !masked[y*w+x] {
					masked[y*w+x] = true
					maskedCount++
				}
			}
		}
	}
	changed := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if masked[y*w+x] {
				continue
			}
			if colorDistance(base.At(x, y), got.At(x, y)) > 0.01 {
				changed++
			}
		}
	}
	total := w*h - maskedCount
	ratio := 0.0
	if total > 0 {
		ratio = float64(changed) / float64(total)
	}
	return Result{
		Width: w, Height: h, ChangedPixels: changed, TotalPixels: total,
		DiffPercent: ratio * 100, Threshold: threshold * 100,
		MaskedPixels: maskedCount, Passed: ratio <= threshold,
	}, nil
}

func colorDistance(a, b color.Color) float64 {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	dr := float64(ar) - float64(br)
	dg := float64(ag) - float64(bg)
	db := float64(ab) - float64(bb)
	da := float64(aa) - float64(ba)
	return math.Sqrt(dr*dr+dg*dg+db*db+da*da) / 65535
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
