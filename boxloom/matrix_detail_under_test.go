package boxloom

import (
	"strings"
	"testing"
)

// 行の説明(detail)は行の題の下に置く(#94)。置き方は IR に書かず、サブシステムが決める
// (#158)。題と説明は1つの箱の段落で、評価の列はその右に並ぶ。
func TestMatrixDetailUnderLabel(t *testing.T) {
	const data = `{"columns": ["案 A", "案 B", "案 C"], "rows": [
  {"label": "費用", "detail": "初年度の持ち出しと、毎年かかる保守の費用を合わせて比べる",
   "cells": [{"status": "support", "text": "少"}, {"status": "conditional"}, {"status": "danger", "text": "多"}]},
  {"label": "止まる時間", "detail": ["夜間に切り替える", "週末の作業は避ける"],
   "cells": [{"status": "support"}, {"status": "support"}, {"status": "danger"}]},
  {"label": "保守の期限", "cells": [{"status": "danger"}, {"status": "support"}, {"status": "support"}]}]}`
	for _, tmpl := range []string{"report", "light"} {
		restore := useFonts(mustTemplate(tmpl).Fonts)
		s := figureFromIR(t, tmpl, "matrix", data).(*MatrixSubsystem)
		v, tx := s.vals(), s.text()
		w := 912.0
		h := s.Measure(Constraint{Width: w}).Natural.Height
		if warns := s.Lint(LBox{Width: w, Height: h}); len(warns) != 0 {
			t.Errorf("%s: lint %v", tmpl, warns)
		}
		pl := s.place(LBox{Width: w, Height: h})
		for i := range s.Data.Rows {
			r := &s.Data.Rows[i]
			hd, c0 := pl.heads[i], pl.cells[i][0]
			if !(hd.X+hd.Width < c0.X) {
				t.Errorf("%s row %d: head %+v cell %+v", tmpl, i, hd, c0)
			}
			// 題の箱の高さは題と説明の両方が入る高さ。
			label, _ := s.rowHeadParas(r, "", "", nil)
			_, lh := runsBox(label, tx.RowTitle, hd.Width-2*v.TextInsetX)
			_, need := s.rowHeadNeed(r, hd.Width-2*v.TextInsetX)
			if len(r.Detail) > 0 || len(r.Points) > 0 {
				if !(need > lh) {
					t.Errorf("%s row %d: need %.1f does not include the detail (label %.1f)", tmpl, i, need, lh)
				}
			}
			if hd.Height < need+2*v.TextInsetY-0.01 {
				t.Errorf("%s row %d: head height %.1f < %.1f", tmpl, i, hd.Height, need+2*v.TextInsetY)
			}
		}
		restore()
	}

	// 描いた PPTX では題の箱が題の段落に続けて説明の段落を持つ。
	sub := figureFromIR(t, "report", "matrix", data)
	p := figureSlide(t, "report", SubsystemNode(sub, WithGrow(1)), "matrix-detail-under")
	found := 0
	for _, sh := range p.Shapes {
		if len(sh.runs) == 0 || len(sh.paras) == 0 {
			continue
		}
		var head, rest strings.Builder
		for _, r := range sh.runs {
			head.WriteString(r.Text)
		}
		for _, para := range sh.paras {
			for _, r := range para {
				rest.WriteString(r.Text)
			}
		}
		switch head.String() {
		case "費用":
			if strings.Contains(rest.String(), "初年度の持ち出し") {
				found++
			}
		case "止まる時間":
			if strings.Contains(rest.String(), "夜間に切り替える") && strings.Contains(rest.String(), "週末の作業は避ける") {
				found++
			}
		}
	}
	if found != 2 {
		t.Errorf("found %d row titles with their detail below, want 2", found)
	}
}
