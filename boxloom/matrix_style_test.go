package boxloom

import (
	"encoding/json"
	"strings"
	"testing"
)

const matrixStyleData = `{"columns": ["東", "西", "北"],
  "columnGroups": [{"title": "東の拠点", "span": 2}, {"title": "西の拠点", "span": 1}],
  "rows": [
    {"label": "人手の不足", "detail": ["採用が遅れる", "退職が続く"],
     "cells": [{"status": "support", "text": "p.35"}, {"status": "none"}, {"status": "conditional"}]},
    {"label": "設備の古さ", "detail": "機械が古い",
     "cells": [{"status": "none", "text": "p.9"}, {"status": "danger", "text": "p.12"}, {"status": "support"}]}]}`

// 行の detail(文か文の列)と列の群(columnGroups)を読む。span の合計が列の数と
// 違えば読み込みが断り、check は matrix-column-groups を返す。
func TestMatrixDetailAndColumnGroupsRead(t *testing.T) {
	d, err := matrixFromIR(json.RawMessage(matrixStyleData))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.ColumnGroups) != 2 || d.ColumnGroups[0].Span != 2 || d.ColumnGroups[1].Title.Plain() != "西の拠点" {
		t.Errorf("columnGroups = %+v", d.ColumnGroups)
	}
	if len(d.Rows[0].Points) != 2 || d.Rows[0].Points[1].Plain() != "退職が続く" || len(d.Rows[0].Detail) != 0 {
		t.Errorf("row 0 detail = %+v / %+v", d.Rows[0].Detail, d.Rows[0].Points)
	}
	if d.Rows[1].Detail.Plain() != "機械が古い" || d.Rows[1].Points != nil {
		t.Errorf("row 1 detail = %+v / %+v", d.Rows[1].Detail, d.Rows[1].Points)
	}
	bad := strings.Replace(matrixStyleData, `"span": 1`, `"span": 2`, 1)
	if _, err := matrixFromIR(json.RawMessage(bad)); err == nil || !strings.Contains(err.Error(), "spans add up to 4 but there are 3 columns") {
		t.Errorf("err = %v", err)
	}
	errs, _ := CheckFile("../examples/ir/check-invalid/matrix-column-groups.json", "")
	if len(errs) != 1 || errs[0].Code != "matrix-column-groups" || errs[0].Path != "/slides/3/body/0/data/columnGroups" {
		t.Errorf("check = %+v", errs)
	}

	md := "---\ntitle: t\n---\n\n# s\n\n```matrix\ncolumns: [A, B]\ncolumnGroups:\n  - {title: G, span: 2}\nrows:\n" +
		"  - label: x\n    detail: [a, b]\n    cells: [{status: support}, {status: none}]\n```\n"
	deck, issues, err := ParseMarkdown("m.bx.md", []byte(md))
	if len(issues) != 0 || err != nil {
		t.Fatalf("parse: %+v %v", issues, err)
	}
	var data map[string]any
	if err := json.Unmarshal(deck.Slides[1].Body[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	g := data["columnGroups"].([]any)[0].(map[string]any)
	row := data["rows"].([]any)[0].(map[string]any)
	if g["span"] != float64(2) || g["title"] != "G" || len(row["detail"].([]any)) != 2 {
		t.Errorf("parsed data = %v", data)
	}
	_, issues, _ = ParseMarkdown("m.bx.md", []byte(strings.Replace(md, "span: 2", "span: two", 1)))
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "1以上の整数") {
		t.Errorf("issues = %+v", issues)
	}
}

// report は cellStyle mark:セルを塗らず、状態ごとに形の違う印(support は丸に白い
// チェック(2本の線)、conditional は三角、danger は ×、none は —。#179)。文は印の下に
// 小さい字。列の群の名前は span 列の幅で、下に罫線。light は filled のまま。
func TestMatrixMarkStyle(t *testing.T) {
	if v := figureFromIR(t, "light", "matrix", matrixStyleData).(*MatrixSubsystem).vals(); v.CellStyle != "filled" {
		t.Errorf("light cellStyle = %q", v.CellStyle)
	}
	sub := figureFromIR(t, "report", "matrix", matrixStyleData).(*MatrixSubsystem)
	if sub.vals().CellStyle != "mark" {
		t.Fatalf("report cellStyle = %q", sub.vals().CellStyle)
	}
	leaf := SubsystemNode(sub, WithGrow(1))
	p := figureSlide(t, "report", leaf, "matrix-mark-report")
	th := mustTemplate("report").Theme
	pl := sub.place(subsystemDrawBox(leaf))
	in := func(b LBox, x, y float64) bool { return x >= b.X && x <= b.X+b.Width && y >= b.Y && y <= b.Y+b.Height }
	type found struct {
		dots        []string
		checks      int
		texts       []string
		filledCells int
	}
	var cells [2][3]found
	for i := range p.Shapes {
		s := &p.Shapes[i]
		for r := range pl.cells {
			for c, b := range pl.cells[r] {
				cx, cy := s.box.X+s.box.Width/2, s.box.Y+s.box.Height/2
				if s.kind == skLine {
					cx, cy = (s.x1+s.x2)/2, (s.y1+s.y2)/2
				}
				if !in(b, cx, cy) {
					continue
				}
				f := &cells[r][c]
				switch {
				case s.kind == skLine:
					if s.lineColor == th.c("bg") {
						f.checks++
					}
				case s.fill != "" && matrixMarkPrst(s.prst):
					f.dots = append(f.dots, s.prst+":"+s.fill)
				case s.fill != "":
					f.filledCells++
				case s.text != "":
					f.texts = append(f.texts, s.text)
					if s.fontSize != sub.text().Small {
						t.Errorf("cell text %q size %v, want small %v", s.text, s.fontSize, sub.text().Small)
					}
				}
			}
		}
	}
	want := [2][3]struct {
		dot    string
		checks int
		text   string
	}{
		{{"support", 2, "p.35"}, {"none", 0, ""}, {"conditional", 0, ""}},
		{{"none", 0, "p.9"}, {"danger", 0, "p.12"}, {"support", 2, ""}},
	}
	for r := range want {
		for c, w := range want[r] {
			f := cells[r][c]
			if f.filledCells != 0 {
				t.Errorf("cell %d,%d has %d filled shapes", r, c, f.filledCells)
			}
			if len(f.dots) != 1 || f.dots[0] != matrixMarkShapes[w.dot]+":"+th.c(w.dot) {
				t.Errorf("cell %d,%d dots = %v, want %q", r, c, f.dots, w.dot)
			}
			if f.checks != w.checks {
				t.Errorf("cell %d,%d check lines = %d, want %d", r, c, f.checks, w.checks)
			}
			if w.text == "" && len(f.texts) != 0 || w.text != "" && (len(f.texts) != 1 || f.texts[0] != w.text) {
				t.Errorf("cell %d,%d texts = %v, want %q", r, c, f.texts, w.text)
			}
		}
	}
	// 列の群:東の拠点は2列(と間)の幅、西の拠点は1列。
	if len(pl.groups) != 2 || pl.groups[0].X != pl.cells[0][0].X || pl.groups[0].X+pl.groups[0].Width != pl.cells[0][1].X+pl.cells[0][1].Width ||
		pl.groups[1].Width != pl.cells[0][2].Width {
		t.Errorf("groups = %+v", pl.groups)
	}
	for _, s := range []string{"東の拠点", "西の拠点", "採用が遅れる", "機械が古い"} {
		ok := false
		for i := range p.Shapes {
			ok = ok || strings.Contains(p.Shapes[i].text, s)
		}
		if !ok {
			t.Errorf("no shape has %q", s)
		}
	}
	// 凡例は使った4つの状態とも(none も — の印を描く)。
	if n := len(sub.legend()); n != 4 {
		t.Errorf("legend = %+v, want 4 statuses", sub.legend())
	}
	if w := sub.Lint(subsystemDrawBox(leaf)); len(w) != 0 {
		t.Errorf("lint = %v", w)
	}
}

// matrixMarkPrst は状態の印の形か。
func matrixMarkPrst(prst string) bool {
	for _, p := range matrixMarkShapes {
		if p == prst {
			return true
		}
	}
	return false
}

// 状態の印は4つとも形が違う(色を抜いても読める、#179)。
func TestMatrixMarkShapesDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, st := range figureStatuses {
		p := matrixMarkShapes[st]
		if p == "" {
			t.Errorf("no mark shape for %s", st)
		}
		if o, ok := seen[p]; ok {
			t.Errorf("%s and %s share the mark shape %s", o, st, p)
		}
		seen[p] = st
	}
}

// cellStyle label:セルの頭に札(状態の色の枠・印・凡例の文)、その下に文。札の文は作者の
// legend(無ければ既定の文)で、テンプレートの marks からは取らない。札の幅は Measure の
// Min の幅に入り、狭い枠では Lint が札のはみ出しを知らせる。
func TestMatrixLabelStyle(t *testing.T) {
	data := strings.Replace(matrixStyleData, `"rows": [`, `"legend": {"support": "推す", "danger": "避ける"}, "rows": [`, 1)
	sub := figureFromIR(t, "light", "matrix", data).(*MatrixSubsystem)
	v := *sub.vals()
	v.CellStyle = "label"
	if err := v.check("matrix"); err != nil {
		t.Fatal(err)
	}
	sub.V = &v
	leaf := SubsystemNode(sub, WithGrow(1))
	p := figureSlide(t, "light", leaf, "matrix-label-light")
	th := mustTemplate("light").Theme
	pl := sub.place(subsystemDrawBox(leaf))
	in := func(b LBox, s *shape) bool {
		cx, cy := s.box.X+s.box.Width/2, s.box.Y+s.box.Height/2
		return cx >= b.X && cx <= b.X+b.Width && cy >= b.Y && cy <= b.Y+b.Height
	}
	want := [2][3]struct{ status, label, text string }{
		{{"support", "推す", "p.35"}, {"none", "非対応", ""}, {"conditional", "条件つき", ""}},
		{{"none", "非対応", "p.9"}, {"danger", "避ける", "p.12"}, {"support", "推す", ""}},
	}
	for r := range want {
		for c, w := range want[r] {
			b := pl.cells[r][c]
			var tags, marks int
			var texts []string
			for i := range p.Shapes {
				s := &p.Shapes[i]
				if s.kind == skLine || !in(b, s) {
					continue
				}
				switch {
				case s.hasOutline && s.outlineColor == th.c(w.status):
					tags++
				case s.fill == th.c(w.status) && s.prst == matrixMarkShapes[w.status]:
					marks++
				case s.text != "" && s.fill == "":
					texts = append(texts, s.text)
				}
			}
			if tags != 1 || marks != 1 {
				t.Errorf("cell %d,%d: tags %d marks %d, want 1 and 1", r, c, tags, marks)
			}
			wantTexts := []string{w.label}
			if w.text != "" {
				wantTexts = append(wantTexts, w.text)
			}
			if strings.Join(texts, "|") != strings.Join(wantTexts, "|") {
				t.Errorf("cell %d,%d texts = %q, want %q", r, c, texts, wantTexts)
			}
		}
	}
	if w := sub.Lint(subsystemDrawBox(leaf)); len(w) != 0 {
		t.Errorf("lint = %v", w)
	}
	// 札の幅は折り返さないので Min の幅に入る。
	tw, _ := sub.tagSize("conditional")
	if m := sub.widths(); m.colMin < tw+2*v.TextInsetX-0.01 {
		t.Errorf("colMin %.2f < tag %.2f + insets", m.colMin, tw)
	}
	// 狭い枠では札がはみ出したことを Lint が知らせる。
	if w := sub.Lint(LBox{Width: 200, Height: 400}); len(w) == 0 {
		t.Error("no lint for a cramped label matrix")
	}
	// 札は既定ではない:同梱のテンプレートはどれも label を選ばない。
	for _, name := range []string{"dark", "light", "report"} {
		if cs := figureFromIR(t, name, "matrix", matrixStyleData).(*MatrixSubsystem).vals().CellStyle; cs == "label" {
			t.Errorf("%s uses cellStyle label by default", name)
		}
	}
}
