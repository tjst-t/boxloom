package boxloom

import (
	"math"
	"strings"
	"testing"
)

// 釣り合いは側の形によらず効く(#113):見出し+箇条+引用(または表)の側と、短い文の側の
// 対では、対の要る高さ(行ごとの高い方の側の合計)が半々で割ったときより小さい。
// 組み方を持つ dark・light・report で同じ。
func TestPairsBalanceGroupedSides(t *testing.T) {
	bullets := strings.Join([]string{
		"- 窓口の予約の枠を午後にも広げて、朝に集まる人を分ける",
		"- 要る書類の一覧を前もって郵便と電子メールの両方で送る",
		"- 混む曜日と時間を案内の紙と掲示板に載せて知らせる",
		"- 二度目に来た人の理由を毎週集めて、案内の文を直す",
	}, "\n")
	quote := ":::quote 利用者 A、2026 年 9 月\n待つ時間が短くなれば、来る人の気持ちも変わると思います。\n:::"
	table := "| 窓口 | 待ち時間 | 件数 |\n|---|---|---|\n| 北口の受付 | 四十分 | 120 |\n| 電話の相談 | 十分 | 300 |"
	forms := map[string]string{"quote": quote, "table": table}
	for name, extra := range forms {
		grouped := "::::subject 打つ手\n" + bullets + "\n\n" + extra + "\n::::\n"
		for _, tmpl := range []string{"dark", "light", "report"} {
			data := []byte("---\ntitle: 釣り合いの試験\ntemplate: " + tmpl + "\n---\n\n# 対の釣り合い\n\n:::::pairs 打つ手 | 論点\n" +
				grouped + ":::response\n待ち時間\n:::\n" +
				grouped + ":::response\n二度手間\n:::\n:::::\n")
			ws := pairsSideWidths(t, tmpl, data, 2)
			s, r := ws[1][0], ws[1][1]
			if s <= r+1 {
				t.Errorf("%s/%s: grouped side %.1fpt is not wider than the short side %.1fpt", name, tmpl, s, r)
			}
			info := pairsInfoOf(t, tmpl, data)
			avail := s + r
			need := func(share float64) float64 {
				h := 0.0
				for _, p := range info.rows {
					h += math.Max(heightForWidth(p.subject, avail*share), heightForWidth(p.response, avail*(1-share)))
				}
				return h
			}
			got, half := need(s/avail), need(0.5)
			if got >= half-0.01 {
				t.Errorf("%s/%s: pairs need %.1fpt after balancing, not less than %.1fpt at halves", name, tmpl, got, half)
			} else {
				t.Logf("%s/%s: share %.2f need %.1fpt (halves %.1fpt)", name, tmpl, s/avail, got, half)
			}
		}
	}
}

// 図の側が使わずに残した幅は、もう片側へ回す(#113):縦長の画像の側と長い文の側の対では、
// 画像が高さで縮んで残した横の余りが文の側へ回り、spare-width は出ない。
func TestPairsGiveFigureSpare(t *testing.T) {
	long := strings.Repeat("窓口に来る人の半分は月曜と金曜の午前に集まり、そのうち三割は書類が足りずにもう一度来ている。", 4)
	pair := "::::subject 画面\n![縦長の図](img/tall.png)\n::::\n:::response\n" + long + "\n:::\n"
	for _, tmpl := range []string{"dark", "light", "report"} {
		data := []byte("---\ntitle: 余りの試験\ntemplate: " + tmpl + "\n---\n\n# 図の余り\n\n:::::pairs 画面 | 説明\n" +
			pair + pair + ":::::\n")
		opts := BuildOptions{Template: tmpl, Assets: testAssets(t)}
		res := Check("w.bx.md", data, CheckOptions{Template: tmpl, Assets: testAssets(t)})
		for _, w := range res.Warnings {
			if w.Code == "spare-width" {
				t.Errorf("%s: %s %s", tmpl, w.Path, w.Message)
			}
		}
		dump, err := Layout("w.bx.md", data, opts)
		if err != nil {
			t.Fatal(err)
		}
		seen := pairsLayoutNodes(dump)
		sub, resp := seen["/slides/1/body/0/pairs/0/subject"], seen["/slides/1/body/0/pairs/0/response"]
		if sub == nil || resp == nil {
			t.Fatalf("%s: no side boxes", tmpl)
		}
		share := sub.Box.Width / (sub.Box.Width + resp.Box.Width)
		if min := mustTemplate(tmpl).Values.PairsSideMin; share >= min-0.01 {
			t.Errorf("%s: image side keeps share %.3f (pairsSideMin %.2f); its spare was not given to the text side", tmpl, share, min)
		}
		t.Logf("%s: image side %.1fpt, text side %.1fpt", tmpl, sub.Box.Width, resp.Box.Width)
	}
}

// 回せない余り(同じ列の別の対の文がそれ以上狭められない)は、側の幅に対して測って
// 対の側を指す spare-width で知らせる(対の並びの全体の幅では測らない)。
func TestPairsSpareWidthPerSide(t *testing.T) {
	long := strings.Repeat("窓口に来る人の半分は月曜と金曜の午前に集まり、そのうち三割は書類が足りずにもう一度来ている。", 3)
	for _, tmpl := range []string{"dark", "light", "report"} {
		data := []byte("---\ntitle: 余りの試験\ntemplate: " + tmpl + "\n---\n\n# 図の余り\n\n:::::pairs 画面 | 説明\n" +
			":::subject\n![縦長の図](img/tall.png)\n:::\n:::response\n短い説明\n:::\n" +
			":::subject\n" + long + "\n:::\n:::response\n短い説明\n:::\n:::::\n")
		res := Check("w.bx.md", data, CheckOptions{Template: tmpl, Assets: testAssets(t)})
		found := 0
		for _, w := range res.Warnings {
			if w.Code != "spare-width" {
				continue
			}
			found++
			if w.Path != "/slides/1/body/0/pairs/0/subject/body/0" || !strings.Contains(w.Message, "side width") {
				t.Errorf("%s: spare-width not measured per side: %s %s", tmpl, w.Path, w.Message)
			}
			if w.Hint != sparePairHint {
				t.Errorf("%s: hint %q", tmpl, w.Hint)
			}
		}
		if found == 0 {
			t.Errorf("%s: no spare-width for the image side that could not give its spare", tmpl)
		}
	}
}
