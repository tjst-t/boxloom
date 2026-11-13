package boxloom

import (
	"fmt"
	"strings"
	"testing"
)

// 帯の上の文の色と帯の地の色のコントラスト比(Backlog #97)。例の文は架空。

var ganttBarStatuses = []string{"support", "conditional", "none", "danger"}

// ganttBarContrastDeck は4つの状態の帯(塗った帯と仮の帯)を1行ずつ持つ deck。
func ganttBarContrastDeck() string {
	var b strings.Builder
	b.WriteString("---\ntitle: 架空の計画\ncover: false\n---\n\n# 帯の色\n\n```gantt\nspan: 2026-01 .. 2027-01\ngroups:\n  - id: g1\n    label: [作業]\n    rows:\n")
	for _, st := range ganttBarStatuses {
		for _, tent := range []bool{false, true} {
			fmt.Fprintf(&b, "      - label: %s\n        segments:\n          - {from: 2026-01, to: 2026-12, status: %s, label: %s, tentative: %v}\n", st, st, barLabel(st, tent), tent)
		}
	}
	b.WriteString("```\n")
	return b.String()
}

func barLabel(status string, tentative bool) string {
	if tentative {
		return "仮の" + status
	}
	return "帯の" + status
}

// 3つのテンプレートのすべての状態で、描いた帯の文の色と帯の見えている地の色の
// コントラスト比が 4.5 以上。山形の薄い面は bg に重ねた色、仮の帯(塗り無し)は bg。
func TestGanttBarLabelContrast(t *testing.T) {
	src := ganttBarContrastDeck()
	for _, name := range []string{"dark", "light", "report"} {
		tm, _, _, shapes := buildGanttSlide(t, name, src)
		bg := tm.Theme.Color("bg")
		texts := textsOf(shapes)
		for _, st := range ganttBarStatuses {
			for _, tent := range []bool{false, true} {
				label := barLabel(st, tent)
				ts := texts[label]
				if len(ts) != 1 {
					t.Fatalf("%s: label %q drawn %d times", name, label, len(ts))
				}
				lt := ts[0]
				bar := barUnder(shapes, lt.box)
				if bar == nil {
					t.Fatalf("%s: no bar under %q", name, label)
				}
				fill := bar.fill
				switch {
				case fill == "":
					fill = bg
				case bar.fillAlpha > 0 && bar.fillAlpha < 1:
					fill = mixHex(fill, bg, 1-bar.fillAlpha)
				}
				if cr := contrastRatio(fill, lt.color); cr < minTextContrast {
					t.Errorf("%s %s: text %s on %s: contrast %.2f < %.1f", name, label, lt.color, fill, cr, minTextContrast)
				}
			}
		}
	}
}

// barUnder は文の枠 lb の中心を含む、文を持たない帯の図形。
func barUnder(shapes []shape, lb LBox) *shape {
	cx, cy := lb.X+lb.Width/2, lb.Y+lb.Height/2
	for i := range shapes {
		s := &shapes[i]
		if s.kind == skText || s.text != "" || (s.fill == "" && !s.hasOutline) {
			continue
		}
		if cx >= s.box.X && cx <= s.box.X+s.box.Width && cy >= s.box.Y && cy <= s.box.Y+s.box.Height && s.box.Height < 60 {
			return s
		}
	}
	return nil
}

// 塗った帯(角の丸い矩形)を描く dark・light の4つの状態の文が、規則だけで読める。
// report は山形で描く(地は薄い)ので TestGanttBarLabelContrast が確かめる。report の
// 色で不透明な帯を描くと conditional・none はどの役割でも 4.5 に届かない。
func TestGanttBarLabelRoleRect(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		gv := *defaultGanttValues()
		gv.BarShape = "rect"
		for _, st := range ganttBarStatuses {
			for _, tent := range []bool{false, true} {
				role := ganttBarLabelRole(&gv, GanttSegment{Role: st, Tentative: tent}, tm.Theme)
				fill := tm.Theme.Color(st)
				if tent {
					fill = tm.Theme.Color("bg")
				}
				if cr := contrastRatio(fill, tm.Theme.Color(role)); cr < minTextContrast {
					t.Errorf("%s %s tentative=%v: role %s contrast %.2f", name, st, tent, role, cr)
				}
			}
		}
	}
}

// 今読めている組み合わせは変えない:light の support・none は text_primary、
// conditional は on_conditional のまま。
func TestGanttBarLabelRoleKeepsReadable(t *testing.T) {
	tm, err := LookupTemplate("light")
	if err != nil {
		t.Fatal(err)
	}
	gv := defaultGanttValues()
	for st, want := range map[string]string{"support": "text_primary", "none": "text_primary", "conditional": "on_conditional"} {
		if got := ganttBarLabelRole(gv, GanttSegment{Role: st}, tm.Theme); got != want {
			t.Errorf("light %s: %s, want %s", st, got, want)
		}
	}
}
