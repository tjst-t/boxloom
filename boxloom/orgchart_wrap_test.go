package boxloom

import (
	"strings"
	"testing"
)

// orphanIn は幅 w の箱で文を折り返したとき、最後の行が泣き別れならその行を返す。
func orphanIn(in Inline, size, w float64, fv figureBoxValues) string {
	lines := wrapRuns(itemRuns(in, "", nil), size, w-2*fv.insetX)
	if len(lines) < 2 {
		return ""
	}
	last := strings.TrimSpace(runsText(lines[len(lines)-1]))
	if len([]rune(strings.ReplaceAll(last, " ", ""))) <= orphanRunes {
		return last
	}
	return ""
}

// 組織図の箱で、文の最後の1〜2文字だけが次の行に落ちる(「欧州 AI 委員 / 会」)
// ときは、列の両側の余りの中で箱を広げて避ける。箱どうしは重ならない。
func TestOrgChartUnitAvoidsOrphanLine(t *testing.T) {
	data := `{"root": {"title": "AI Office(欧州委員会)", "detail": "汎用 AI モデルの監督と各国の調整(第 64 条)", "emphasis": true},
	 "units": [
	  {"title": "欧州 AI 委員会", "detail": "加盟国の代表(第 65・66 条)"},
	  {"title": "諮問フォーラム", "detail": "産業・学界・市民社会(第 67 条)"},
	  {"title": "科学パネル", "detail": "独立の専門家。系統的リスクを警告(第 68 条)"},
	  {"title": "各国の所管当局", "detail": "加盟国が指定(第 70 条)", "status": "conditional",
	   "members": [{"title": "通知当局", "detail": "適合性評価機関の指定"}, {"title": "市場監視当局", "detail": "高リスク AI の監督"}]}],
	 "legend": {"conditional": "2026 年 8 月 2 日から監督を担う"}}`
	for _, tmpl := range []string{"light", "dark"} {
		tm := mustTemplate(tmpl)
		restore := useFonts(tm.Fonts)
		sub := figureFromIR(t, tmpl, "orgchart", data).(*OrgChartSubsystem)
		tx := sub.text()
		fv, mv := sub.boxValues("rect"), sub.boxValues("roundRect")
		for _, width := range []float64{912, 780} {
			g := sub.geom(width)
			for i := range sub.Data.Units {
				u := &sub.Data.Units[i]
				if last := orphanIn(u.Title, tx.Title, g.unitW[i], fv); last != "" {
					t.Errorf("%s %.0f: unit %q leaves %q alone on the last line (box %.1fpt)", tmpl, width, u.Title.Plain(), last, g.unitW[i])
				}
				// 3段目:780pt では列の余りが箱ごとに約 3pt しかなく直せない(広げられなければ
				// もとの幅のまま)。本文の幅いっぱい(912pt)では直る。
				if width < 912 {
					if g.memberWs[i] != g.memberW {
						t.Errorf("%s %.0f: members of unit %d widened to %.1f without enough room", tmpl, width, i, g.memberWs[i])
					}
					continue
				}
				for j := range u.Members {
					m := &u.Members[j]
					for _, p := range []struct {
						in   Inline
						size float64
					}{{m.Title, tx.Sub}, {m.Detail, tx.Detail}} {
						if last := orphanIn(p.in, p.size, g.memberWs[i], mv); last != "" {
							t.Errorf("%s %.0f: member %q leaves %q alone on the last line (box %.1fpt)", tmpl, width, p.in.Plain(), last, g.memberWs[i])
						}
					}
				}
			}
			// 同じ段の箱どうしは重ならず、2段目は unitGap、3段目は memberGap 以上離れる。
			pl := sub.place(LBox{Width: width, Height: 400})
			var mem []LBox
			for i := range pl.units {
				if i > 0 {
					if gap := pl.units[i].X - (pl.units[i-1].X + pl.units[i-1].Width); gap < sub.vals().UnitGap-0.01 {
						t.Errorf("%s %.0f: units %d and %d are %.1fpt apart, want >= %.0f", tmpl, width, i-1, i, gap, sub.vals().UnitGap)
					}
				}
				mem = append(mem, pl.members[i]...)
			}
			for k := 1; k < len(mem); k++ {
				if gap := mem[k].X - (mem[k-1].X + mem[k-1].Width); gap < sub.vals().MemberGap-0.01 {
					t.Errorf("%s %.0f: member boxes %d and %d are %.1fpt apart", tmpl, width, k-1, k, gap)
				}
			}
			if last := mem[len(mem)-1]; last.X+last.Width > width-sub.vals().Padding+0.01 {
				t.Errorf("%s %.0f: last member box ends at %.1f, past the frame", tmpl, width, last.X+last.Width)
			}
			if lint := sub.Lint(LBox{Width: width, Height: 400}); len(lint) > 0 {
				t.Errorf("%s %.0f: unexpected warnings %v", tmpl, width, lint)
			}
		}
		restore()
	}
}

// 泣き別れの数え方:2行以上に折り返して最後の行が2文字以下なら1つ。広げても
// 直らなければ元の幅のまま。
func TestNodeComfortWidth(t *testing.T) {
	defer useFonts(mustTemplate("light").Fonts)()
	fv := figureBoxValues{insetX: 8, insetY: 4}
	n := &FigureNode{Title: Inline{{Text: "欧州 AI 委員会"}}}
	// 1行に収まる幅の少し手前で、最後の1文字が落ちる幅を探す。
	natural := nodeNaturalWidth(n, 14, 12, fv)
	w := natural - 6
	if nodeOrphans(n, 14, 12, w, fv) != 1 {
		t.Fatalf("expected an orphan at %.1fpt (natural %.1f)", w, natural)
	}
	if got := nodeComfortWidth(n, 14, 12, w, w+20, fv); nodeOrphans(n, 14, 12, got, fv) != 0 || got > natural+1 {
		t.Errorf("comfort width %.1f should remove the orphan and stay near the natural width %.1f", got, natural)
	}
	if got := nodeComfortWidth(n, 14, 12, w, w+2, fv); got != w {
		t.Errorf("when widening cannot help, keep the width (got %.1f, want %.1f)", got, w)
	}
	short := &FigureNode{Title: Inline{{Text: "科学パネル"}}}
	if nodeOrphans(short, 14, 12, 200, fv) != 0 {
		t.Errorf("a one-line title has no orphan")
	}
}
