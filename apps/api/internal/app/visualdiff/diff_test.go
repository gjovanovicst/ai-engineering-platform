package visualdiff

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngBytes(t *testing.T, fill color.Color, changeAt image.Point) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, fill)
		}
	}
	if changeAt.X >= 0 {
		img.Set(changeAt.X, changeAt.Y, color.RGBA{R: 255, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCompareThresholdAndMask(t *testing.T) {
	base := pngBytes(t, color.RGBA{A: 255}, image.Point{-1, -1})
	changed := pngBytes(t, color.RGBA{A: 255}, image.Pt(2, 2))
	res, err := ComparePNG(bytes.NewReader(base), bytes.NewReader(changed), 0, nil)
	if err != nil || res.Passed || res.ChangedPixels != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = ComparePNG(bytes.NewReader(base), bytes.NewReader(changed), 0.02, nil)
	if err != nil || !res.Passed {
		t.Fatalf("threshold res=%+v err=%v", res, err)
	}
	res, err = ComparePNG(bytes.NewReader(base), bytes.NewReader(changed), 0, []Rect{{X: 2, Y: 2, W: 1, H: 1}})
	if err != nil || !res.Passed || res.MaskedPixels != 1 {
		t.Fatalf("mask res=%+v err=%v", res, err)
	}
}
