package boxloom

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// denseRowsDeck は架空の図書館の本文のスライド(表・対の並び・matrix)。density は
// 呼ぶ側が {…} に入れる。
func denseRowsDeck(attr string) string {
	return "---\ntitle: 架空の図書館\n---\n\n# 架空の返却 " + attr + "\n\n" +
		"| 曜日 | 返却 |\n|---|---|\n| 月 | 30 |\n| 火 | 28 |\n\n" +
		":::::pairs 分かったこと | 打ち手\n:::subject\n夜に返却が集まる\n:::\n:::response\n箱を増やす\n:::\n" +
		":::subject\n雨で本が濡れる\n:::\n:::response\n庇を付ける\n:::\n:::::\n\n" +
		"```matrix\ncolumns: [箱, 窓口]\nrows:\n  - label: 架空の行の題\n    cells:\n" +
		"      - {status: support, text: 速い}\n      - {status: conditional, text: 遅い}\n```\n"
}

// denseRows は組んだスライドの行ごとの余白の値。
type denseRows struct {
	padX, padY      float64 // 表のセルの余白
	pairsGap        float64 // 対の行の間
	pairMark        float64 // 対の印の一辺(印を描くテンプレートだけ。無ければ 0)
	matrixRowTitle  float64 // matrix の行の題の字
	tables, matrixs int
}

func measureDenseRows(t *testing.T, tm *Template, attr string) denseRows {
	t.Helper()
	d, issues, err := ParseMarkdown("x.bx.md", []byte(denseRowsDeck(attr)))
	if err != nil || len(issues) > 0 {
		t.Fatalf("%v %+v", err, issues)
	}
	b, err := tm.BuildSlide(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	var z denseRows
	var walk func(s *Spec)
	walk = func(s *Spec) {
		if s.pairs != nil {
			z.pairsGap = s.pairs.list.Gap
			if row := s.pairs.rows[0].row; len(row.Children) > 2 && row.Children[0].Width != nil {
				z.pairMark = *row.Children[0].Width
			}
		}
		switch x := s.Subsystem.(type) {
		case *tableLeaf:
			z.padX, z.padY = x.padX, x.padY
			z.tables++
		case *MatrixSubsystem:
			z.matrixRowTitle = x.text().RowTitle
			z.matrixs++
		}
		for _, c := range s.Children {
			walk(c)
		}
	}
	walk(b.Body)
	if z.tables != 1 || z.matrixs != 1 || z.pairsGap == 0 {
		t.Fatalf("%s %q: did not find the table, pairs and matrix: %+v", tm.Name, attr, z)
	}
	return z
}

// dense のスライドでは表のセルの余白・対の行の間と印・matrix の行の題の字が値の表の
// dense の組になり、normal では values のまま(#136)。
func TestDenseRowSpacing(t *testing.T) {
	for _, name := range []string{"dark", "light", "report"} {
		tm := mustTemplate(name)
		v, dv := tm.Values, tm.Dense
		normal := measureDenseRows(t, tm, "")
		dense := measureDenseRows(t, tm, "{density=dense}")
		for _, w := range []struct {
			what      string
			got, want float64
		}{
			{"normal padX", normal.padX, v.TableCellPadX}, {"normal padY", normal.padY, v.TableCellPadY},
			{"normal pairsRowGap", normal.pairsGap, v.PairsRowGap}, {"normal matrixRowTitle", normal.matrixRowTitle, v.MatrixRowTitle},
			{"dense padX", dense.padX, dv.TableCellPadX}, {"dense padY", dense.padY, dv.TableCellPadY},
			{"dense pairsRowGap", dense.pairsGap, dv.PairsRowGap}, {"dense matrixRowTitle", dense.matrixRowTitle, dv.MatrixRowTitle},
		} {
			if w.got != w.want {
				t.Errorf("%s: %s = %v, want %v", name, w.what, w.got, w.want)
			}
		}
		if name == "report" {
			if normal.pairMark != v.PairMarkSize || dense.pairMark != dv.PairMarkSize {
				t.Errorf("report: pair mark normal %v dense %v, want %v %v", normal.pairMark, dense.pairMark, v.PairMarkSize, dv.PairMarkSize)
			}
			// report の dense はこれらの余白・字が normal より小さい(元の資料の密なページ)。
			for _, c := range []struct {
				what          string
				dense, normal float64
			}{
				{"tableCellPaddingX", dense.padX, normal.padX}, {"tableCellPaddingY", dense.padY, normal.padY},
				{"pairsRowGap", dense.pairsGap, normal.pairsGap}, {"pairMarkSize", dense.pairMark, normal.pairMark},
				{"matrixRowTitle", dense.matrixRowTitle, normal.matrixRowTitle},
			} {
				if !(c.dense < c.normal) {
					t.Errorf("report: dense %s %v is not smaller than normal %v", c.what, c.dense, c.normal)
				}
			}
		}
		// normal の値は従来の組み方と同じ(normal の出力を変えない):対の行の間は
		// dark・light が blockGap・report が 4pt、matrix の行の題は本文の字。
		wantGap := v.BlockGap
		if name == "report" {
			wantGap = 4
		}
		if v.PairsRowGap != wantGap || v.MatrixRowTitle != v.Body {
			t.Errorf("%s: values pairsRowGap %v matrixRowTitle %v, want %v and body %v", name, v.PairsRowGap, v.MatrixRowTitle, wantGap, v.Body)
		}
	}
}

// matrix の行の題は描いた PPTX でも matrixRowTitle の字(report の dense は 11pt)。
func TestDenseMatrixRowTitleRendered(t *testing.T) {
	tm := mustTemplate("report")
	for _, c := range []struct {
		attr string
		size float64
	}{{"", tm.Values.MatrixRowTitle}, {"{density=dense}", tm.Dense.MatrixRowTitle}} {
		d, _, err := ParseMarkdown("x.bx.md", []byte(denseRowsDeck(c.attr)))
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "m.pptx")
		if _, err := renderDeckToFile(tm, d, out); err != nil {
			t.Fatal(err)
		}
		slide := slideXML(t, out, "ppt/slides/slide2.xml")
		i := strings.Index(slide, "<a:t>架空の行の題</a:t>")
		if i < 0 {
			t.Fatalf("%q: row title not found", c.attr)
		}
		run := slide[strings.LastIndex(slide[:i], "<a:rPr"):i]
		if want := fmt.Sprintf(`sz="%d"`, int(c.size*100)); !strings.Contains(run, want) {
			t.Errorf("%q: row title run %s, want %s", c.attr, run, want)
		}
	}
}

// 値の表の dense の新しい名前:values より大きければ誤り、表のセルの上下の余白だけは 0 でもよい。
func TestDenseRowValuesChecked(t *testing.T) {
	for _, c := range []struct {
		name string
		key  string
		val  float64
		want string
	}{
		{"pad larger", "tableCellPaddingX", 12, "dense.tableCellPaddingX: 12 is larger than values.tableCellPaddingX"},
		{"gap larger", "pairsRowGap", 24, "dense.pairsRowGap: 24 is larger than values.pairsRowGap"},
		{"title off scale", "matrixRowTitle", 13, "dense.matrixRowTitle: 13 is not on the type scale"},
		{"padX zero", "tableCellPaddingX", 0, "dense.tableCellPaddingX: must be greater than 0"},
		{"padY zero", "tableCellPaddingY", 0, ""},
	} {
		_, err := parseTemplateFile("light", mutate(t, func(m map[string]any) { m["dense"].(map[string]any)[c.key] = c.val }))
		if c.want == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}
