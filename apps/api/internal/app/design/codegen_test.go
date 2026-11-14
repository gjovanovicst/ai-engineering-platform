package design

import (
	"strings"
	"testing"
)

func TestCodegen(t *testing.T) {
	tokens := []Token{{Name: "Color/Brand", Value: "#4f46e5"}, {Name: "Radius md", Value: "6px"}}
	css := CSS(tokens)
	if !strings.Contains(css, "--color-brand: #4f46e5;") || !strings.Contains(css, "--radius-md: 6px;") {
		t.Fatalf("css = %s", css)
	}
	ts := TypeScript(tokens)
	if !strings.Contains(ts, `"color-brand": "#4f46e5"`) {
		t.Fatalf("ts = %s", ts)
	}
	if _, _, err := Render("xml", tokens); err == nil {
		t.Fatal("unknown format must fail")
	}
}
