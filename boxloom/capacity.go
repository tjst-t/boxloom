// 容量の計測(capacity):本文にあとどれだけ入るかを数字で出す。
//
// はみ出しの検査(overflow.go)と同じ測り方(measureBody:縮めずに要る高さを
// 積む)を、警告が無いときも数字のまま返す。AI や道具が「あと何行入るか」
// 「どこでスライドを分けるか」を決めるため。形は docs/ir.md の「容量の計測」。
//
// layout-overflow の警告とは、同じ measureBody から作るので数字が食い違わない
// (警告の文の pt は、ここの値を 0.1pt に丸めたものと同じ)。

package boxloom

import (
	"fmt"
	"math"
)

// SlideCapacity は本文を持つ1枚のスライドの容量。表紙など本文の無い
// スライドは出さない。高さはすべて pt で、0.1pt に丸める。
type SlideCapacity struct {
	File      string  `json:"file,omitempty"`
	Line      int     `json:"line,omitempty"`
	Path      string  `json:"path"`      // /slides/S
	BodyArea  float64 `json:"bodyArea"`  // 本文の領域の高さ(ページの内側 − 題・出典・フッター − gap)
	Need      float64 `json:"need"`      // ブロックを縮めずに積んだ高さ(gap を含む)
	Remaining float64 `json:"remaining"` // bodyArea − need。負ならはみ出している
	// BlockGap はブロックの間の gap。新しいブロックを足すなら、先にこれだけ要る。
	BlockGap float64 `json:"blockGap"`
	// BodyLineHeight は本文の字の大きさの1行の高さ。
	BodyLineHeight float64 `json:"bodyLineHeight"`
	// RemainingBodyLines は、残りに本文の大きさの行がいくつ入るか(既にある
	// ブロックに行を足す場合。新しいブロックなら blockGap を引いてから数える)。
	// 残りが負なら 0。
	RemainingBodyLines int `json:"remainingBodyLines"`
	// Density は密なスライド(density: dense)のとき "dense"(本文の値は値の表の dense の組)。
	// normal のスライドでは省く。
	Density string `json:"density,omitempty"`
	// DenseRemaining は normal の本文のスライドを dense にしたときの remaining(実際に
	// dense の値で組み直して測る。density.go)。負なら dense でもはみ出す。
	// dense のスライドでは省く(remaining がそれ)。
	DenseRemaining *float64 `json:"denseRemaining,omitempty"`
	// Fit は -fit(BuildOptions.Fit・CheckOptions.Fit)で本文を詰めたときの段の名前
	// ("dense"・"fit.shrink[K]"・"fit.grow[K]"・"fit.emergency[K]"。fit_shrink.go・fit_grow.go)。remaining・本文の行の高さはその段の値。
	// 詰めなかったスライドでは省く。
	Fit string `json:"fit,omitempty"`
	// Split は -fit で本文を続きのスライドに分けたとき(fit_split.go)の、この枚の印(何枚目・
	// 枚数・各枚の始まり)。分けたスライドは枚ごとに容量を出し、path はどれも元の /slides/S。
	// 分けなかったスライドでは省く。
	Split  *SplitCapacity  `json:"split,omitempty"`
	Blocks []BlockCapacity `json:"blocks"`
}

// BlockCapacity は本文の1つのブロックの測り値。
type BlockCapacity struct {
	Line     int     `json:"line,omitempty"`
	Path     string  `json:"path"` // /slides/S/body/N
	Type     string  `json:"type"` // IR の type
	Need     float64 `json:"need"`
	Lines    int     `json:"lines"` // 中の文字の行の数(折り返し後)
	Top      float64 `json:"top"`
	Bottom   float64 `json:"bottom"`
	Overflow float64 `json:"overflow"` // 本文の領域の下端を越えた高さ(0 なら収まる)
	// RemainingWidth は、ブロックの中の図・画像が左右に使わずに残した幅の合計
	// (縦横比を保つ図・横の上限を持つ図の横の空き、spare.go)。本文のブロック
	// だけで、無ければ省く。
	RemainingWidth float64 `json:"remainingWidth,omitempty"`
	// Columns は並列のまとまり(columns)のときだけ、まとまりごとの測り値。
	Columns []ColumnCapacity `json:"columns,omitempty"`
	// Cards は横に同じ幅のカードを並べるブロック(compare の選択肢・stat の数)の
	// ときだけ、カードごとの測り値。ブロックの need を決めるのは一番高いカード
	// (tallest)で、ほかのカードに文を足しても need は増えない。
	Cards []CardCapacity `json:"cards,omitempty"`
	// Pairs は対の並び(pairs)のときだけ、対ごとの測り値。
	Pairs []PairCapacity `json:"pairs,omitempty"`
	// Topics は印つきの塊の並び(topics)のときだけ、塊ごとの測り値。
	Topics []TopicCapacity `json:"topics,omitempty"`
	// Split は本文の表がはみ出したときだけ、表を何枚に分けられるかの案。
	Split *TableSplitCapacity `json:"split,omitempty"`
}

// TableSplitCapacity ははみ出した表の分け方の案。表を tables 枚に分け、2枚目からの
// 表は splitBefore の本体の行(IR の rows の番号)から始める。続きの表には
// continues: true を書き、見出しの行は書かない(docs/ir.md の 14章)。表の後の
// ブロック(注記など)は最初の枚に残し、続きの枚は本文が表だけで、題・出典・フッターは
// 同じと見込む。
type TableSplitCapacity struct {
	Tables      int   `json:"tables"`
	SplitBefore []int `json:"splitBefore"`
	Rows        []int `json:"rows"` // 表ごとの本体の行の数
}

// CardCapacity は横に並ぶカードの1枚の測り値。
type CardCapacity struct {
	Path    string  `json:"path"` // /slides/S/body/N/options/K(stat は …/items/K)
	Need    float64 `json:"need"`
	Lines   int     `json:"lines"`
	Tallest bool    `json:"tallest,omitempty"` // このカードがブロックの高さを決める
}

// ColumnCapacity は並列のまとまりの1つの測り値(Top・Bottom は本文の領域の上端から)。
// Need は題と本文のブロックを積んだ高さで、一番高いまとまりがブロックの need になる。
type ColumnCapacity struct {
	Line     int             `json:"line,omitempty"`
	Path     string          `json:"path"` // /slides/S/body/N/columns/C
	Need     float64         `json:"need"`
	Lines    int             `json:"lines"`
	Top      float64         `json:"top"`
	Bottom   float64         `json:"bottom"`
	Overflow float64         `json:"overflow"`
	Blocks   []BlockCapacity `json:"blocks"` // まとまりの本文(path は …/columns/C/body/K)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// slideCapacity は measureBody の結果から容量を作る。si は IR のスライドの番号、
// gap は本文のブロックの間の gap、bodySize は本文の字の大きさ。
func slideCapacity(blocks []BlockMeasure, area, need, gap float64, si int, types []string, bodySize float64) SlideCapacity {
	lh := bodySize * lineHeightMult()
	sc := SlideCapacity{
		Path:           fmt.Sprintf("/slides/%d", si),
		BodyArea:       round1(area),
		Need:           round1(need),
		Remaining:      round1(area - need),
		BlockGap:       round1(gap),
		BodyLineHeight: round1(lh),
		Blocks:         []BlockCapacity{},
	}
	if rem := area - need; rem > 0 && lh > 0 {
		sc.RemainingBodyLines = int(math.Floor(rem/lh + 1e-9))
	}
	for _, b := range blocks {
		t := ""
		if b.Index < len(types) {
			t = types[b.Index]
		}
		bc := blockCapacity(b, fmt.Sprintf("%s/body/%d", sc.Path, b.Index), t)
		for _, c := range b.Columns {
			cp := fmt.Sprintf("%s/columns/%d", bc.Path, c.Index)
			cc := ColumnCapacity{Path: cp, Need: round1(c.Need), Lines: c.Lines, Top: round1(c.Top),
				Bottom: round1(c.Bottom), Overflow: round1(c.Overflow), Blocks: []BlockCapacity{}}
			for _, ib := range c.Blocks {
				cc.Blocks = append(cc.Blocks, blockCapacity(ib, fmt.Sprintf("%s/body/%d", cp, ib.Index), ib.Type))
			}
			bc.Columns = append(bc.Columns, cc)
		}
		sc.Blocks = append(sc.Blocks, bc)
	}
	return sc
}

func blockCapacity(b BlockMeasure, path, typ string) BlockCapacity {
	bc := BlockCapacity{
		Path:     path,
		Type:     typ,
		Need:     round1(b.Need),
		Lines:    b.Lines,
		Top:      round1(b.Top),
		Bottom:   round1(b.Bottom),
		Overflow: round1(b.Overflow),
	}
	for _, p := range b.Pairs {
		bc.Pairs = append(bc.Pairs, pairCapacity(p, path))
	}
	for _, m := range b.Topics {
		bc.Topics = append(bc.Topics, topicCapacity(m, path))
	}
	if s := b.TableSplit; s != nil {
		bc.Split = &TableSplitCapacity{Tables: s.Tables, SplitBefore: s.SplitBefore, Rows: s.Rows}
	}
	tall := tallestCard(b.Cards)
	for i, c := range b.Cards {
		bc.Cards = append(bc.Cards, CardCapacity{Path: fmt.Sprintf("%s/%s/%d", path, b.CardsKey, c.Index),
			Need: round1(c.Need), Lines: c.Lines, Tallest: i == tall})
	}
	return bc
}

// String は check の text の出し方(-capacity)の1行:
//
//	capacity: deck.bx.md:12 /slides/1: body area 413.4pt, need 300.0pt, remaining 113.4pt (6 body lines)
//	  /slides/1/body/0 points: 7 lines, 168.3pt
func (c SlideCapacity) String() string {
	loc := c.File
	if c.Line > 0 {
		loc = fmt.Sprintf("%s:%d", loc, c.Line)
	}
	out := fmt.Sprintf("capacity: %s %s: body area %.1fpt, need %.1fpt, remaining %.1fpt (%d body lines)",
		loc, c.Path, c.BodyArea, c.Need, c.Remaining, c.RemainingBodyLines)
	if c.Density != "" {
		out += ", " + c.Density
	}
	if c.Fit != "" {
		out += ", fit " + c.Fit
	}
	if s := c.Split; s != nil {
		out += fmt.Sprintf(", split part %d of %d", s.Part, s.Slides)
	}
	if c.DenseRemaining != nil {
		out += fmt.Sprintf(", remaining %.1fpt if dense", *c.DenseRemaining)
	}
	for _, b := range c.Blocks {
		out += fmt.Sprintf("\n  %s %s: %d lines, %.1fpt", b.Path, b.Type, b.Lines, b.Need)
		if b.Overflow > 0 {
			out += fmt.Sprintf(", overflows by %.1fpt", b.Overflow)
		}
		if b.RemainingWidth > 0 {
			out += fmt.Sprintf(", %.1fpt of width unused beside the figure", b.RemainingWidth)
		}
		if s := b.Split; s != nil {
			out += fmt.Sprintf(", can be split into %d tables before rows %v", s.Tables, s.SplitBefore)
		}
		for _, c := range b.Cards {
			out += fmt.Sprintf("\n    %s: %d lines, %.1fpt", c.Path, c.Lines, c.Need)
			if c.Tallest {
				out += " (tallest: sets the block height)"
			}
		}
		for _, p := range b.Pairs {
			out += fmt.Sprintf("\n    %s: %d lines, %.1fpt (subject %.1fpt, response %.1fpt)", p.Path, p.Lines, p.Need, p.Subject, p.Response)
			if p.Overflow > 0 {
				out += fmt.Sprintf(", overflows by %.1fpt", p.Overflow)
			}
		}
		for _, c := range b.Topics {
			out += fmt.Sprintf("\n    %s: %d lines, %.1fpt", c.Path, c.Lines, c.Need)
			if c.Overflow > 0 {
				out += fmt.Sprintf(", overflows by %.1fpt", c.Overflow)
			}
		}
		for _, c := range b.Columns {
			out += fmt.Sprintf("\n    %s: %d lines, %.1fpt", c.Path, c.Lines, c.Need)
			if c.Overflow > 0 {
				out += fmt.Sprintf(", overflows by %.1fpt", c.Overflow)
			}
		}
	}
	return out
}
