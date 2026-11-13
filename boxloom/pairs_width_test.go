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
		if s/(s+r) > 0.45 {
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
	defer useFonts(tm.Fonts)()
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
		// 短い名前の側を下限まで狭めたときより高くならない(同じ高さなら半々に近い方)。
		need, _ := pairsNeed(info, 800)
		if got, low := need(share), need(min); got > low+pairSideTie {
			t.Errorf("min %.2f: share %.3f needs %.1fpt, more than %.1fpt at the lower bound", min, share, got, low)
		}
	}
}

// 側の幅は要る面積で割る(#108):短い題の側と長い箇条の側の対では、箇条の側が広くなり、
// 対の要る高さ(行ごとの高い方の側の合計)は半々で割ったとき以下になる。箇条の1行が
// 短くても(自然な幅は最長の1行で決まり、量を表さない)、量の多い側が広くなる。
// 側が1つのブロックの形と、見出しと複数のブロックの形の両方で確かめる。
func TestPairsSideWidthByArea(t *testing.T) {
	var items []string
	for _, s := range []string{
		"窓口の予約の枠を午後にも広げて、朝に集まる人を分ける",
		"要る書類の一覧を前もって郵便と電子メールの両方で送る",
		"混む曜日と時間を案内の紙と掲示板に載せて知らせる",
		"二度目に来た人の理由を毎週集めて、案内の文を直す",
		"窓口の数を月曜と金曜の朝だけ一つ増やして待ちを減らす",
		"待っている間に書類を確かめる係を入口に一人置いておく",
	} {
		items = append(items, "- "+s)
	}
	bullets := strings.Join(items, "\n")
	single := pairsWidthDeck("%s", [][2]string{{"待ち時間", bullets}, {"二度手間", bullets}})
	grouped := []byte("---\ntitle: 幅の試験\ntemplate: %s\n---\n\n# 対の幅\n\n:::::pairs 論点 | 打つ手\n" +
		":::subject 待ち時間が長い\n朝に集まる\n:::\n:::response 打つ手\n" + bullets + "\n:::\n" +
		":::subject 二度手間が多い\n書類が足りない\n:::\n:::response 打つ手\n" + bullets + "\n:::\n:::::\n")
	for name, form := range map[string][]byte{"single": single, "grouped": grouped} {
		for _, tmpl := range []string{"dark", "light", "report"} {
			data := []byte(strings.Replace(string(form), "%s", tmpl, 1))
			ws := pairsSideWidths(t, tmpl, data, 2)
			s, r := ws[1][0], ws[1][1]
			if r <= s+1 {
				t.Errorf("%s/%s: bullets side %.1fpt is not wider than the title side %.1fpt", name, tmpl, r, s)
			}
			info := pairsInfoOf(t, tmpl, data)
			// 2つの側の幅の合計は、配置で決まった側の幅の和(印・隙間を除いた幅)。
			avail := s + r
			need := func(share float64) float64 {
				h := 0.0
				for _, p := range info.rows {
					h += math.Max(heightForWidth(p.subject, avail*share), heightForWidth(p.response, avail*(1-share)))
				}
				return h
			}
			share := s / avail
			if got, half := need(share), need(0.5); got > half+0.01 {
				t.Errorf("%s/%s: pairs need %.1fpt at share %.2f, more than %.1fpt at halves", name, tmpl, got, share, half)
			} else {
				t.Logf("%s/%s: share %.2f need %.1fpt (halves %.1fpt)", name, tmpl, share, got, half)
			}
		}
	}
}

// pairsInfoOf は1枚目の pairs の箱の部品(テンプレート tmpl で組んだもの)。
func pairsInfoOf(t *testing.T, tmpl string, data []byte) *pairsInfo {
	t.Helper()
	deck, err := ReadDeck("w.bx.md", data)
	if err != nil {
		t.Fatal(err)
	}
	tm := mustTemplate(tmpl)
	b, err := tm.BuildSlide(deck, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 呼び出し側が heightForWidth で測るので、試験の終わりまでテンプレートの字体で測る
	// (組み立てと同じ字体でないと、行の高さが組み立ての測りと違う)。
	t.Cleanup(useFonts(tm.Fonts))
	var info *pairsInfo
	var find func(s *Spec)
	find = func(s *Spec) {
		if s.pairs != nil && info == nil {
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
	return info
}
