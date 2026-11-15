package boxloom

import (
	"strings"
	"testing"
)

// 狭い列に図を置いてはみ出したスライド(#161 の評価の R4・P4。examples/eval の誤った版から)。
const (
	narrowPhasesDeck = "---\ntitle: 架空の館の見直し\ntemplate: report\n---\n\n" + "# 移り方の3つの段\n\n:::lead\n3年かけて、準備・受け取りの館・学びの館の順に移る。各段の終わりに利用を数え、次の段の中身を直す。\n:::\n\n::::columns\n:::column 段ごとの要点 {role=main}\n- 1年目は選書を中央館に集め、4つの地区で説明会を開く\n- 2年目は北館と東館を受け取りの館に移し、浮いた人を夜に回す\n- 3年目は西館と南館を学びの館に移す\n:::\n:::column\n```phases\nsteps:\n  - id: p1\n    title: 準備\n    period: 1年目\n    items:\n      - title: 選書を集める\n        detail:\n          - 中央館に選書の係を置く\n          - 館ごとの棚の型を決める\n      - title: 説明会\n        detail: 4つの地区で2回ずつ\n  - id: p2\n    title: 受け取りの館\n    period: 2年目\n    items:\n      - title: 北館と東館を移す\n        detail:\n          - 開館を11時から20時に\n          - 受け取りの棚を置く\n        emphasis: true\n      - title: 人手を移す\n        detail: 浮いた6人を夜と週末に\n  - id: p3\n    title: 学びの館\n    period: 3年目\n    items:\n      - title: 西館と南館を移す\n        detail:\n          - 学習の席を倍にする\n          - 相談の窓口を置く\n      - title: 司書の研修\n        detail: 年4回の研修\n        status: conditional\ngroups:\n  - title: 体制を整える\n    from: p1\n    to: p1\n  - title: 館を移す\n    from: p2\n    to: p3\nlegend:\n  conditional: 研修の予算が付くときだけ\n```\n:::\n::::\n\n:::source\n作業部会の案(2026年10月)\n:::\n\n"
	narrowMatrixDeck = "---\ntitle: 架空の町の提案\ntemplate: dark\n---\n\n" + "# 広げ方の案\n\n:::lead\n3つの案のうち、地区ごとに段階で広げる案だけが、費用と利用の両方の条件を満たす。\n:::\n\n::::columns\n:::column 読み取り {role=main}\n- 案Bだけが4つの条件を全て満たす\n- 案Aは費用と運営の負担で外れる\n- 案Cは利用の伸びと北の地区で外れる\n:::\n:::column\n```matrix\ncolumns: [案A 一斉に広げる, 案B 地区ごとに広げる, 案C 試行のまま]\nemphasis: 案B 地区ごとに広げる\nrows:\n  - label: 費用\n    detail: 3年の総額\n    cells:\n      - {status: danger, text: 9億円}\n      - {status: support, text: 5億円}\n      - {status: support, text: 2億円}\n  - label: 利用の伸び\n    detail: 3年後の月の利用\n    cells:\n      - {status: support, text: 8万回}\n      - {status: support, text: 7万回}\n      - {status: danger, text: 3万回}\n  - label: 運営の負担\n    detail: 運ぶ便と点検の人手\n    cells:\n      - {status: danger, text: 一度に倍}\n      - {status: conditional, text: 年ごとに増やす}\n      - {status: support, text: 今のまま}\n  - label: 北の地区\n    detail: 坂の多い地区への対応\n    cells:\n      - {status: conditional, text: 電動は一部}\n      - {status: support, text: 電動から入れる}\n      - {status: danger, text: 対応しない}\nlegend:\n  support: 条件を満たす\n  conditional: 条件つき\n  danger: 満たさない\n```\n:::\n::::\n\n:::source\n費用は2026年10月の見積もり、利用は試行の伸びからの推計\n:::\n\n"
)

// 列の中の図が列の幅に押されて縦に伸びてはみ出したとき、hint は dense を勧めず、
// 図を列から出す・図の列を広げることを言う。容量にも図の名前と全幅の高さが出る(#163)。
func TestColumnFigureOverflowHint(t *testing.T) {
	for _, c := range []struct{ deck, fig string }{
		{narrowPhasesDeck, "phases"},
		{narrowMatrixDeck, "matrix"},
	} {
		res := Check("x.bx.md", []byte(c.deck), CheckOptions{})
		hints := overflowHints(res)
		if len(hints) == 0 {
			t.Fatalf("%s: no layout-overflow: %v", c.fig, res.Warnings)
		}
		for _, h := range hints {
			if strings.Contains(h, "{density=dense}") || strings.Contains(h, "dense にしても") {
				t.Errorf("%s: hint suggests dense: %s", c.fig, h)
			}
			for _, want := range []string{"(" + c.fig + ")", "columns から出して全幅に置く", "role=main にして広げて"} {
				if !strings.Contains(h, want) {
					t.Errorf("%s: hint lacks %q: %s", c.fig, want, h)
				}
			}
		}
		found := false
		for _, sc := range res.Capacity {
			for _, b := range sc.Blocks {
				for _, col := range b.Columns {
					for _, ib := range col.Blocks {
						if ib.Subsystem == c.fig && ib.FullWidthNeed != nil && *ib.FullWidthNeed < ib.Need {
							found = true
						}
					}
				}
			}
		}
		if !found {
			t.Errorf("%s: capacity lacks subsystem/fullWidthNeed", c.fig)
		}
	}
}

// columns の無いスライドのはみ出しには、図の hint は出ない。
func TestColumnFigureHintOnlyInColumns(t *testing.T) {
	res := Check("x.bx.md", []byte(pointsDeck(40, "")), CheckOptions{})
	hints := overflowHints(res)
	if len(hints) == 0 {
		t.Fatal("no overflow")
	}
	for _, h := range hints {
		if strings.Contains(h, "columns から出して") {
			t.Errorf("figure hint on a slide without columns: %s", h)
		}
	}
}
