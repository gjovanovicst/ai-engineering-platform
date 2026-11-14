package runs

import (
	"archive/tar"
	"bytes"
	"testing"
)

func TestVisualScreenshotsOnlyPNGUnderVisual(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Name: "visual/home.png", Mode: 0o644, Size: 3, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("png"))
	_ = tw.WriteHeader(&tar.Header{Name: "src/app.go", Mode: 0o644, Size: 3, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("go!"))
	_ = tw.Close()
	got := visualScreenshots(buf.Bytes())
	if len(got) != 1 || string(got["home"]) != "png" {
		t.Fatalf("screens = %v", got)
	}
}
