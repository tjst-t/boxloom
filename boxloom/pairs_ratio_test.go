package boxloom

import (
	"math"
	"strings"
	"testing"
)

// 側の幅の比は細かく探す(#115):左が長い対と右が長い対が混ざる pairs では、選んだ比の
// 要る高さ(行ごとの高い方の側の合計)が、前の粗い候補(0.2・0.3・0.5 と、その裏の 0.7・0.8)
// のどれよりも高くならない。比は粗い候補以外の値も取る。組み方を持つ dark・light・report で同じ。
func TestPairsSideShareFine(t *testing.T) {
	left := strings.Repeat("窓口に来る人の半分は月曜の午前に集まり、三割は書類が足りずにもう一度来る。", 3)
	right := strings.Repeat("予約の枠を午後に広げ、要る書類の一覧を前もって送る。", 2)
	pairs := [][2]string{
		{left, "待ち時間"},
		{"二度手間", right},
		{left, right},
	}
	offGrid := false
	for _, tmpl := range []string{"dark", "light", "report"} {
		data := pairsWidthDeck(tmpl, pairs)
		ws := pairsSideWidths(t, tmpl, data, len(pairs))
		s, r := ws[1][0], ws[1][1]
		avail := s + r
		share := s / avail
		info := pairsInfoOf(t, tmpl, data)
		need := func(share float64) float64 {
			h := 0.0
			for _, p := range info.rows {
				h += math.Max(heightForWidth(p.subject, avail*share), heightForWidth(p.response, avail*(1-share)))
			}
			return h
		}
		got := need(share)
		for _, c := range []float64{0.2, 0.3, 0.5, 0.7, 0.8} {
			if h := need(c); got > h+pairSideTie {
				t.Errorf("%s: share %.3f needs %.1fpt, more than %.1fpt at the coarse share %.1f", tmpl, share, got, h, c)
			}
		}
		if got >= need(0.5)-pairSideTie {
			t.Errorf("%s: share %.3f needs %.1fpt, not less than %.1fpt at halves", tmpl, share, got, need(0.5))
		}
		if math.Abs(share*20-math.Round(share*20)) > 1e-3 {
			offGrid = true
		}
		t.Logf("%s: share %.3f (%.1f/%.1fpt) need %.1fpt (halves %.1fpt, 0.3 %.1fpt, 0.7 %.1fpt)", tmpl, share, s, r, got, need(0.5), need(0.3), need(0.7))
	}
	if !offGrid {
		t.Error("every template chose a coarse share (a multiple of 0.05)")
	}
}
