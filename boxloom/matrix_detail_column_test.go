package boxloom

import (
	"encoding/json"
	"strings"
	"testing"
)

// matrixDetailData は行ごとに要約(detail)を持つ matrix(元の資料の p28 の形)。
// detailColumn を足すと要約が題の右の列に出る。
func matrixDetailData(detailColumn string) string {
	dc := ""
	if detailColumn != "" {
		dc = `"detailColumn": "` + detailColumn + `", `
	}
	return `{` + dc + `"columns": ["案 A", "案 B", "案 C"], "rows": [
  {"label": "費用", "detail": "初年度の持ち出しと、毎年かかる保守の費用を合わせて比べる",
   "cells": [{"status": "support", "text": "少"}, {"status": "conditional"}, {"status": "danger", "text": "多"}]},
  {"label": "止まる時間", "detail": ["夜間に切り替える", "週末の作業は避ける"],
   "cells": [{"status": "support"}, {"status": "support"}, {"status": "danger"}]},
  {"label": "保守の期限", "detail": "今の機器の保守が切れる年と、新しい機器の保守の年",
   "cells": [{"status": "danger"}, {"status": "support"}, {"status": "support"}]}]}`
}

// detailColumn があると要約は題の右の見出しつきの列に出て、同じ要約を題の下に
// 置くより Min の高さが下がる。評価の列は印と短い文の幅に留まり、文の列が幅を取る。
func TestMatrixDetailColumn(t *testing.T) {
	for _, tmpl := range []string{"report", "light"} {
		restore := useFonts(mustTemplate(tmpl).Fonts)
		under := figureFromIR(t, tmpl, "matrix", matrixDetailData("")).(*MatrixSubsystem)
		col := figureFromIR(t, tmpl, "matrix", matrixDetailData("要約")).(*MatrixSubsystem)
		if col.Data.DetailColumn.Plain() != "要約" || under.Data.DetailColumn != nil {
			t.Fatalf("%s: detailColumn = %v / %v", tmpl, col.Data.DetailColumn, under.Data.DetailColumn)
		}
		widths := []float64{460, 912} // report:列の中(元の p28)と本文の幅
		if tmpl == "light" {
			widths = []float64{912} // light の 16pt の字は 460pt に5列が入りきらない
		}
		for _, w := range widths {
			mu, mc := under.Measure(Constraint{Width: w}), col.Measure(Constraint{Width: w})
			t.Logf("%s width %v: Min height %.1f (detailColumn) vs %.1f (under the label)", tmpl, w, mc.Min.Height, mu.Min.Height)
			if !(mc.Min.Height < mu.Min.Height) {
				t.Errorf("%s width %v: Min height with detailColumn %.1f, under the label %.1f", tmpl, w, mc.Min.Height, mu.Min.Height)
			}
			g := col.geom(w)
			m := col.widths()
			if g.detW <= 0 || g.colW > m.colNat+noWrapSlack+0.01 {
				t.Errorf("%s width %v: detW %.1f colW %.1f (colNat %.1f)", tmpl, w, g.detW, g.colW, m.colNat)
			}
			h := mc.Natural.Height
			if warns := col.Lint(LBox{Width: w, Height: h}); len(warns) != 0 {
				t.Errorf("%s width %v: lint %v", tmpl, w, warns)
			}
			// 箱が重ならず、題 → 要約 → 評価の順に左から並ぶ。
			pl := col.place(LBox{Width: w, Height: h})
			for i := range col.Data.Rows {
				hd, dt, c0 := pl.heads[i], pl.dets[i], pl.cells[i][0]
				if !(hd.X+hd.Width < dt.X && dt.X+dt.Width < c0.X) {
					t.Errorf("%s width %v row %d: head %+v detail %+v cell %+v", tmpl, w, i, hd, dt, c0)
				}
			}
			if pl.detHead.X != pl.dets[0].X || pl.detHead.Y != pl.header[0].Y {
				t.Errorf("%s width %v: detail header %+v", tmpl, w, pl.detHead)
			}
		}
		// Min の幅で入る(列の中に置いても文が横にはみ出さない)。
		cw := col.columnMinWidth()
		if warns := col.Lint(LBox{Width: cw, Height: col.Measure(Constraint{Width: cw}).Max.Height}); len(warns) != 0 {
			t.Errorf("%s: lint at columnMinWidth %.1f: %v", tmpl, cw, warns)
		}
		restore()
	}

	// 描いた PPTX に見出しと要約の文が出る。
	sub := figureFromIR(t, "report", "matrix", matrixDetailData("要約"))
	p := figureSlide(t, "report", SubsystemNode(sub, WithGrow(1)), "matrix-detail-column")
	var b strings.Builder
	for _, sh := range p.Shapes {
		b.WriteString(">" + sh.text + "<")
		for _, para := range append([][]TextRun{sh.runs}, sh.paras...) {
			b.WriteString(">")
			for _, r := range para {
				b.WriteString(r.Text)
			}
			b.WriteString("<")
		}
	}
	xml := b.String()
	for _, s := range []string{">要約<", "初年度の持ち出し", "週末の作業は避ける"} {
		if !strings.Contains(xml, s) {
			t.Errorf("slide lacks %q", s)
		}
	}

	if _, err := matrixFromIR(json.RawMessage(`{"detailColumn": "", "columns": ["A"], "rows": [{"label": "x", "cells": [{"status": "none"}]}]}`)); err == nil ||
		!strings.Contains(err.Error(), "detailColumn: must not be empty") {
		t.Errorf("empty detailColumn: err = %v", err)
	}

	md := "---\ntitle: t\n---\n\n# s\n\n```matrix\ndetailColumn: 要約\ncolumns: [A, B]\nrows:\n" +
		"  - label: x\n    detail: a\n    cells: [{status: support}, {status: none}]\n```\n"
	deck, issues, err := ParseMarkdown("m.bx.md", []byte(md))
	if len(issues) != 0 || err != nil {
		t.Fatalf("parse: %+v %v", issues, err)
	}
	var data map[string]any
	if err := json.Unmarshal(deck.Slides[1].Body[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["detailColumn"] != "要約" {
		t.Errorf("parsed data = %v", data)
	}
}
