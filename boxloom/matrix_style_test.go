package boxloom

import (
	"encoding/json"
	"strings"
	"testing"
)

const matrixStyleData = `{"columns": ["東", "西", "北"],
  "columnGroups": [{"title": "本州", "span": 2}, {"title": "北海道", "span": 1}],
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
	if len(d.ColumnGroups) != 2 || d.ColumnGroups[0].Span != 2 || d.ColumnGroups[1].Title.Plain() != "北海道" {
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

// report は cellStyle mark:セルを塗らず、support は丸(support の色)に白い
// チェック(2本の線)、none は何も描かない、その他は状態の色の丸。文は丸の下に
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
				case s.prst == "ellipse":
					f.dots = append(f.dots, s.fill)
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
		{{"support", 2, "p.35"}, {"", 0, ""}, {"conditional", 0, ""}},
		{{"", 0, "p.9"}, {"danger", 0, "p.12"}, {"support", 2, ""}},
	}
	for r := range want {
		for c, w := range want[r] {
			f := cells[r][c]
			if f.filledCells != 0 {
				t.Errorf("cell %d,%d has %d filled shapes", r, c, f.filledCells)
			}
			if w.dot == "" && len(f.dots) != 0 || w.dot != "" && (len(f.dots) != 1 || f.dots[0] != th.c(w.dot)) {
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
	// 列の群:本州は2列(と間)の幅、北海道は1列。
	if len(pl.groups) != 2 || pl.groups[0].X != pl.cells[0][0].X || pl.groups[0].X+pl.groups[0].Width != pl.cells[0][1].X+pl.cells[0][1].Width ||
		pl.groups[1].Width != pl.cells[0][2].Width {
		t.Errorf("groups = %+v", pl.groups)
	}
	for _, s := range []string{"本州", "北海道", "採用が遅れる", "機械が古い"} {
		ok := false
		for i := range p.Shapes {
			ok = ok || strings.Contains(p.Shapes[i].text, s)
		}
		if !ok {
			t.Errorf("no shape has %q", s)
		}
	}
	// 凡例に none は出ない(セルに何も描かないので)。
	for _, li := range sub.legend() {
		if li.Role == "none" {
			t.Error("mark legend lists none")
		}
	}
	if w := sub.Lint(subsystemDrawBox(leaf)); len(w) != 0 {
		t.Errorf("lint = %v", w)
	}
}
