// ```process・```orgchart・```matrix・```funnel・```phases・```cycle・```pillars のコードブロックの中身(YAML の
// 部分集合)を、IR の data(schema/subsystems/<名前>.schema.json)にする。
//
// キーは IR と同じ。文の値は本文と同じ文中の記法で読み、YAML の列で書くと各行を
// 意味のある改行(br)でつなぐ。ここで断るのは Schema と同じ形の誤り(知らない
// キー・型・知らない状態の名前)だけ。matrix の行のセルの数が列の数と合わない
// ことは、boxloom check が matrix-cells の誤り(その行の行つき)で、列の群の span の
// 合計が列の数と違うことは matrix-column-groups で返す。

package boxloom

import (
	"fmt"
	"strconv"
)

var (
	figureNodeFields []gField
	orgUnitFields    []gField
	matrixRowFields  []gField
	matrixCellFields []gField
	funnelStageField []gField
)

func init() {
	status := gEnum(figureStatuses)
	figureNodeFields = []gField{
		{"title", gInline, true}, {"detail", gInline, false},
		{"status", status, false}, {"emphasis", gBool, false},
	}
	orgUnitFields = append(append([]gField{}, figureNodeFields...),
		gField{"members", gSeq(gObject(figureNodeFields), 0), false},
		gField{"assistant", gObject([]gField{{"title", gInline, true}, {"detail", gInline, false}}), false})
	matrixCellFields = []gField{{"status", status, true}, {"text", gInline, false}}
	matrixRowFields = []gField{
		{"label", gInline, true},
		{"detail", gPhaseDetail, false},
		{"cells", gSeq(gObject(matrixCellFields), 1), true},
	}
	funnelStageField = []gField{
		{"label", gInline, true}, {"value", gFunnelValue, true}, {"note", gInline, false},
		{"status", status, false}, {"emphasis", gBool, false},
	}
}

func processDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject([]gField{
		{"caption", gInline, false},
		{"numbered", gBool, false},
		{"steps", gSeq(gObject(figureNodeFields), 2), true},
		{"legend", gStatusLegend, false},
	})(p, n, path)
}

func orgChartDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject([]gField{
		{"root", gObject(figureNodeFields), true},
		{"units", gSeq(gObject(orgUnitFields), 1), true},
		{"legend", gStatusLegend, false},
	})(p, n, path)
}

func matrixDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject([]gField{
		{"columns", gSeq(gInline, 1), true},
		{"columnGroups", gSeq(gObject([]gField{{"title", gInline, true}, {"span", gCount, true}}), 1), false},
		{"rows", gSeq(gObject(matrixRowFields), 1), true},
		{"legend", gStatusLegend, false},
		{"emphasis", gString, false},
	})(p, n, path)
}

func cycleDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject([]gField{
		{"steps", gSeqMax(gSeq(gObject(figureNodeFields), cycleMinSteps), cycleMaxSteps), true},
		{"start", gCycleStart, false},
		{"legend", gStatusLegend, false},
	})(p, n, path)
}

func pillarsDataFromYAML(p *mdParser, n *yNode, path string) any {
	pillar := []gField{
		{"title", gInline, true},
		{"items", gSeq(gInline, 1), false},
		{"label", gPillarLabel, false},
	}
	return gObject([]gField{
		{"roof", gInline, true},
		{"pillars", gSeqMax(gSeq(gObject(pillar), pillarsMin), pillarsMax), true},
		{"foundation", gInline, false},
	})(p, n, path)
}

// gPillarLabel は柱の短い名前(1〜2字の見える字。数字だけでも文字列)。
func gPillarLabel(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "名前")
	if !ok {
		return nil
	}
	if err := checkPillarLabel(s); err != nil {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("柱の名前 %q は書けません(見える字を含む %d 字まで)", s, pillarLabelMaxRunes), nil,
			"柱の上の小さな丸に入る名前です。長い名前は title に書いてください")
		return nil
	}
	return s
}

// gSeqMax は列の項目が max を越えたら誤りにする(項目の読み方は seq)。
func gSeqMax(seq ganttConv, max int) ganttConv {
	return func(p *mdParser, n *yNode, path string) any {
		if n.kind == 'q' && len(n.items) > max {
			gFail(p, n, path, codeInvalidValue, fmt.Sprintf("項目は %d つまでです(%d つ書かれています)", max, len(n.items)), nil, "")
		}
		return seq(p, n, path)
	}
}

// gCycleStart は cycle の番号の始まり(0 か 1)。
func gCycleStart(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "値")
	if !ok {
		return nil
	}
	if !n.quoted && (s == "0" || s == "1") {
		return float64(s[0] - '0')
	}
	gFail(p, n, path, codeInvalidValue, fmt.Sprintf("start %q は 0 か 1 で書いてください", s), []string{"0", "1"}, "")
	return nil
}

func funnelDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject([]gField{
		{"unit", gString, false},
		{"stages", gSeq(gObject(funnelStageField), 2), true},
		{"legend", gStatusLegend, false},
	})(p, n, path)
}

// gFunnelValue は数(引用しない)か文字列。
func gFunnelValue(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "値")
	if !ok {
		return nil
	}
	if !n.quoted {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	if s == "" {
		gFail(p, n, path, codeInvalidValue, "空の値は書けません", nil, "")
		return nil
	}
	return s
}

// gStatusLegend は凡例: キーは状態の名前、値は文。書いた順を保つ。
func gStatusLegend(p *mdParser, n *yNode, path string) any {
	if n.kind != 'm' {
		gFail(p, n, path, codeInvalidValue, "凡例は「状態の名前: 説明」の対応表で書いてください", nil, "")
		return nil
	}
	out := &jobj{}
	for k, key := range n.keys {
		if !contains(figureStatuses, key) {
			p.fail(n.keyLine[k], n.vals[k].col, path, codeUnknownKey, fmt.Sprintf("凡例のキー %s はありません", key), figureStatuses, "")
			continue
		}
		if v := gInline(p, n.vals[k], path+"/"+key); v != nil {
			out.set(key, v)
		}
	}
	return out
}

// phasesDataFromYAML は ```phases の中身。箱の detail は文か文の列(列は箇条)。
func phasesDataFromYAML(p *mdParser, n *yNode, path string) any {
	status := gEnum(figureStatuses)
	item := []gField{
		{"title", gInline, true}, {"detail", gPhaseDetail, false},
		{"status", status, false}, {"emphasis", gBool, false},
	}
	step := []gField{
		{"id", gID, false}, {"title", gInline, true}, {"period", gInline, false},
		{"items", gSeq(gObject(item), 1), true},
	}
	group := []gField{{"title", gInline, true}, {"from", gID, true}, {"to", gID, true}}
	return gObject([]gField{
		{"steps", gSeq(gObject(step), 2), true},
		{"groups", gSeq(gObject(group), 1), false},
		{"legend", gStatusLegend, false},
	})(p, n, path)
}

// gCount は1以上の整数(引用しない)。matrix の列の群の span。合計が列の数かは
// boxloom check が matrix-column-groups で見る。
func gCount(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "数")
	if !ok {
		return nil
	}
	k, err := strconv.Atoi(s)
	if err != nil || k < 1 || n.quoted {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("%s は1以上の整数で書いてください", s), nil, "例 span: 2")
		return nil
	}
	return k
}

// gPhaseDetail は1つの文か、文の列(各項目が箇条の1つ。gInline と違って改行でつながない)。
func gPhaseDetail(p *mdParser, n *yNode, path string) any {
	if n.kind == 'q' {
		return gSeq(gInline, 1)(p, n, path)
	}
	return gInline(p, n, path)
}
