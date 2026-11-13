package boxloom

import (
	"strings"
	"testing"
)

// matrixShortHeadData は説明の列を持ち、評価の列の見出しが全角3字・中身が印と短い文
// (「p.35」)だけの matrix(元の資料の p28・29 の形を架空の内容にしたもの、#151)。
func matrixShortHeadData(detailColumn bool) string {
	dc := ""
	if detailColumn {
		dc = `"detailColumn": "要約", `
	}
	cells := `[{"status": "support", "text": "p.35"}, {"status": "conditional"}, {"status": "danger", "text": "p.12"}, {"status": "support"}, {"status": "none"}, {"status": "support", "text": "p.8"}]`
	return `{` + dc + `"columns": ["実現性", "費用感", "即効性", "拡張性", "安全性", "運用性"], "rows": [
  {"label": "初期の費用と毎年の費用", "detail": "初年度に持ち出す費用と、毎年かかる保守と運用の費用を合わせて五年で比べる。人件費も含める", "cells": ` + cells + `},
  {"label": "切り替えの手間", "detail": "夜間に切り替えて日中の業務を止めない。週末の作業はできるだけ避ける", "cells": ` + cells + `},
  {"label": "保守の期限", "detail": "今の機器の保守が切れる年と、新しい仕組みの保守が続く年を並べ、空白の期間が無いかを確かめる", "cells": ` + cells + `}]}`
}

// 説明の列があるとき、評価の列は中身(印と短い文)の幅まで縮み、列の見出しは折り返す
// (#151)。Min の幅は評価の列の最小で測り、見出しの行の高さは折り返した行の数で測る。
func TestMatrixHeaderWrapsInNarrowColumns(t *testing.T) {
	restore := useFonts(mustTemplate("report").Fonts)
	defer restore()
	s := figureFromIR(t, "report", "matrix", matrixShortHeadData(true)).(*MatrixSubsystem)
	v, tx := s.vals(), s.text()
	m := s.widths()
	C := len(s.Data.Columns)
	for _, w := range []float64{555, 700, 912} {
		g := s.geom(w)
		// 評価の列は印の幅程度(30pt 前後)。
		if g.colW > 40 || g.colW < m.colMin-0.01 {
			t.Errorf("width %v: evaluation column %.1f (min %.1f), want about 30pt", w, g.colW, m.colMin)
		}
		// 見出しは折り返し(2〜3行)、見出しの行の高さは折り返した行の数で測る。
		lines := (g.headerNeed - 2*v.TextInsetY) / (tx.Title * lineHeightMult())
		if lines < 1.99 || lines > float64(matrixHeaderMaxLines)+0.01 {
			t.Errorf("width %v: header %.1f lines", w, lines)
		}
		// 残りを題と要約の列が取る。
		if !(g.detW > 4*g.colW && g.headW >= m.headFloor-0.01) {
			t.Errorf("width %v: head %.1f detail %.1f col %.1f", w, g.headW, g.detW, g.colW)
		}
		mc := s.Measure(Constraint{Width: w})
		if warns := s.Lint(LBox{Width: w, Height: mc.Min.Height}); len(warns) != 0 {
			t.Errorf("width %v: lint at Min height %v", w, warns)
		}
	}
	// Measure の Min の幅は評価の列の最小で測る。その幅でも評価の列は最小を保つ。
	min := s.Measure(Constraint{Width: 912}).Min.Width
	if want := m.minWidth(v, C); min < want-0.01 {
		t.Errorf("Min width %.1f < %.1f", min, want)
	}
	if g := s.geom(min); g.colW < m.colMin-0.01 {
		t.Errorf("at Min width %.1f: evaluation column %.1f < its minimum %.1f", min, g.colW, m.colMin)
	}

	// 見出しが長すぎて3行に入らないときだけ、見出しの3行の幅が最小を決める。
	long := strings.Replace(matrixShortHeadData(true), `"実現性"`, `"実現までの期間と難しさ"`, 1)
	l := figureFromIR(t, "report", "matrix", long).(*MatrixSubsystem)
	lm := l.widths()
	if !(lm.colMin > m.colMin+1) {
		t.Errorf("long header: column min %.1f, short %.1f", lm.colMin, m.colMin)
	}
	g := l.geom(912)
	if _, h := runsBox(l.headerRuns(0, "", nil), tx.Title, g.colW-2*l.colInsetX()); h > float64(matrixHeaderMaxLines)*tx.Title*lineHeightMult()+0.01 {
		t.Errorf("long header wraps to %.1f pt at column %.1f", h, g.colW)
	}
}
