// 容量の計測(capacity):本文にあとどれだけ入るかを数字で出す。
//
// はみ出しの検査(overflow.go)と同じ測り方(measureBody:縮めずに要る高さを
// 積む)を、警告が無いときも数字のまま返す。AI や道具が「あと何行入るか」
// 「どこでスライドを分けるか」を決めるため。形は docs/ir.md の「容量の計測」。
//
// layout-overflow の警告とは、同じ measureBody から作るので数字が食い違わない
// (警告の文の pt は、ここの値を 0.1pt に丸めたものと同じ)。
package main

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
	BodyArea  float64 `json:"bodyArea"`  // 本文の領域の高さ(ページの内側 − 題・フッター − gap)
	Need      float64 `json:"need"`      // ブロックを縮めずに積んだ高さ(gap を含む)
	Remaining float64 `json:"remaining"` // bodyArea − need。負ならはみ出している
	// BlockGap はブロックの間の gap。新しいブロックを足すなら、先にこれだけ要る。
	BlockGap float64 `json:"blockGap"`
	// BodyLineHeight は本文の字の大きさの1行の高さ。
	BodyLineHeight float64 `json:"bodyLineHeight"`
	// RemainingBodyLines は、残りに本文の大きさの行がいくつ入るか(既にある
	// ブロックに行を足す場合。新しいブロックなら blockGap を引いてから数える)。
	// 残りが負なら 0。
	RemainingBodyLines int             `json:"remainingBodyLines"`
	Blocks             []BlockCapacity `json:"blocks"`
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
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// slideCapacity は measureBody の結果から容量を作る。si は IR のスライドの番号、
// gap は本文のブロックの間の gap、bodySize は本文の字の大きさ。
func slideCapacity(blocks []BlockMeasure, area, need, gap float64, si int, types []string, bodySize float64) SlideCapacity {
	lh := bodySize * lineHeightMult
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
		sc.Blocks = append(sc.Blocks, BlockCapacity{
			Path:     fmt.Sprintf("%s/body/%d", sc.Path, b.Index),
			Type:     t,
			Need:     round1(b.Need),
			Lines:    b.Lines,
			Top:      round1(b.Top),
			Bottom:   round1(b.Bottom),
			Overflow: round1(b.Overflow),
		})
	}
	return sc
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
	for _, b := range c.Blocks {
		out += fmt.Sprintf("\n  %s %s: %d lines, %.1fpt", b.Path, b.Type, b.Lines, b.Need)
		if b.Overflow > 0 {
			out += fmt.Sprintf(", overflows by %.1fpt", b.Overflow)
		}
	}
	return out
}
