package boxloom

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestMarksOnlyThroughVocabulary は、計測と書き出しの印(irType・columns・cards・
// keepLine・nestedSlack・List・card)を部品が Spec の欄に直接書かず、配置の段の
// 語彙(layout.go の SpecOpt)だけで付けていることを確かめる。
func TestMarksOnlyThroughVocabulary(t *testing.T) {
	marks := map[string]bool{"irType": true, "columns": true, "cards": true, "pairs": true, "keepLine": true,
		"nestedSlack": true, "List": true, "card": true}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if f == "layout.go" || strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, l := range as.Lhs {
				if sel, ok := l.(*ast.SelectorExpr); ok && marks[sel.Sel.Name] {
					t.Errorf("%s: writes the mark %s directly; use the SpecOpt in layout.go", fset.Position(sel.Pos()), sel.Sel.Name)
				}
			}
			return true
		})
	}
}
