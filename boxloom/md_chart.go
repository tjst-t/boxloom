// ```chart のコードブロックの中身(YAML の部分集合)を、IR の data
// (schema/subsystems/chart.schema.json)にする。
//
// キーは IR と同じ。ここで断るのは Schema と同じ形の誤り(知らないキー・型・知らない
// 比べ方)だけ。比べ方とデータの組み合わせ(値の数が項目の数と合わない・強調する
// 項目が無いなど)は、boxloom check が chart-values・chart-emphasis の誤り(その行
// つき)で返す。

package boxloom

var (
	chartDataFields   []gField
	chartSeriesFields []gField
	chartAxisFields   []gField
)

func init() {
	chartAxisFields = []gField{{"title", gString, true}, {"unit", gString, false}}
	chartSeriesFields = []gField{
		{"name", gString, true},
		{"values", gSeq(gChartNumber, 1), false},
		{"points", gSeq(gChartPoint, 1), false},
		{"emphasis", gBool, false},
	}
	chartDataFields = []gField{
		{"compare", gChoice(chartCompares, "比べ方", "比べ方は item・timeSeries・component・frequency・correlation のどれか。グラフの形はサブシステムが選びます"), true},
		{"categories", gSeq(gString, 1), false},
		{"series", gSeq(gObject(chartSeriesFields), 1), true},
		{"stacked", gBool, false},
		{"unit", gString, false},
		{"emphasis", gSeq(gString, 1), false},
		{"xAxis", gObject(chartAxisFields), false},
		{"yAxis", gObject(chartAxisFields), false},
	}
}

func chartDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject(chartDataFields)(p, n, path)
}

// gChartNumber は引用しない数(負の数・小数も書ける)。
func gChartNumber(p *mdParser, n *yNode, path string) any {
	f, ok := gNumber(p, n, path)
	if !ok {
		return nil
	}
	return f
}

// gChartPoint は散布図の点 [x, y](どちらも数)。
func gChartPoint(p *mdParser, n *yNode, path string) any {
	if n.kind != 'q' || len(n.items) != 2 {
		gFail(p, n, path, codeInvalidValue, "点は [x, y] の2つの数で書いてください", nil, "例 points: [[1, 9], [2, 8]]")
		return nil
	}
	x, y := gChartNumber(p, n.items[0], path+"/0"), gChartNumber(p, n.items[1], path+"/1")
	if x == nil || y == nil {
		return nil
	}
	return []any{x, y}
}
