package boxloom

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

// pairsRatioDeck は1枚の pairs の deck(架空の内容)。dense なら本文の密度を dense に、
// label は対ごとの名前の書式(%d に対の番号。空なら名前なし)、grouped なら側を見出しと
// 2つのブロックにする。
func pairsRatioDeck(tmpl string, dense bool, label string, grouped bool) []byte {
	left := strings.Repeat("窓口に来る人の半分は月曜の午前に集まり、三割は書類が足りずにもう一度来る。", 3)
	right := strings.Repeat("予約の枠を午後に広げ、要る書類の一覧を前もって送る。", 2)
	sides := [][2]string{{left, "待ち時間"}, {"二度手間", right}, {left, right}}
	var b strings.Builder
	d := ""
	if dense {
		d = " {density=dense}"
	}
	fmt.Fprintf(&b, "---\ntitle: 比の試験\ntemplate: %s\n---\n\n# 対の比%s\n\n:::::pairs 論点 | 説明\n", tmpl, d)
	for k, s := range sides {
		l := ""
		if label != "" {
			l = " {label=" + fmt.Sprintf(label, k+1) + "}"
		}
		if grouped {
			fmt.Fprintf(&b, ":::subject 現状%s\n%s\n\n- 窓口の数は変えていない\n:::\n:::response 打ち手\n%s\n:::\n", l, s[0], s[1])
			continue
		}
		fmt.Fprintf(&b, ":::subject%s\n%s\n:::\n:::response\n%s\n:::\n", l, s[0], s[1])
	}
	b.WriteString(":::::\n")
	return []byte(b.String())
}

// report・dense・対の名前(印の中・見出しの列・subject の頭)・見出しと複数のブロックの側でも、
// 側の幅の比は刻みで探される(#135):組んで並べた比のとき、見出しの行と対の行の高さ(側の高い方と、
// 印・名前の列の高さのうち高い方)の合計が、粗い比(0.2・0.3・0.5・0.7・0.8)のどれよりも
// 高くならず、半々より低い。比は粗い候補以外の値も取る。
func TestPairsSideShareReportDenseLabel(t *testing.T) {
	type tc struct {
		tmpl    string
		dense   bool
		label   string
		grouped bool
	}
	var cases []tc
	for _, tmpl := range []string{"report", "dark", "light"} {
		for _, dense := range []bool{false, true} {
			for _, label := range []string{"", "%d", "要因の%d"} {
				for _, grouped := range []bool{false, true} {
					cases = append(cases, tc{tmpl, dense, label, grouped})
				}
			}
		}
	}
	offGrid := false
	for _, c := range cases {
		name := fmt.Sprintf("%s dense=%v label=%q grouped=%v", c.tmpl, c.dense, c.label, c.grouped)
		deck, err := ReadDeck("r.bx.md", pairsRatioDeck(c.tmpl, c.dense, c.label, c.grouped))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, share := range checkPairsShare(t, name, c.tmpl, deck, 1, true) {
			if math.Abs(share*20-math.Round(share*20)) > 1e-3 {
				offGrid = true
			}
		}
	}
	if !offGrid {
		t.Error("every case chose a coarse share (a multiple of 0.05)")
	}
}

// 例の deck(架空の内容)を report で、本文の密度を normal と dense にして組んでも、選んだ比の
// 対の行の高さの合計は粗い比のどれよりも高くならない(#135。dense のときも探し直す)。
func TestPairsSideShareExamplesReport(t *testing.T) {
	for _, f := range []string{"pairs", "pairs-label", "pairs-side-body", "pairs-side-untitled"} {
		data, err := os.ReadFile("../examples/md/" + f + ".bx.md")
		if err != nil {
			t.Fatal(err)
		}
		for _, dense := range []bool{false, true} {
			deck, err := ReadDeck(f+".bx.md", data)
			if err != nil {
				t.Fatal(err)
			}
			for i := range deck.Slides {
				if deck.Slides[i].Type != "content" {
					continue
				}
				if dense {
					deck.Slides[i].Density = "dense"
				}
				checkPairsShare(t, fmt.Sprintf("%s slide %d dense=%v", f, i, dense), "report", deck, i, false)
			}
		}
	}
}

// checkPairsShare は deck の i 枚目を tmpl で組んで並べ、全ての対の並びについて、選んだ比の
// 見出しの行と対の行の高さ(行を並べて測る。側の高い方と、印・名前の列の高さのうち高い方)の合計が粗い比
// (0.2・0.3・0.5・0.7・0.8)のどれよりも高くないことを確かめる。belowHalves なら半々より
// 低いことも。選んだ比を返す。
func checkPairsShare(t *testing.T, name, tmpl string, deck *IRDeck, i int, belowHalves bool) []float64 {
	t.Helper()
	tm := mustTemplate(tmpl)
	defer useFonts(tm.Fonts)()
	tm.SetAssets(os.DirFS("../examples/md"))
	b, err := tm.BuildSlide(deck, i)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	var infos []*pairsInfo
	var find func(s *Spec)
	find = func(s *Spec) {
		if s.pairs != nil {
			infos = append(infos, s.pairs)
		}
		for _, k := range s.Children {
			find(k)
		}
	}
	find(b.Root)
	var out []float64
	for _, info := range infos {
		r0 := info.rows[0]
		share := r0.subject.Box.Width / (r0.subject.Box.Width + r0.response.Box.Width)
		rowW := r0.row.Box.Width
		need := func(sh float64) float64 {
			info.setShare(sh)
			h := heightForWidth(info.head, rowW)
			for _, r := range info.rows {
				h += heightForWidth(r.row, rowW)
			}
			return h
		}
		got := need(share)
		for _, coarse := range []float64{0.2, 0.3, 0.5, 0.7, 0.8} {
			if h := need(coarse); got > h+pairSideTie {
				t.Errorf("%s: share %.3f needs %.1fpt, more than %.1fpt at the coarse share %.1f", name, share, got, h, coarse)
			}
		}
		if half := need(0.5); belowHalves && got >= half-pairSideTie {
			t.Errorf("%s: share %.3f needs %.1fpt, not less than %.1fpt at halves", name, share, got, half)
		}
		t.Logf("%s: share %.3f need %.1fpt (halves %.1fpt)", name, share, got, need(0.5))
		out = append(out, share)
	}
	return out
}

// 対の名前の見出しの列(report の3字以上の label。#118)が折り返して行を高くするときは、
// 側がその高さに収まるうちは側を狭めても行は高くならない。比の探索はその高さも数える
// (#135):前は側だけを数えて半々近く(0.48)を選び、名前の列で決まる行の分だけ、
// 長い文の側が要らずに高かった。
func TestPairsSideShareLabelColumnFloor(t *testing.T) {
	long := strings.Repeat("予約の枠を午後に広げ、要る書類の一覧を前もって送る。", 3)
	mid := "窓口の前で待つ人の列が昼まで長く続く"
	md := "---\ntitle: 比の試験\ntemplate: report\n---\n\n# 対の比\n\n:::::pairs 論点 | 説明\n" +
		":::subject {label=窓口で待つ時間の長さ}\n" + mid + "\n:::\n:::response\n短い\n:::\n" +
		":::subject {label=二度手間の多さと理由}\n" + mid + "\n:::\n:::response\n" + long + "\n:::\n:::::\n"
	deck, err := ReadDeck("r.bx.md", []byte(md))
	if err != nil {
		t.Fatal(err)
	}
	shares := checkPairsShare(t, "label column", "report", deck, 1, true)
	if len(shares) != 1 || shares[0] > 0.3 {
		t.Errorf("shares %v: want the short side narrowed below 0.3 (the label column sets the row height)", shares)
	}
}
