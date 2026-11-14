package runs

import (
	"archive/tar"
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Symbols extracts top-level Go symbols (and a small TS/JS heuristic) for the
// files selected by the repository map.
func Symbols(archive []byte, files []string, limit int) []string {
	if limit <= 0 {
		limit = 60
	}
	want := map[string]bool{}
	for _, file := range files {
		want[file] = true
	}
	contents := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if hdr.Typeflag != tar.TypeReg || !want[name] || hdr.Size > 256<<10 {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(tr, 256<<10))
		contents[name] = data
	}

	var out []string
	for name, data := range contents {
		var symbols []string
		switch {
		case strings.HasSuffix(name, ".go"):
			symbols = goSymbols(name, data)
		case strings.HasSuffix(name, ".ts"), strings.HasSuffix(name, ".tsx"), strings.HasSuffix(name, ".js"), strings.HasSuffix(name, ".jsx"):
			symbols = jsSymbols(data)
		}
		if len(symbols) == 0 {
			continue
		}
		if len(symbols) > 12 {
			symbols = symbols[:12]
		}
		out = append(out, name+": "+strings.Join(symbols, ", "))
		if len(out) >= limit {
			break
		}
	}
	sort.Strings(out)
	return out
}

func goSymbols(name string, data []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), name, data, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var out []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) > 0 {
				out = append(out, "func ("+receiverName(d.Recv.List[0].Type)+") "+d.Name.Name+"()")
			} else {
				out = append(out, "func "+d.Name.Name+"()")
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					out = append(out, "type "+typeSpec.Name.Name)
				}
			}
		}
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return "T"
}

var jsSymbol = regexp.MustCompile(`(?m)^\s*export\s+(?:async\s+)?(?:function|class|const|let)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)

func jsSymbols(data []byte) []string {
	matches := jsSymbol.FindAllSubmatch(data, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, string(match[1]))
	}
	return out
}

// SymbolPrompt renders the symbol hints.
func SymbolPrompt(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("KEY SYMBOLS (from the repository map):\n")
	for _, line := range lines {
		b.WriteString("- " + line + "\n")
	}
	return b.String()
}
