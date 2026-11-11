// ```shapes のコードブロックの中身(YAML の部分集合)を、IR の shapes の data
// (schema/subsystems/shapes.schema.json)にする。
//
// キーは IR と同じ。数(座標・大きさ・aspect)は引用しない YAML の値で書き、
// 線の点は流れの書き方の列([0.1, 0.5])で書く。ここで断るのは Schema と同じ
// 形の誤り(知らないキー・0〜1 の外の数・16進の色・知らない字の段)だけ。
// x+w が 1 を超える・知らない役割名・無い id を指すコネクタは、boxloom check が
// 図形ごとの行と決まった code で返す(SourceMap の mark は gObject・gSeq が付ける)。
package main

import (
	"fmt"
	"strconv"
)

var shapeFieldsByKind map[string][]gField

func init() {
	kind := gChoice(shapeKindsAll, "図形の種類", "")
	boxed := func(kindConv ganttConv, extra ...gField) []gField {
		return append([]gField{
			{"id", gID, false}, {"kind", kindConv, true},
			{"x", gFrac, true}, {"y", gFrac, true}, {"w", gExtent, true}, {"h", gExtent, true},
		}, extra...)
	}
	style := gChoice(shapeTextStyles, "字の段", "文字の大きさは数ではなく、テンプレートの字の段の名前で書きます")
	shape := boxed(kind,
		gField{"fill", gRole, false}, gField{"line", gRole, false},
		gField{"text", gInline, false}, gField{"textStyle", style, false}, gField{"textColor", gRole, false})
	text := boxed(kind, gField{"text", gInline, true}, gField{"textStyle", style, false}, gField{"color", gRole, false})
	line := []gField{{"kind", kind, true}, {"from", gPoint, true}, {"to", gPoint, true}, {"color", gRole, false}}
	conn := []gField{{"kind", kind, true}, {"from", gID, true}, {"to", gID, true}, {"arrow", gBool, false}, {"color", gRole, false}}
	shapeFieldsByKind = map[string][]gField{
		"rect": shape, "roundRect": shape, "ellipse": shape, "text": text,
		"line": line, "arrow": line, "connector": conn,
	}
}

func shapesDataFromYAML(p *mdParser, n *yNode, path string) any {
	return gObject([]gField{
		{"aspect", gPositive, false},
		{"items", gSeq(gShapeItem, 1), true},
	})(p, n, path)
}

// gShapeItem は1つの図形・線。kind で書けるキーが変わる。
func gShapeItem(p *mdParser, n *yNode, path string) any {
	if n.kind != 'm' {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("ここには「キー: 値」の対応表を書いてください(%sが書かれています)", gKind(n)), nil, "")
		return nil
	}
	kind := ""
	for k, key := range n.keys {
		if key == "kind" && n.vals[k].kind == 's' {
			kind = n.vals[k].str
		}
	}
	fields, ok := shapeFieldsByKind[kind]
	if !ok {
		if kind == "" {
			gFail(p, n, path, codeMissingKey, "キー kind がありません", shapeKindsAll, "")
		} else {
			gFail(p, n, path, codeInvalidValue, fmt.Sprintf("図形の種類 %q はありません", kind), shapeKindsAll, "")
		}
		return nil
	}
	return gObject(fields)(p, n, path)
}

func gChoice(values []string, what, hint string) ganttConv {
	return func(p *mdParser, n *yNode, path string) any {
		s, ok := gScalar(p, n, path, what)
		if !ok {
			return nil
		}
		if contains(values, s) {
			return s
		}
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("%s %q はありません", what, s), values, hint)
		return nil
	}
}

// gNumber は引用しない数。
func gNumber(p *mdParser, n *yNode, path string) (float64, bool) {
	if n.kind != 's' || n.quoted {
		gFail(p, n, path, codeInvalidValue, "ここには数を書いてください(引用しない)", nil, "例 0.25")
		return 0, false
	}
	f, err := strconv.ParseFloat(n.str, 64)
	if err != nil {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("%q は数ではありません", n.str), nil, "例 0.25")
		return 0, false
	}
	return f, true
}

// gFrac は 0〜1 の相対の位置。
func gFrac(p *mdParser, n *yNode, path string) any {
	f, ok := gNumber(p, n, path)
	if !ok {
		return nil
	}
	if f < 0 || f > 1 {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("%v は 0〜1 の外です", f), nil, "座標は枠の内側に対する 0〜1 の相対で書きます")
		return nil
	}
	return f
}

// gExtent は 0 より大きく 1 以下の相対の大きさ。
func gExtent(p *mdParser, n *yNode, path string) any {
	f, ok := gNumber(p, n, path)
	if !ok {
		return nil
	}
	if f <= 0 || f > 1 {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("大きさ %v は 0 より大きく 1 以下にしてください", f), nil, "大きさは枠の内側に対する相対で書きます")
		return nil
	}
	return f
}

func gPositive(p *mdParser, n *yNode, path string) any {
	f, ok := gNumber(p, n, path)
	if !ok {
		return nil
	}
	if f <= 0 {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("%v は 0 より大きくしてください", f), nil, "")
		return nil
	}
	return f
}

// gPoint は相対の点 [x, y]。
func gPoint(p *mdParser, n *yNode, path string) any {
	if n.kind != 'q' || len(n.items) != 2 {
		gFail(p, n, path, codeInvalidValue, "点は [x, y] の2つの数で書いてください", nil, "例 from: [0.1, 0.5]")
		return nil
	}
	x, y := gFrac(p, n.items[0], path+"/0"), gFrac(p, n.items[1], path+"/1")
	if x == nil || y == nil {
		return nil
	}
	return []any{x, y}
}

// gRole は役割名。16進の色は断る。名前がテーマにあるかは boxloom check が見る。
func gRole(p *mdParser, n *yNode, path string) any {
	s, ok := gScalar(p, n, path, "役割名")
	if !ok {
		return nil
	}
	if !roleNameRe.MatchString(s) || hexColorLikeRe.MatchString(s) {
		gFail(p, n, path, codeInvalidValue, fmt.Sprintf("%q は役割名ではありません", s), themeRoles, "色はテーマの役割名で書きます。16進の色は書けません")
		return nil
	}
	return s
}
