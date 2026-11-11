// ```process・```orgchart・```matrix・```funnel のコードブロックの中身(YAML の
// 部分集合)を、IR の data(schema/subsystems/<名前>.schema.json)にする。
//
// キーは IR と同じ。文の値は本文と同じ文中の記法で読み、YAML の列で書くと各行を
// 意味のある改行(br)でつなぐ。ここで断るのは Schema と同じ形の誤り(知らない
// キー・型・知らない状態の名前)だけ。matrix の行のセルの数が列の数と合わない
// ことは、boxloom check が matrix-cells の誤り(その行の行つき)で返す。

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
		gField{"members", gSeq(gObject(figureNodeFields), 0), false})
	matrixCellFields = []gField{{"status", status, true}, {"text", gInline, false}}
	matrixRowFields = []gField{
		{"label", gInline, true},
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
		{"rows", gSeq(gObject(matrixRowFields), 1), true},
		{"legend", gStatusLegend, false},
		{"emphasis", gString, false},
	})(p, n, path)
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
