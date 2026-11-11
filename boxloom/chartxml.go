// chartxml.go: PowerPoint の本物のグラフ(c:chartSpace)と、その値を持つ埋め込みの
// xlsx を、標準ライブラリだけで書く。グラフの見た目(色・字の大きさ)は呼ぶ側が
// 解決済みの値で渡す(役割名の解決はサブシステムの仕事)。
//
// 書く部品(1つのグラフにつき):
//
//	ppt/charts/chartN.xml                       c:chartSpace
//	ppt/charts/_rels/chartN.xml.rels            → ../embeddings/Microsoft_Excel_Worksheet{N}.xlsx(package)
//	ppt/embeddings/Microsoft_Excel_Worksheet{N}.xlsx
//
// 埋め込みの表の形:A1 は空、B1.. に系列の名前、A2.. に項目の名前、B2.. に値。
// 散布図は系列ごとに X と Y の2列(1行目は X の軸の題と系列の名前)。
// PowerPoint・LibreOffice で「データの編集」を開くとこの表が出る。
package boxloom

import (
	"archive/zip"
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ChartKind はグラフの描き方(OOXML の種類)。
type ChartKind string

const (
	ChartColumn        ChartKind = "column"        // 縦棒(項目の比較・時系列の少ない点)
	ChartBar           ChartKind = "bar"           // 横棒(順位・項目の比較)
	ChartStackedColumn ChartKind = "stackedColumn" // 積み上げ縦棒(構成の推移)
	ChartLine          ChartKind = "line"          // 折れ線(時系列)
	ChartPie           ChartKind = "pie"           // 円(構成比。系列は1つ)
	ChartScatter       ChartKind = "scatter"       // 散布図(相関。項目の名前は X の値)
	ChartStackedBar    ChartKind = "stackedBar"    // 積み上げ横棒(項目ごとの内訳)
	ChartHistogram     ChartKind = "histogram"     // ヒストグラム(度数分布。棒の間を詰めた縦棒)
)

// ChartSeries は1つの系列。Color は16進(解決済み)。PointColors は項目ごとの色
// (強調する項目、円の扇)。
type ChartSeries struct {
	Name        string
	Values      []float64
	XValues     []float64 // 散布図だけ
	Color       string
	PointColors map[int]string
}

// ChartData は1つのグラフの中身と見た目(解決済み)。0 の寸法は既定の値
// (GapWidth 60・LineWidth 2.25・MarkerSize 5・AxisLineWidth 0.75・GridLineWidth 0.5・
// PieOutlineWidth 1.5)。
type ChartData struct {
	Kind       ChartKind
	Categories []string
	Series     []ChartSeries
	NumFmt     string // 値の書式(Excel の書式。空なら "General")
	XNumFmt    string // 散布図の X の書式(空なら "General")
	FontSize   float64
	TextColor  string
	GridColor  string
	AxisColor  string
	Legend     bool
	DataLabels bool
	Latin, EA  string // 字体(空ならテーマの字体)

	XTitle, YTitle string // 軸の題(散布図。空なら書かない)

	GapWidth        float64 // 棒の間(棒の幅に対する %)
	LineWidth       float64 // 折れ線の太さ(pt)
	MarkerSize      float64 // 点の大きさ(pt、2〜72)
	AxisLineWidth   float64
	GridLineWidth   float64
	PieOutline      string // 円の扇の境の色(背景の色にする)。空なら FFFFFF
	PieOutlineWidth float64
}

func orDefault(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

// isBar は棒の形(c:barChart)で書く種類か。
func (k ChartKind) isBar() bool {
	switch k {
	case ChartColumn, ChartBar, ChartStackedColumn, ChartStackedBar, ChartHistogram:
		return true
	}
	return false
}

// horizontal は項目の軸が縦(左)に立つ種類か。
func (k ChartKind) horizontal() bool { return k == ChartBar || k == ChartStackedBar }

func (k ChartKind) stacked() bool { return k == ChartStackedColumn || k == ChartStackedBar }

const (
	nsC = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	nsA = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsR = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// col は 0 始まりの列の番号を Excel の列の名前にする(0→A)。
func xlCol(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+i%26)) + s
		i = i/26 - 1
	}
	return s
}

func fmtNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(v, 'g', 15, 64)
}

func solidFill(hex string) string {
	return `<a:solidFill><a:srgbClr val="` + hexColor(hex) + `"/></a:solidFill>`
}

// txPr は字の大きさ・色・字体を持つ文字の書式(c:txPr)。
func (d *ChartData) txPr() string { return d.txPrBody(`<a:bodyPr/>`) }

// labelTxPr は値のラベルの書式。折り返さない(wrap="none")。アプリは値のラベルの
// 幅を自分で決めて折り返すので(LibreOffice は「2,859 億ド/ル」のように単位の途中で
// 折る)、数と単位を1行に保つ。
func (d *ChartData) labelTxPr() string { return d.txPrBody(`<a:bodyPr wrap="none"/>`) }

func (d *ChartData) txPrBody(bodyPr string) string {
	font := d.fontRefs()
	return fmt.Sprintf(`<c:txPr>%s<a:lstStyle/><a:p><a:pPr><a:defRPr sz="%d" b="0">%s%s</a:defRPr></a:pPr><a:endParaRPr lang="ja-JP"/></a:p></c:txPr>`,
		bodyPr, int(math.Round(d.FontSize*100)), solidFill(d.TextColor), font)
}

func lineProps(hex string, wPt float64) string {
	return fmt.Sprintf(`<c:spPr><a:ln w="%d">%s</a:ln></c:spPr>`, ptToEMU(wPt), solidFill(hex))
}

// strRef は表の範囲を指す文字の参照と、その写し(cache)。
func strRef(ref string, vals []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:strRef><c:f>%s</c:f><c:strCache><c:ptCount val="%d"/>`, ref, len(vals))
	for i, v := range vals {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, escapeXML(v))
	}
	b.WriteString(`</c:strCache></c:strRef>`)
	return b.String()
}

func numRef(ref, format string, vals []float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:numRef><c:f>%s</c:f><c:numCache><c:formatCode>%s</c:formatCode><c:ptCount val="%d"/>`, ref, escapeXML(format), len(vals))
	for i, v := range vals {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, fmtNum(v))
	}
	b.WriteString(`</c:numCache></c:numRef>`)
	return b.String()
}

func (d *ChartData) numFmt() string {
	if d.NumFmt == "" {
		return "General"
	}
	return d.NumFmt
}

// ChartXML は c:chartSpace の文書。rid は埋め込みの xlsx への関係の Id。
func (d *ChartData) ChartXML(rid string) string {
	n := len(d.Categories)
	catRef := fmt.Sprintf("Sheet1!$A$2:$A$%d", n+1)
	var sers strings.Builder
	for si, s := range d.Series {
		col := xlCol(si + 1)
		tx := `<c:tx>` + strRef(fmt.Sprintf("Sheet1!$%s$1", col), []string{s.Name}) + `</c:tx>`
		valRef := fmt.Sprintf("Sheet1!$%s$2:$%s$%d", col, col, n+1)
		var dpts strings.Builder
		for i := 0; i < n; i++ {
			c, ok := s.PointColors[i]
			if !ok {
				continue
			}
			fmt.Fprintf(&dpts, `<c:dPt><c:idx val="%d"/>`, i)
			if d.Kind != ChartPie {
				dpts.WriteString(`<c:invertIfNegative val="0"/>`)
			}
			fmt.Fprintf(&dpts, `<c:bubble3D val="0"/><c:spPr>%s<a:ln><a:noFill/></a:ln></c:spPr></c:dPt>`, solidFill(c))
		}
		dl := d.dLbls()
		switch d.Kind {
		case ChartLine:
			fmt.Fprintf(&sers, `<c:ser><c:idx val="%d"/><c:order val="%d"/>%s<c:spPr><a:ln w="%d" cap="rnd">%s<a:round/></a:ln></c:spPr><c:marker><c:symbol val="circle"/><c:size val="%d"/><c:spPr>%s<a:ln><a:noFill/></a:ln></c:spPr></c:marker>%s<c:cat>%s</c:cat><c:val>%s</c:val><c:smooth val="0"/></c:ser>`,
				si, si, tx, ptToEMU(orDefault(d.LineWidth, 2.25)), solidFill(s.Color), d.markerSize(), solidFill(s.Color), dl, strRef(catRef, d.Categories), numRef(valRef, d.numFmt(), s.Values))
		case ChartPie:
			outline := d.PieOutline
			if outline == "" {
				outline = "FFFFFF"
			}
			fmt.Fprintf(&sers, `<c:ser><c:idx val="%d"/><c:order val="%d"/>%s<c:spPr><a:ln w="%d">%s</a:ln></c:spPr>%s%s<c:cat>%s</c:cat><c:val>%s</c:val></c:ser>`,
				si, si, tx, ptToEMU(orDefault(d.PieOutlineWidth, 1.5)), solidFill(outline), dpts.String(), dl, strRef(catRef, d.Categories), numRef(valRef, d.numFmt(), s.Values))
		case ChartScatter:
			// 散布図は系列ごとに X と Y の2列(A・B が1つ目、C・D が2つ目…)。
			n := len(s.Values)
			xc, yc := xlCol(2*si), xlCol(2*si+1)
			tx = `<c:tx>` + strRef(fmt.Sprintf("Sheet1!$%s$1", yc), []string{s.Name}) + `</c:tx>`
			valRef := fmt.Sprintf("Sheet1!$%s$2:$%s$%d", yc, yc, n+1)
			xRef := fmt.Sprintf("Sheet1!$%s$2:$%s$%d", xc, xc, n+1)
			fmt.Fprintf(&sers, `<c:ser><c:idx val="%d"/><c:order val="%d"/>%s<c:spPr><a:ln><a:noFill/></a:ln></c:spPr><c:marker><c:symbol val="circle"/><c:size val="%d"/><c:spPr>%s<a:ln><a:noFill/></a:ln></c:spPr></c:marker>%s%s<c:xVal>%s</c:xVal><c:yVal>%s</c:yVal><c:smooth val="0"/></c:ser>`,
				si, si, tx, d.markerSize(), solidFill(s.Color), dpts.String(), dl, numRef(xRef, d.xNumFmt(), s.XValues), numRef(valRef, d.numFmt(), s.Values))
		default:
			fmt.Fprintf(&sers, `<c:ser><c:idx val="%d"/><c:order val="%d"/>%s<c:spPr>%s<a:ln><a:noFill/></a:ln></c:spPr><c:invertIfNegative val="0"/>%s%s<c:cat>%s</c:cat><c:val>%s</c:val></c:ser>`,
				si, si, tx, solidFill(s.Color), dpts.String(), dl, strRef(catRef, d.Categories), numRef(valRef, d.numFmt(), s.Values))
		}
	}

	var plot string
	switch d.Kind {
	case ChartPie:
		plot = `<c:pieChart><c:varyColors val="1"/>` + sers.String() + `<c:firstSliceAng val="0"/></c:pieChart>`
	case ChartLine:
		plot = `<c:lineChart><c:grouping val="standard"/><c:varyColors val="0"/>` + sers.String() +
			`<c:marker val="1"/><c:axId val="1001"/><c:axId val="1002"/></c:lineChart>` + d.catAx("b") + d.valAx("l")
	case ChartScatter:
		plot = `<c:scatterChart><c:scatterStyle val="lineMarker"/><c:varyColors val="0"/>` + sers.String() +
			`<c:axId val="1001"/><c:axId val="1002"/></c:scatterChart>` + d.valAxID(1001, 1002, "b", false, d.XTitle, d.xNumFmt()) + d.valAxID(1002, 1001, "l", true, d.YTitle, d.numFmt())
	default:
		dir, grouping, overlap := "col", "clustered", ""
		if d.Kind.horizontal() {
			dir = "bar"
		}
		if d.Kind.stacked() {
			grouping, overlap = "stacked", `<c:overlap val="100"/>`
		}
		catPos, valPos := "b", "l"
		if dir == "bar" {
			catPos, valPos = "l", "b"
		}
		gap := int(math.Round(orDefault(d.GapWidth, 60)))
		plot = `<c:barChart><c:barDir val="` + dir + `"/><c:grouping val="` + grouping + `"/><c:varyColors val="0"/>` + sers.String() +
			fmt.Sprintf(`<c:gapWidth val="%d"/>`, gap) + overlap + `<c:axId val="1001"/><c:axId val="1002"/></c:barChart>` + d.catAx(catPos) + d.valAx(valPos)
	}

	legend := ""
	if d.Legend {
		legend = `<c:legend><c:legendPos val="b"/><c:overlay val="0"/>` + d.txPr() + `</c:legend>`
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="` + nsC + `" xmlns:a="` + nsA + `" xmlns:r="` + nsR + `"><c:date1904 val="0"/><c:lang val="ja-JP"/><c:roundedCorners val="0"/>` +
		`<c:chart><c:autoTitleDeleted val="1"/><c:plotArea><c:layout/>` + plot + `<c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr></c:plotArea>` + legend +
		`<c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart>` +
		`<c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr>` + d.txPr() +
		`<c:externalData r:id="` + rid + `"><c:autoUpdate val="0"/></c:externalData></c:chartSpace>`
}

func (d *ChartData) dLbls() string {
	if !d.DataLabels {
		return ""
	}
	pos := ""
	switch d.Kind {
	case ChartPie:
		pos = `<c:dLblPos val="outEnd"/>`
	case ChartColumn, ChartBar, ChartHistogram:
		pos = `<c:dLblPos val="outEnd"/>`
	case ChartLine:
		pos = `<c:dLblPos val="t"/>`
	}
	cat := "0"
	if d.Kind == ChartPie {
		cat = "1"
	}
	sep := ""
	if cat == "1" {
		sep = "<c:separator>\n</c:separator>" // 項目の名前と値を2行に
	}
	return `<c:dLbls><c:numFmt formatCode="` + escapeXML(d.numFmt()) + `" sourceLinked="0"/><c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr>` + d.labelTxPr() + pos +
		`<c:showLegendKey val="0"/><c:showVal val="1"/><c:showCatName val="` + cat + `"/><c:showSerName val="0"/><c:showPercent val="0"/><c:showBubbleSize val="0"/>` + sep + `</c:dLbls>`
}

func (d *ChartData) catAx(pos string) string {
	orient := "minMax"
	if d.Kind.horizontal() {
		orient = "maxMin" // 横棒は上から項目の順に並べる
	}
	return `<c:catAx><c:axId val="1001"/><c:scaling><c:orientation val="` + orient + `"/></c:scaling><c:delete val="0"/><c:axPos val="` + pos + `"/>` +
		`<c:numFmt formatCode="General" sourceLinked="0"/><c:majorTickMark val="none"/><c:minorTickMark val="none"/><c:tickLblPos val="low"/>` +
		lineProps(d.AxisColor, orDefault(d.AxisLineWidth, 0.75)) + d.txPr() + `<c:crossAx val="1002"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/><c:noMultiLvlLbl val="0"/></c:catAx>`
}

func (d *ChartData) valAx(pos string) string {
	return d.valAxID(1002, 1001, pos, true, "", d.numFmt())
}

func (d *ChartData) markerSize() int {
	return int(math.Max(2, math.Min(72, math.Round(orDefault(d.MarkerSize, 5)))))
}

func (d *ChartData) xNumFmt() string {
	if d.XNumFmt == "" {
		return "General"
	}
	return d.XNumFmt
}

// axisTitle は軸の題(c:title)。縦の軸(pos が l)の題は 90 度回す。
func (d *ChartData) axisTitle(text, pos string) string {
	if text == "" {
		return ""
	}
	rot := ""
	if pos == "l" || pos == "r" {
		rot = ` rot="-5400000" vert="horz"`
	}
	return fmt.Sprintf(`<c:title><c:tx><c:rich><a:bodyPr%s/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="%d" b="0"/></a:pPr><a:r><a:rPr lang="ja-JP" sz="%d" b="0">%s</a:rPr><a:t>%s</a:t></a:r></a:p></c:rich></c:tx><c:overlay val="0"/></c:title>`,
		rot, int(math.Round(d.FontSize*100)), int(math.Round(d.FontSize*100)), solidFill(d.TextColor)+d.fontRefs(), escapeXML(text))
}

func (d *ChartData) fontRefs() string {
	font := ""
	if d.Latin != "" {
		font += `<a:latin typeface="` + escapeXML(d.Latin) + `"/>`
	}
	if d.EA != "" {
		font += `<a:ea typeface="` + escapeXML(d.EA) + `"/>`
	}
	return font
}

func (d *ChartData) valAxID(id, cross int, pos string, grid bool, title, numFmt string) string {
	g := ""
	if grid {
		g = `<c:majorGridlines>` + lineProps(d.GridColor, orDefault(d.GridLineWidth, 0.5)) + `</c:majorGridlines>`
	}
	crosses := "autoZero"
	if d.Kind.horizontal() {
		// 横棒は項目の軸を逆向き(上から順)にしているので、値の軸を下に置くには
		// 項目の軸の最大の側で交わらせる。
		crosses = "max"
	}
	between := `<c:crossBetween val="between"/>`
	if d.Kind == ChartScatter {
		between = `<c:crossBetween val="midCat"/>`
	}
	return fmt.Sprintf(`<c:valAx><c:axId val="%d"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="%s"/>%s%s`+
		`<c:numFmt formatCode="%s" sourceLinked="0"/><c:majorTickMark val="none"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/>`+
		`<c:spPr><a:ln><a:noFill/></a:ln></c:spPr>%s<c:crossAx val="%d"/><c:crosses val="%s"/>%s</c:valAx>`,
		id, pos, g, d.axisTitle(title, pos), escapeXML(numFmt), d.txPr(), cross, crosses, between)
}

// XLSX は値を持つ埋め込みの表(最小の xlsx)。文字は inlineStr で書き、共有文字列の
// 部品は持たない。
func (d *ChartData) XLSX() ([]byte, error) {
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	str := func(ref, v string) string {
		return `<c r="` + ref + `" t="inlineStr"><is><t>` + escapeXML(v) + `</t></is></c>`
	}
	num := func(ref string, v float64) string {
		return `<c r="` + ref + `"><v>` + fmtNum(v) + `</v></c>`
	}
	if d.Kind == ChartScatter {
		// 系列ごとに X と Y の2列。1行目は X の題と系列の名前。
		rows := 0
		sheet.WriteString(`<row r="1">`)
		for si, s := range d.Series {
			xt := d.XTitle
			if xt == "" {
				xt = "X"
			}
			sheet.WriteString(str(xlCol(2*si)+"1", xt) + str(xlCol(2*si+1)+"1", s.Name))
			rows = max(rows, len(s.Values))
		}
		sheet.WriteString(`</row>`)
		for i := 0; i < rows; i++ {
			r := strconv.Itoa(i + 2)
			fmt.Fprintf(&sheet, `<row r="%s">`, r)
			for si, s := range d.Series {
				if i < len(s.Values) && i < len(s.XValues) {
					sheet.WriteString(num(xlCol(2*si)+r, s.XValues[i]) + num(xlCol(2*si+1)+r, s.Values[i]))
				}
			}
			sheet.WriteString(`</row>`)
		}
	} else {
		sheet.WriteString(`<row r="1">`)
		for si, s := range d.Series {
			sheet.WriteString(str(xlCol(si+1)+"1", s.Name))
		}
		sheet.WriteString(`</row>`)
		for i := range d.Categories {
			r := strconv.Itoa(i + 2)
			fmt.Fprintf(&sheet, `<row r="%s">`, r)
			sheet.WriteString(str("A"+r, d.Categories[i]))
			for si, s := range d.Series {
				if i < len(s.Values) {
					sheet.WriteString(num(xlCol(si+1)+r, s.Values[i]))
				}
			}
			sheet.WriteString(`</row>`)
		}
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	files := []struct{ name, body string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="` + nsR + `"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs></styleSheet>`},
		{"xl/worksheets/sheet1.xml", sheet.String()},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// chartFrameXML はスライドの上のグラフの枠(p:graphicFrame)。
func chartFrameXML(id int, box LBox, rid string) string {
	return fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Chart %d"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`+
		`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`+
		`<a:graphic><a:graphicData uri="%s"><c:chart xmlns:c="%s" r:id="%s"/></a:graphicData></a:graphic></p:graphicFrame>
`, id, id, ptToEMU(box.X), ptToEMU(box.Y), ptToEMU(box.Width), ptToEMU(box.Height), nsC, nsC, rid)
}
