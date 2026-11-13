package boxloom

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// #133: 最小(Min)を割った図・表が枠の外に描かれ、次のブロックに重なっていた。
// 列(Col)の子は縮めずに要る高さ(requiredHeight)を割らず、足りない分は次の
// ブロックを下へ押し出す。押し出して本文の箱を越えた分は layout-overflow の測り
// (measureBody)と一致する。

// inkBox は spec を描いた図形の外接矩形(描かなければ ok が false)。
func inkBox(tm *Template, s *Spec) (LBox, bool) {
	p := NewPPTXBuilder(tm.Values.PageW, tm.Values.PageH)
	renderSpec(RenderContext{Out: p, Roles: newCheckedRoles(tm.Theme)}, s)
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(ax, ay, bx, by float64) {
		x0, y0 = math.Min(x0, math.Min(ax, bx)), math.Min(y0, math.Min(ay, by))
		x1, y1 = math.Max(x1, math.Max(ax, bx)), math.Max(y1, math.Max(ay, by))
	}
	for _, sh := range p.Shapes {
		switch sh.kind {
		case skLine, skConnector:
			add(sh.x1, sh.y1, sh.x2, sh.y2)
		default:
			if sh.box.Width <= 0 && sh.box.Height <= 0 {
				continue
			}
			add(sh.box.X, sh.box.Y, sh.box.X+sh.box.Width, sh.box.Y+sh.box.Height)
		}
	}
	if math.IsInf(x0, 1) {
		return LBox{}, false
	}
	return LBox{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}, true
}

// stackOverlaps は列 col の子を上から見て、描いた図形が次の子の枠に入り込む
// ものを返す。並列のまとまりの中の列も見る。
func stackOverlaps(tm *Template, col *Spec, where string) []string {
	var out []string
	for i, c := range col.Children {
		if c.columns != nil {
			for ci, info := range c.columns.cols {
				out = append(out, stackOverlaps(tm, info.body, where+"/columns["+itoa(ci)+"]")...)
			}
		}
		if i+1 >= len(col.Children) {
			continue
		}
		next := col.Children[i+1]
		ink, ok := inkBox(tm, c)
		if !ok {
			continue
		}
		if bottom := ink.Y + ink.Height; bottom > next.Box.Y+overflowEps {
			out = append(out, where+"/body["+itoa(i)+"] is drawn down to "+ftoa(bottom)+", into body["+itoa(i+1)+"] at "+ftoa(next.Box.Y))
		}
	}
	return out
}

func ftoa(v float64) string { return fmt.Sprintf("%.1f", v) }

// buildLaidOut は文書を tm で組み、スライドごとに並べた木を返す。
func buildLaidOut(t *testing.T, tm *Template, f string, data []byte) []*BuiltSlide {
	t.Helper()
	d, err := ReadDeck(f, data)
	if err != nil {
		t.Fatalf("%s: %v", f, err)
	}
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatalf("%s@%s: %v", f, tm.Name, err)
	}
	for _, b := range built {
		computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	}
	return built
}

// checkPushedDown は本文のブロックが並べたとおりの位置で、measureBody の
// Top・Bottom と一致し(本文の箱の内側の上端から)、本文の箱の内側の高さが
// 本文の領域(capacity)と一致することを確かめる。はみ出したスライドの数を返す。
func checkPushedDown(t *testing.T, name string, b *BuiltSlide) int {
	t.Helper()
	if b.Body == nil {
		return 0
	}
	top := b.Body.Box.Y + b.Body.padY()
	inner := b.Body.Box.Height - 2*b.Body.padY()
	type span struct{ y0, y1 float64 }
	var drawn []span
	for _, c := range b.Body.Children {
		drawn = append(drawn, span{c.Box.Y - top, c.Box.Y + c.Box.Height - top})
	}
	blocks, capacity, _, ok := measureBody(b.Root, b.Body)
	if !ok {
		return 0
	}
	over := false
	for _, m := range blocks {
		if m.Overflow > 0 {
			over = true
		}
	}
	if !over {
		return 0
	}
	if math.Abs(inner-capacity) > 0.01 {
		t.Errorf("%s: body box inner height %.2f, measured body area %.2f", name, inner, capacity)
	}
	for i, m := range blocks {
		d := drawn[i]
		if math.Abs(d.y0-m.Top) > 0.01 || math.Abs(d.y1-m.Bottom) > 0.01 {
			t.Errorf("%s: body[%d] drawn at %.2f–%.2f, measured %.2f–%.2f", name, i, d.y0, d.y1, m.Top, m.Bottom)
		}
		if m.Overflow > 0 {
			if got := d.y1 - math.Max(d.y0, capacity); math.Abs(got-m.Overflow) > 0.01 {
				t.Errorf("%s: body[%d] drawn %.2fpt past the body area, layout-overflow says %.2fpt", name, i, got, m.Overflow)
			}
		}
	}
	return 1
}

// 全ての例で、ブロックの描いた図形は次のブロックの枠に入り込まない。はみ出した
// スライドでは、描いた位置と layout-overflow の測りが一致する。
func TestBlocksDoNotOverlapNext(t *testing.T) {
	var files []string
	for _, pat := range []string{"../examples/ir/*.json", "../examples/ir/check-warnings/*.json", "../examples/md/*.bx.md", "../examples/md/warnings/*.bx.md", "../examples/yaml/*.yaml"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	sort.Strings(files)
	overflowing := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"dark", "light", "report"} {
			tm, err := LookupTemplate(name)
			if err != nil {
				t.Fatal(err)
			}
			tm.SetAssets(os.DirFS(filepath.Dir(f)))
			for si, b := range buildLaidOut(t, tm, f, data) {
				where := f + "@" + name + " slide " + itoa(si+1)
				if b.Body != nil {
					for _, o := range stackOverlaps(tm, b.Body, "") {
						t.Errorf("%s: %s", where, o)
					}
				}
				overflowing += checkPushedDown(t, where, b)
			}
		}
	}
	if overflowing == 0 {
		t.Fatal("no overflowing slide among the examples")
	}
}

// 第6回の再現:最小を割る表・phases・列の中の組織図の下にブロックがあっても、
// 図形は次のブロックに重ならず、越えた分は layout-overflow の pt と一致する。
func TestUnderMinPushesNextBlock(t *testing.T) {
	var rows []string
	for i := 0; i < 24; i++ {
		rows = append(rows, "| 仕組み"+itoa(i)+" | 営業 | 移すときに気をつけることを短く書いた文 |")
	}
	var steps []string
	for i := 0; i < 3; i++ {
		steps = append(steps, "  - title: 段"+itoa(i)+"\n    items:\n"+
			"      - {title: 地域の拠点の運営, detail: [週末の催しを開く, 担い手を三人育てる, 費用の見通しを立てる]}\n"+
			"      - {title: 小さく試す, detail: [週末の催しを開く, 担い手を三人育てる]}\n"+
			"      - {title: 広げる, detail: [週末の催しを開く, 担い手を三人育てる, 費用の見通しを立てる]}\n"+
			"      - {title: 続ける, detail: [週末の催しを開く, 担い手を三人育てる, 費用の見通しを立てる]}\n"+
			"      - {title: 振り返る, detail: [週末の催しを開く, 担い手を三人育てる, 費用の見通しを立てる]}")
	}
	var units []string
	for i := 0; i < 3; i++ {
		units = append(units, "  - title: チーム"+itoa(i)+"\n    detail: 架空 太郎\n    members:\n      - {title: 班A, detail: 架空 一郎}\n      - {title: 班B, detail: 架空 二郎}\n      - {title: 班C, detail: 架空 三郎}\n      - {title: 班D, detail: 架空 四郎}\n      - {title: 班E, detail: 架空 五郎}\n      - {title: 班F, detail: 架空 六郎}")
	}
	note := "注:数字はすべて架空です。表の下の注は、表と重ならずに下へ押し出されます。"
	src := "---\ntitle: 再現\n---\n\n" +
		"# 表の下の注\n\n| 仕組み | 部署 | 気をつけること |\n| --- | --- | --- |\n" + strings.Join(rows, "\n") + "\n\n" + note + "\n\n" +
		"# phases の下の注\n\n```phases\nsteps:\n" + strings.Join(steps, "\n") + "\n```\n\n" + note + "\n\n" +
		"# 列の中の組織図\n\n::::columns\n:::column 体制\n```orgchart\nroot: {title: 推進の責任者, detail: 架空 花子}\nunits:\n" + strings.Join(units, "\n") + "\n```\n\n- 下の箇条は組織図に重ならない\n- 二つ目の箇条\n- 三つ目の箇条\n- 四つ目の箇条\n- 五つ目の箇条\n- 六つ目の箇条\n- 七つ目の箇条\n- 八つ目の箇条\n:::\n:::column 補足\n短い補足\n:::\n::::\n"
	for _, name := range []string{"dark", "light", "report"} {
		tm, err := LookupTemplate(name)
		if err != nil {
			t.Fatal(err)
		}
		built := buildLaidOut(t, tm, "repro.bx.md", []byte(src))
		over := 0
		for si, b := range built {
			if b.Body == nil {
				continue
			}
			where := name + " slide " + itoa(si+1)
			for _, o := range stackOverlaps(tm, b.Body, "") {
				t.Errorf("%s: %s", where, o)
			}
			// 押し出された:図・表は Min を割らない。
			var walk func(s *Spec)
			walk = func(s *Spec) {
				if s.Kind == KindSubsystem && s.Subsystem != nil {
					if m := s.Subsystem.Measure(Constraint{Width: s.Box.Width}); m.Min.Height > s.Box.Height+overflowEps {
						t.Errorf("%s: %s is %.1fpt high, under its min %.1fpt", where, s.Subsystem.Name(), s.Box.Height, m.Min.Height)
					}
				}
				for _, c := range s.Children {
					walk(c)
				}
			}
			walk(b.Body)
			n := checkPushedDown(t, where, b)
			if n == 0 {
				t.Logf("%s: fits", where)
			}
			over += n
		}
		if over != 3 {
			t.Errorf("%s: %d overflowing slides, want 3 (the example must be under min)", name, over)
		}
	}
}
