// はみ出しの印(#141)。-allow-overflow(BuildOptions.AllowOverflow)で作るときだけ、
// はみ出したスライドの本文の領域の下端に細い線と「ここから N pt はみ出し」の文を描く。
// 押し出された分はページの下端の外に描かれて切れて見えるだけなので、どこから外れたかを
// PPTX の上で見えるようにする。2つの図形はどちらも名前 boxloom-overflow で、作者が
// 選んで消せる。はみ出しが無い・AllowOverflow でないときは何も足さない(出力は変わらない)。

package boxloom

import "fmt"

// overflowMarkName は印の図形の名前(線と文の両方)。
const overflowMarkName = "boxloom-overflow"

// overflowMark は1枚のスライドの印の置き場所。X・Width は本文の領域の横、Y は下端、
// Over は下端を越えた高さ(pt)。
type overflowMark struct {
	X, Y, Width float64
	Over        float64
	Color       string  // 16進(役割名 danger。無ければ accent・text)
	FontSize    float64 // 文の字の大きさ(値の表の small)
}

// newOverflowMark は本文の箱 area(ページの箱の直接の子)の印を作る。capacity・need は
// measureBody の本文の領域の高さと積んだ高さ。
func (t *Template) newOverflowMark(area *Spec, capacity, need float64) *overflowMark {
	color := ""
	for _, role := range []string{"danger", "accent", "text"} {
		if c := t.Theme[role]; c != "" {
			color = c
			break
		}
	}
	return &overflowMark{
		X:        area.Box.X + area.padX(),
		Y:        area.Box.Y + area.padY() + capacity,
		Width:    max0(area.Box.Width - 2*area.padX()),
		Over:     need - capacity,
		Color:    color,
		FontSize: t.Values.Small,
	}
}

// addOverflowMark は印をスライドの一番上(図形の並びの最後)に足す。
func (p *PPTXBuilder) addOverflowMark(m *overflowMark) {
	const lineW = 1.5
	p.AddLine(m.X, m.Y, m.X+m.Width, m.Y, lineW, m.Color, "")
	p.Shapes[len(p.Shapes)-1].name = overflowMarkName
	label := fmt.Sprintf("ここから %.1f pt はみ出し", m.Over)
	h := m.FontSize * lineHeightMult()
	w := min(m.Width, runWidthPt(label, m.FontSize, false)+4)
	p.AddTextBoxNoWrap(LBox{X: m.X + m.Width - w, Y: m.Y - lineW - h, Width: w, Height: h}, label, m.FontSize, m.Color, false, "right", "bottom")
	p.Shapes[len(p.Shapes)-1].name = overflowMarkName
}
