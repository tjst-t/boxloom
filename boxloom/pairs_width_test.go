package boxloom

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// pairsWidthDeck は1枚の pairs の deck(Markdown)。sides は対ごとの subject と response。
func pairsWidthDeck(tmpl string, sides [][2]string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: 幅の試験\ntemplate: %s\n---\n\n# 対の幅\n\n:::::pairs 論点 | 説明\n", tmpl)
	for _, s := range sides {
		fmt.Fprintf(&b, ":::subject\n%s\n:::\n:::response\n%s\n:::\n", s[0], s[1])
	}
	b.WriteString(":::::\n")
	return []byte(b.String())
}

// pairsSideWidths は各対(見出しの行を含む)の subject と response の幅。
func pairsSideWidths(t *testing.T, tmpl string, data []byte, n int) [][2]float64 {
	t.Helper()
	dump, err := Layout("w.bx.md", data, BuildOptions{Template: tmpl})
	if err != nil {
		t.Fatal(err)
	}
	seen := pairsLayoutNodes(dump)
	var out [][2]float64
	paths := []string{"/slides/1/body/0/headings"}
	for k := 0; k < n; k++ {
		paths = append(paths, fmt.Sprintf("/slides/1/body/0/pairs/%d", k))
	}
	for _, p := range paths {
		s, r := seen[p+"/subject"], seen[p+"/response"]
		if s == nil || r == nil {
			t.Fatalf("%s: no side boxes at %s", tmpl, p)
		}
		out = append(out, [2]float64{s.Box.Width, r.Box.Width})
	}
	return out
}

// 側の幅は中身で割る:短い側(論点の名前)と長い側(説明の文)の対では長い側が広く、
// 短い側も pairsSideMin より狭くならない。全ての行(見出しの行も)が同じ割り方。
// 両側が同じくらいなら半々のまま。組み方を持つ dark・light・report で同じ規則。
func TestPairsSideWidthFromContent(t *testing.T) {
	long := "窓口に来る人の半分は月曜と金曜の午前に集まり、そのうち三割は書類が足りずにもう一度来ている。予約の枠を午後に広げ、要る書類の一覧を前もって送れば、待ち時間と二度手間の両方が減る見込みがある"
	uneven := [][2]string{{"待ち時間", long}, {"二度手間", long}, {"案内", long}}
	even := [][2]string{
		{"予約の枠を午後に広げる", "要る書類の一覧を送る"},
		{"混む時間を案内に載せる", "窓口の数を朝だけ増やす"},
	}
	for _, tmpl := range []string{"dark", "light", "report"} {
		tm := mustTemplate(tmpl)
		ws := pairsSideWidths(t, tmpl, pairsWidthDeck(tmpl, uneven), len(uneven))
		for i, w := range ws {
			if !nearPt(w[0], ws[0][0]) || !nearPt(w[1], ws[0][1]) {
				t.Errorf("%s: row %d split %.1f/%.1f differs from the headings %.1f/%.1f", tmpl, i, w[0], w[1], ws[0][0], ws[0][1])
			}
		}
		s, r := ws[1][0], ws[1][1]
		if s/(s+r) > 0.4 {
			t.Errorf("%s: long side %.1fpt is not clearly wider than the short side %.1fpt", tmpl, r, s)
		}
		if share := s / (s + r); share < tm.Values.PairsSideMin-0.001 {
			t.Errorf("%s: short side share %.3f is below pairsSideMin %.2f", tmpl, share, tm.Values.PairsSideMin)
		}
		// 長い側を広くしたことで、行は低くなる(はみ出しも無い)。
		res := Check("w.bx.md", pairsWidthDeck(tmpl, uneven), CheckOptions{Template: tmpl})
		for _, w := range res.Warnings {
			if w.Code == "layout-overflow" {
				t.Errorf("%s: %+v", tmpl, w)
			}
		}

		ws = pairsSideWidths(t, tmpl, pairsWidthDeck(tmpl, even), len(even))
		for i, w := range ws {
			if math.Abs(w[0]-w[1]) > 0.01 {
				t.Errorf("%s: even row %d split %.1f/%.1f, want halves", tmpl, i, w[0], w[1])
			}
		}
	}
}

// pairSideShare は下限の外へ出ない(長い文どうしでも、短い名前と長い文でも)。
func TestPairSideShareBounds(t *testing.T) {
	tm := mustTemplate("light")
	for _, min := range []float64{0.2, 0.35} {
		tm.Values.PairsSideMin = min
		long := strings.Repeat("長い説明の文を並べる。", 20)
		data := pairsWidthDeck("light", [][2]string{{"名前", long}, {"別の名前", long}})
		deck, err := ReadDeck("w.bx.md", data)
		if err != nil {
			t.Fatal(err)
		}
		b, err := tm.BuildSlide(deck, 1)
		if err != nil {
			t.Fatal(err)
		}
		var info *pairsInfo
		var find func(s *Spec)
		find = func(s *Spec) {
			if s.pairs != nil {
				info = s.pairs
			}
			for _, c := range s.Children {
				find(c)
			}
		}
		find(b.Root)
		if info == nil {
			t.Fatal("no pairs box")
		}
		share := pairSideShare(info, 800)
		if share < min-1e-9 || share > 0.5 {
			t.Errorf("min %.2f: share %.3f", min, share)
		}
		if math.Abs(share-min) > pairSideStep+1e-9 {
			t.Errorf("min %.2f: a short name next to a long text should sit near the lower bound, got %.3f", min, share)
		}
	}
}
