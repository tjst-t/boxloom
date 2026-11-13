package boxloom

import "testing"

// 対の側の中の表と引用(Backlog #101)の試験。架空の対の応答の側に表と引用2つを
// 置き、本文が足りないときも側の中のブロックは縮めずに積む(足りない分は
// layout-overflow で知らせる)ことと、引用の人の印が文と重ならないことを確かめる。

const crushPairsMD = `---
title: T
---

# 窓口の待ち時間を減らす

:::::pairs 分かったこと | 打ち手と声
:::subject 夕方に混む
- 来訪の半分は 17 時から 19 時に集まる
:::
::::response 打ち手
| 時間帯 | 窓口の数 | 待ち時間 |
|---|---|---|
| 9-12 時 | 2 | 5 分 |
| 12-17 時 | 2 | 8 分 |
| 17-19 時 | 4 | 12 分 |
| 19-21 時 | 3 | 7 分 |

:::quote 利用者 A、聞き取りにて
夕方に来ると、いつも長く待たされる。仕事の帰りにしか来られないので困っている
:::
:::quote 窓口担当 B、聞き取りにて
夕方だけ人を増やせば、待ち時間は半分にできると思う
:::
:::quote 窓口担当 C、聞き取りにて
夕方の受付を予約にすれば、待つ人の数そのものを減らせる
:::
::::
:::subject 窓口が遠い
- 駅から 20 分かかる
- バスは 1 時間に 1 本しか無い
- 坂の上にあり、歩いて来る人には遠い
:::
:::response
- 駅前に出張の窓口を置く
:::
:::::
`

func layoutBoxesOverlap(a, b LayoutBox) bool {
	const eps = 0.01
	return a.X < b.X+b.Width-eps && b.X < a.X+a.Width-eps && a.Y < b.Y+b.Height-eps && b.Y < a.Y+a.Height-eps
}

// leaves は n の下の葉(子の無い箱)を集める。
func layoutLeaves(n *LayoutNode, out []*LayoutNode) []*LayoutNode {
	if len(n.Children) == 0 {
		return append(out, n)
	}
	for _, c := range n.Children {
		out = layoutLeaves(c, out)
	}
	return out
}

func TestPairsSideNoCrush(t *testing.T) {
	for _, tmpl := range []string{"report", "dark", "light"} {
		data := []byte(crushPairsMD)
		_, warns, _ := checkCaps(t, data, tmpl)
		overflow := false
		for _, w := range warns {
			if w.Code == "subsystem-min" {
				t.Errorf("%s: %s", tmpl, w.Message)
			}
			if w.Code == "layout-overflow" {
				overflow = true
			}
		}
		if !overflow {
			t.Errorf("%s: the deck should overflow (the test needs a short body): %+v", tmpl, warns)
		}
		dump, err := Layout("p.bx.md", data, BuildOptions{Template: tmpl})
		if err != nil {
			t.Fatal(err)
		}
		seen := pairsLayoutNodes(dump)
		side := "/slides/1/body/0/pairs/0/response"
		table := seen[side+"/body/0"]
		if table == nil || table.Subsystem == nil || table.Subsystem.Name != "table" {
			t.Fatalf("%s: no table at %s/body/0: %+v", tmpl, side, table)
		}
		if !nearPt(table.Box.Height, table.Subsystem.Natural.Height) {
			t.Errorf("%s: table height %.2f, natural %.2f", tmpl, table.Box.Height, table.Subsystem.Natural.Height)
		}
		// 側の中のブロックは重ならずに上から積む。
		prev := table.Box.Y + table.Box.Height
		for i := 1; i <= 2; i++ {
			q := seen[side+"/body/"+string(rune('0'+i))]
			if q == nil {
				t.Fatalf("%s: no quote %d", tmpl, i)
			}
			if q.Box.Y < prev-0.01 {
				t.Errorf("%s: quote %d starts at %.2f above the previous block's bottom %.2f", tmpl, i, q.Box.Y, prev)
			}
			prev = q.Box.Y + q.Box.Height
			// 印(report は人の印・dark と light は引用符)の描く矩形は、文の矩形と重ならない。
			var marks, texts []LayoutBox
			for _, l := range layoutLeaves(q, nil) {
				switch {
				case l.Subsystem != nil && l.Subsystem.Name == "callout":
					marks = append(marks, l.Subsystem.Draw)
				case l.Kind == "text":
					texts = append(texts, l.Box)
				}
			}
			if tmpl == "report" && len(marks) != 2 {
				t.Fatalf("%s: quote %d has %d callout marks", tmpl, i, len(marks))
			}
			if tmpl != "report" {
				// 引用符の文字の葉が印で、残りが文。
				if len(texts) < 2 {
					t.Fatalf("%s: quote %d has %d texts", tmpl, i, len(texts))
				}
				marks, texts = texts[:1], texts[1:]
			}
			for _, m := range marks {
				for _, x := range texts {
					if layoutBoxesOverlap(m, x) {
						t.Errorf("%s: quote %d mark %+v overlaps text %+v", tmpl, i, m, x)
					}
				}
			}
		}
	}
}
