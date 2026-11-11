// PPTXBuilder: python-pptx を使わず、OOXML(zip + XML)を直接組み立てる最小実装。
//
// 自由に使える成熟したGo製pptxライブラリが無い(unidoc/unioffice は商用ライセンス、
// 他のOSS候補はshape作成すら未実装)ため、Goの標準ライブラリ(archive/zip,
// encoding/xml相当の文字列組み立て)だけでOOXMLを手書きする。サポート範囲は
// このデモが実際に必要とする最小限(角丸/矩形の塗り、テキストボックス、直線/
// コネクタ、ノート)に絞っている。

package boxloom

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"strings"
)

type shapeKind int

const (
	skRect shapeKind = iota
	skRoundRect
	skOval
	skText
	skLine
	skShape     // 図形(矩形・角丸・楕円)。枠線と中の文字を持てる(shapes サブシステム)
	skConnector // 矢印を付けられる線。図形の接続点に繋げられる(shapes サブシステム)
	skPicture   // 画像(p:pic。pptx_blocks.go)
	skTable     // 表(a:tbl。pptx_blocks.go)
	skChart     // 本物のグラフ(p:graphicFrame → c:chartSpace + 埋め込み xlsx)
)

type shape struct {
	id   int
	kind shapeKind
	// ph は文字の図形をプレースホルダーとして書く種類("title" | "ctrTitle" |
	// "subTitle"。空ならテキストボックス)。
	ph string

	box    LBox
	fill   string // "" = 塗りなし
	radius float64

	// 矩形の枠線(本物のrender_gantt()にある「破線の未確定区間」等で使う)。
	hasOutline     bool
	outlineColor   string
	outlineDash    string // "" | "dash"
	outlineWidthPt float64

	text     string
	fontSize float64
	bold     bool
	color    string
	align    string
	anchor   string
	runs     []TextRun // 空でなければ text・color・bold の代わりに使う
	wrap     bool      // true: このGo版の自前ロジックで幅に収まるよう折り返す。false: "\n"のみを境界として扱い、幅では折り返さない(本物のwrap=Falseに相当)。

	x1, y1, x2, y2 float64
	lineColor      string
	lineWidthPt    float64
	dash           string // "" | "dash"

	// skShape
	prst   string  // rect | roundRect | ellipse
	insetX float64 // 文字の枠の内側の余白
	insetY float64

	// skShape の2つ目以降の段落(題の下の補足など)と、その字の大きさ
	paras     [][]TextRun
	paraSizes []float64

	// skConnector
	elbow         bool    // 縦に出て横に曲がり、縦に入る線(bentConnector3)
	elbowAdj      float64 // 曲がる位置(始点からの縦の割合。0 なら 0.5)
	arrowEnd      bool    // to の端に矢印(a:tailEnd)
	stID, stIdx   int     // 始点を繋ぐ図形の id と接続点の番号(id 0 は繋がない)
	endID, endIdx int     // 終点を繋ぐ図形の id と接続点の番号

	// skPicture(text は代わりの文)。media・rid は書き出すときに決まる
	image      *imageAsset
	media, rid string

	// skTable
	table *TableDraw
	// skChart
	chart    *ChartData
	chartNum int    // 文書の中の通し番号(ppt/charts/chartN.xml)。書くときに決める
	chartRID string // スライドからグラフへの関係の Id
}

// PPTXBuilder は1枚のスライドを表す。Canvas(box_engine.pyのcanvas葉ノード)の
// レンダリング関数はこれを受け取り、確定したboxの中に自由に図形を追加する。
type PPTXBuilder struct {
	WidthPt, HeightPt float64
	Shapes            []shape
	Notes             string
	// Fonts は書き出す字体(テーマの a:latin・a:ea)と行の送りの決め方。
	// 作ったときの activeFonts(測った字体)を写す。
	Fonts *FontSet
	// ColorTheme はテーマの配色(theme1.xml の a:clrScheme)とマスターの clrMap を
	// 作る元(テンプレートの名前と値の表の色)。nil なら固定の既定の配色。
	ColorTheme *DeckColors
	nextID     int
}

// DeckColors は PPTX のテーマに写すテンプレートの名前と色(役割名 → 16進)。
type DeckColors struct {
	Name   string
	Colors map[string]string
}

func NewPPTXBuilder(widthPt, heightPt float64) *PPTXBuilder {
	return &PPTXBuilder{WidthPt: widthPt, HeightPt: heightPt, Fonts: activeFonts, nextID: 2}
}

func (p *PPTXBuilder) allocID() int {
	id := p.nextID
	p.nextID++
	return id
}

// AddRect は塗り付きの矩形(radius>0なら角丸)を追加する。枠線なし。
func (p *PPTXBuilder) AddRect(box LBox, fill string, radius float64) {
	p.AddRectFull(box, fill, radius, "", "", 0)
}

// AddRectFull は塗り・角丸・枠線すべてを指定できる矩形(本物の_add_rect()の
// フル機能版に相当)。fill=""なら塗りなし。outlineColor=""なら枠線なし。
func (p *PPTXBuilder) AddRectFull(box LBox, fill string, radius float64, outlineColor, outlineDash string, outlineWidthPt float64) {
	k := skRect
	if radius > 0 {
		k = skRoundRect
	}
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: k, box: box, fill: fill, radius: radius,
		hasOutline: outlineColor != "", outlineColor: outlineColor, outlineDash: outlineDash, outlineWidthPt: outlineWidthPt,
	})
}

// AddDot は塗りつぶされた円(本物の_add_dot()に相当。軸目盛りの点やopen_endの
// 連続ドットに使う)を追加する。
func (p *PPTXBuilder) AddDot(cx, cy, r float64, color string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skOval,
		box: LBox{X: cx - r, Y: cy - r, Width: r * 2, Height: r * 2}, fill: color,
	})
}

// AddChart は本物のグラフを box に置く。
func (p *PPTXBuilder) AddChart(box LBox, d *ChartData) {
	p.Shapes = append(p.Shapes, shape{id: p.allocID(), kind: skChart, box: box, chart: d})
}

// AddTextBox はテキストボックスを追加する(wrap=True相当)。テキストはこちら側で
// 幅に収まるよう事前に折り返し、PowerPoint/LibreOffice側の自動折り返しには
// 依存しない(box_engine.pyが計測した高さと実際の表示を一致させるため)。
func (p *PPTXBuilder) AddTextBox(box LBox, text string, fontSize float64, color string, bold bool, align, anchor string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skText, box: box,
		text: text, fontSize: fontSize, bold: bold, color: color, align: align, anchor: anchor, wrap: true,
	})
}

// AddTextBoxNoWrap はwrap=False相当。幅による折り返しは行わない(“\n”のみ
// 段落境界として扱う)。本物のrender_gantt()で凡例・軸目盛り・行ラベル・
// セグメントラベル等、固定幅に収まる前提の短いラベルに使われているのと同じ。
func (p *PPTXBuilder) AddTextBoxNoWrap(box LBox, text string, fontSize float64, color string, bold bool, align, anchor string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skText, box: box,
		text: text, fontSize: fontSize, bold: bold, color: color, align: align, anchor: anchor, wrap: false,
	})
}

// AddRichTextBox は装飾つきの文字(wrap=True 相当)。
func (p *PPTXBuilder) AddRichTextBox(box LBox, runs []TextRun, fontSize float64, align, anchor string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skText, box: box,
		text: runsText(runs), runs: runs, fontSize: fontSize, align: align, anchor: anchor, wrap: true,
	})
}

// AddLine は直線/コネクタを追加する。dash は "" (実線) または "dash"。
func (p *PPTXBuilder) AddLine(x1, y1, x2, y2, widthPt float64, color, dash string) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skLine,
		x1: x1, y1: y1, x2: x2, y2: y2, lineColor: color, lineWidthPt: widthPt, dash: dash,
	})
}

// ShapeOpts は AddShape の指定。Fill・Outline が "" なら塗り・枠線なし。
type ShapeOpts struct {
	Prst           string // rect | roundRect | ellipse
	Fill           string
	Outline        string
	OutlineWidthPt float64
	Radius         float64 // roundRect の角
	Runs           []TextRun
	FontSize       float64
	InsetX, InsetY float64 // 文字の枠(楕円なら内接する矩形)の内側の余白
	// Align・Anchor は文字の横・縦の置き方("" なら中央)。
	Align, Anchor string
	// Paras は Runs の後ろに続ける段落で、ParaSizes はその字の大きさ。
	Paras     [][]TextRun
	ParaSizes []float64
}

// AddShape は図形を足し、その id(コネクタの接続に使う)を返す。中の文字は
// 図形の文字の枠の幅で事前に折り返し、既定では中央に置く。
func (p *PPTXBuilder) AddShape(box LBox, o ShapeOpts) int {
	id := p.allocID()
	align, anchor := o.Align, o.Anchor
	if align == "" {
		align = "center"
	}
	if anchor == "" {
		anchor = "middle"
	}
	text := runsText(o.Runs)
	for _, r := range o.Paras {
		text += "\n" + runsText(r)
	}
	p.Shapes = append(p.Shapes, shape{
		id: id, kind: skShape, box: box, prst: o.Prst, fill: o.Fill, radius: o.Radius,
		hasOutline: o.Outline != "", outlineColor: o.Outline, outlineWidthPt: o.OutlineWidthPt,
		runs: o.Runs, text: text, fontSize: o.FontSize, insetX: o.InsetX, insetY: o.InsetY,
		align: align, anchor: anchor, paras: o.Paras, paraSizes: o.ParaSizes,
	})
	return id
}

// ConnectorEnd は線の端を図形の接続点に繋ぐ指定(ShapeID 0 は繋がない)。
type ConnectorEnd struct{ ShapeID, Site int }

// AddConnector は本物のコネクタ(p:cxnSp)を足す。st・end を渡すと a:stCxn・
// a:endCxn で図形に繋ぐので、PowerPoint で図形を動かすと線が付いてくる。
// arrowEnd なら終点に矢印(a:tailEnd)を付ける。
func (p *PPTXBuilder) AddConnector(x1, y1, x2, y2, widthPt float64, color string, arrowEnd bool, st, end ConnectorEnd) {
	p.Shapes = append(p.Shapes, shape{
		id: p.allocID(), kind: skConnector,
		x1: x1, y1: y1, x2: x2, y2: y2, lineColor: color, lineWidthPt: widthPt,
		arrowEnd: arrowEnd, stID: st.ShapeID, stIdx: st.Site, endID: end.ShapeID, endIdx: end.Site,
	})
}

// AddElbowConnector は、始点から縦に出て、adj(始点からの縦の割合)の高さで横に
// 曲がり、終点へ縦に入るコネクタ(bentConnector3)を足す。終点は始点より下に
// あること(組織図の親の下の辺 → 子の上の辺)。横にずれていなければ直線にする。
func (p *PPTXBuilder) AddElbowConnector(x1, y1, x2, y2, widthPt float64, color string, arrowEnd bool, st, end ConnectorEnd, adj float64) {
	p.AddConnector(x1, y1, x2, y2, widthPt, color, arrowEnd, st, end)
	if math.Abs(x2-x1) >= 0.5 && y2 > y1 {
		s := &p.Shapes[len(p.Shapes)-1]
		s.elbow, s.elbowAdj = true, adj
	}
}

func (p *PPTXBuilder) SetNotes(text string) { p.Notes = text }

// --- XML組み立てヘルパー ---

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func ptToEMU(pt float64) int64 { return int64(pt * 12700) }

func hexColor(c string) string { return strings.TrimPrefix(strings.ToUpper(c), "#") }

func anchorAttr(a string) string {
	switch a {
	case "middle":
		return "ctr"
	case "bottom":
		return "b"
	default:
		return "t"
	}
}

func alignAttr(a string) string {
	switch a {
	case "center":
		return "ctr"
	case "right":
		return "r"
	default:
		return "l"
	}
}

// textLines は文字の図形を書く行に分ける(自前で折り返し、wrap="none" で
// 書くため)。折り返しはこのスライドを組んだときの字体の一式で測る。
func (p *PPTXBuilder) textLines(s *shape) [][]TextRun {
	if p.Fonts != nil {
		defer useFonts(p.Fonts)()
	}
	var lines [][]TextRun
	switch {
	case len(s.runs) > 0:
		lines = wrapRuns(s.runs, s.fontSize, s.box.Width)
	case s.wrap:
		for _, l := range wrapText(s.text, s.fontSize, s.box.Width, s.bold) {
			lines = append(lines, []TextRun{{Text: l, Color: s.color, Bold: s.bold}})
		}
	default:
		for _, l := range strings.Split(s.text, "\n") {
			lines = append(lines, []TextRun{{Text: l, Color: s.color, Bold: s.bold}})
		}
	}
	return lines
}

func (p *PPTXBuilder) shapeXML(s *shape) string {
	switch s.kind {
	case skRect, skRoundRect:
		prst := "rect"
		adj := ""
		if s.kind == skRoundRect {
			prst = "roundRect"
			// 角丸の比率(短辺に対するradius)。avLstのgdは全体の縦横比に対する割合。
			minSide := s.box.Width
			if s.box.Height < minSide {
				minSide = s.box.Height
			}
			ratio := 0.0
			if minSide > 0 {
				ratio = s.radius / minSide
				if ratio > 0.5 {
					ratio = 0.5
				}
			}
			adj = fmt.Sprintf(`<a:avLst><a:gd name="adj" fmla="val %d"/></a:avLst>`, int(ratio*100000))
		}
		fillXML := "<a:noFill/>"
		if s.fill != "" {
			fillXML = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(s.fill))
		}
		lnXML := "<a:ln><a:noFill/></a:ln>"
		if s.hasOutline {
			dashXML := ""
			if s.outlineDash == "dash" {
				dashXML = `<a:prstDash val="dash"/>`
			}
			lnXML = fmt.Sprintf(`<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>`,
				ptToEMU(s.outlineWidthPt), hexColor(s.outlineColor), dashXML)
		}
		return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="Shape %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="%s">%s</a:prstGeom>
%s
%s
</p:spPr>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), prst, adj, fillXML, lnXML)

	case skOval:
		fillXML := "<a:noFill/>"
		if s.fill != "" {
			fillXML = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(s.fill))
		}
		return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="Dot %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
%s
<a:ln><a:noFill/></a:ln>
</p:spPr>
<p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), fillXML)

	case skText:
		lines := p.textLines(s)
		paras := p.parasXML(s, lines)
		return fmt.Sprintf(`<p:sp>
%s
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
<a:noFill/>
</p:spPr>
<p:txBody><a:bodyPr wrap="none" anchor="%s" lIns="0" tIns="0" rIns="0" bIns="0"><a:noAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody>
</p:sp>
`, textNvSpPr(s), ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), anchorAttr(s.anchor), paras)

	case skLine:
		dashXML := ""
		if s.dash == "dash" {
			dashXML = `<a:prstDash val="dash"/>`
		}
		x1, y1, x2, y2 := s.x1, s.y1, s.x2, s.y2
		x0, y0 := x1, y1
		if x2 < x0 {
			x0 = x2
		}
		if y2 < y0 {
			y0 = y2
		}
		w, h := x2-x1, y2-y1
		flipH, flipV := "", ""
		if w < 0 {
			flipH = ` flipH="1"`
			w = -w
		}
		if h < 0 {
			flipV = ` flipV="1"`
			h = -h
		}
		return fmt.Sprintf(`<p:cxnSp>
<p:nvCxnSpPr><p:cNvPr id="%d" name="Line %d"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>
<p:spPr>
<a:xfrm%s%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="line"><a:avLst/></a:prstGeom>
<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>
</p:spPr>
</p:cxnSp>
`, s.id, s.id, flipH, flipV, ptToEMU(x0), ptToEMU(y0), ptToEMU(w), ptToEMU(h), ptToEMU(s.lineWidthPt), hexColor(s.lineColor), dashXML)

	case skShape:
		return p.shapeWithTextXML(s)

	case skConnector:
		return connectorXML(s)

	case skPicture:
		return pictureXML(s)

	case skTable:
		return p.tableXML(s)
	case skChart:
		return chartFrameXML(s.id, s.box, s.chartRID)
	}
	return ""
}

// parasXML は折り返し済みの行を段落(a:p)にする。fonts.fixedLineSpacing なら
// 行の送りを測ったとおり(字の大きさ×lineHeightMult)に pt で書く。
func (p *PPTXBuilder) parasXML(s *shape, lines [][]TextRun) string {
	return p.parasXMLSized(s, lines, s.fontSize)
}

func (p *PPTXBuilder) parasXMLSized(s *shape, lines [][]TextRun, fontSize float64) string {
	var paras strings.Builder
	for _, line := range lines {
		if len(line) == 0 {
			line = []TextRun{{Color: s.color, Bold: s.bold}}
		}
		mark, rest := splitMark(line)
		var bullet, indent string
		if mark != nil && len(rest) > 0 {
			line = rest
			bullet, indent = p.bulletXML(mark, fontSize)
		}
		lnSpc := ""
		if p.Fonts != nil && p.Fonts.FixedLineSpacing {
			lnSpc = fmt.Sprintf(`<a:lnSpc><a:spcPts val="%d"/></a:lnSpc>`, int(math.Round(fontSize*lineHeightMult*100)))
		}
		if lnSpc == "" && bullet == "" {
			paras.WriteString(fmt.Sprintf(`<a:p><a:pPr%s algn="%s"/>`, indent, alignAttr(s.align)))
		} else {
			paras.WriteString(fmt.Sprintf(`<a:p><a:pPr%s algn="%s">%s%s</a:pPr>`, indent, alignAttr(s.align), lnSpc, bullet))
		}
		for _, r := range line {
			boldAttr := ""
			if r.Bold {
				boldAttr = ` b="1"`
			}
			paras.WriteString(fmt.Sprintf(`<a:r><a:rPr lang="ja-JP" sz="%d"%s><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r>`,
				int(runSize(r, fontSize)*100), boldAttr, hexColor(r.Color), escapeXML(r.Text)))
		}
		paras.WriteString(`</a:p>`)
	}
	return paras.String()
}

// splitMark は行の頭の印の塊(Mark)と残りに分ける。印が無ければ mark は nil。
func splitMark(line []TextRun) (mark, rest []TextRun) {
	i := 0
	for i < len(line) && line[i].Mark {
		i++
	}
	if i == 0 {
		return nil, line
	}
	return line[:i], line[i:]
}

// bulletXML は頭の印を段落の箇条の印(a:buClr・a:buFont・a:buChar)にし、
// 本文を印の幅(測ったとおり。印の後ろの空白も含む)だけ字下げする属性
// (marL と、印を左端に戻す負の indent)を返す。印の幅は折り返しの測りと
// 同じ数え方なので、1行目に入る文字は測ったとおり。印の字体は文字の
// 決まり(U+2000 未満は欧文、以降は和文)で選ぶ。
func (p *PPTXBuilder) bulletXML(mark []TextRun, fontSize float64) (bullet, indent string) {
	w := naturalRunsWidth(mark, fontSize)
	ch := strings.TrimSpace(runsText(mark))
	if ch == "" {
		ch = runsText(mark)
	}
	fonts := p.Fonts
	if fonts == nil {
		fonts = defaultFontSet()
	}
	font := fonts.Latin.Family
	for _, r := range ch {
		if r >= 0x2000 {
			font = fonts.EastAsian.Family
		}
		break
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<a:buClr><a:srgbClr val="%s"/></a:buClr>`, hexColor(mark[0].Color))
	if sz := runSize(mark[0], fontSize); sz != fontSize {
		fmt.Fprintf(&b, `<a:buSzPts val="%d"/>`, int(sz*100))
	}
	fmt.Fprintf(&b, `<a:buFont typeface="%s"/>`, escapeXML(font))
	fmt.Fprintf(&b, `<a:buChar char="%s"/>`, escapeXML(ch))
	e := ptToEMU(w)
	return b.String(), fmt.Sprintf(` marL="%d" indent="%d"`, e, -e)
}

// textNvSpPr は文字の図形の名前の部分。プレースホルダーなら p:ph を書き、
// スライドのレイアウトの同じ種類の置き場に結び付ける(PowerPoint のアウトライン・
// 読み上げ・レイアウトの差し替えが題として扱う)。位置・字の大きさ・色は図形の
// 側に書いたままなので、見た目は変わらない。
func textNvSpPr(s *shape) string {
	switch s.ph {
	case "":
		return fmt.Sprintf(`<p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`, s.id, s.id)
	case "title", "ctrTitle":
		return fmt.Sprintf(`<p:nvSpPr><p:cNvPr id="%d" name="Title %d"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="%s"/></p:nvPr></p:nvSpPr>`, s.id, s.id, s.ph)
	case "subTitle":
		return fmt.Sprintf(`<p:nvSpPr><p:cNvPr id="%d" name="Subtitle %d"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="subTitle" idx="1"/></p:nvPr></p:nvSpPr>`, s.id, s.id)
	default:
		panic("unknown placeholder " + s.ph)
	}
}

// slideLayoutNum はスライドが使うレイアウト(1 白紙・2 題だけ・3 表紙)。
// 持っているプレースホルダーで決める。
func slideLayoutNum(p *PPTXBuilder) int {
	n := 1
	for _, s := range p.Shapes {
		switch s.ph {
		case "ctrTitle", "subTitle":
			return 3
		case "title":
			n = 2
		}
	}
	return n
}

// phSpXML はマスター・レイアウトのプレースホルダー(中身の無い置き場)。
func phSpXML(id int, name, phAttrs string, box LBox, sz float64, algn string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph %s/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm></p:spPr><p:txBody><a:bodyPr lIns="0" tIns="0" rIns="0" bIns="0"><a:noAutofit/></a:bodyPr><a:lstStyle><a:lvl1pPr algn="%s"><a:defRPr sz="%d"/></a:lvl1pPr></a:lstStyle><a:p><a:endParaRPr lang="ja-JP"/></a:p></p:txBody></p:sp>`,
		id, name, phAttrs, ptToEMU(box.X), ptToEMU(box.Y), ptToEMU(box.Width), ptToEMU(box.Height), algn, int(sz*100))
}

// shapeTextRect は図形の文字の枠(prstGeom の文字の矩形)。楕円は内接する矩形。
func shapeTextRect(prst string, box LBox) LBox {
	if prst != "ellipse" {
		return box
	}
	const k = 0.70710678 // cos 45°
	w, h := box.Width*k, box.Height*k
	return LBox{X: box.X + (box.Width-w)/2, Y: box.Y + (box.Height-h)/2, Width: w, Height: h}
}

// shapeWithTextXML は塗り・枠線・中の文字を持つ図形(skShape)。
func (p *PPTXBuilder) shapeWithTextXML(s *shape) string {
	adj := "<a:avLst/>"
	if s.prst == "roundRect" {
		minSide := s.box.Width
		if s.box.Height < minSide {
			minSide = s.box.Height
		}
		ratio := 0.0
		if minSide > 0 {
			ratio = s.radius / minSide
			if ratio > 0.5 {
				ratio = 0.5
			}
		}
		adj = fmt.Sprintf(`<a:avLst><a:gd name="adj" fmla="val %d"/></a:avLst>`, int(ratio*100000))
	}
	fillXML := "<a:noFill/>"
	if s.fill != "" {
		fillXML = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, hexColor(s.fill))
	}
	lnXML := "<a:ln><a:noFill/></a:ln>"
	if s.hasOutline {
		lnXML = fmt.Sprintf(`<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`,
			ptToEMU(s.outlineWidthPt), hexColor(s.outlineColor))
	}
	paras := "<a:p/>"
	if len(s.runs) > 0 {
		tr := shapeTextRect(s.prst, s.box)
		restore := func() {}
		if p.Fonts != nil {
			restore = useFonts(p.Fonts)
		}
		lines := wrapRuns(s.runs, s.fontSize, tr.Width-2*s.insetX)
		var extra []string
		for i, r := range s.paras {
			extra = append(extra, p.parasXMLSized(s, wrapRuns(r, s.paraSizes[i], tr.Width-2*s.insetX), s.paraSizes[i]))
		}
		restore()
		paras = p.parasXML(s, lines) + strings.Join(extra, "")
	}
	return fmt.Sprintf(`<p:sp>
<p:nvSpPr><p:cNvPr id="%d" name="Shape %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
<p:spPr>
<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="%s">%s</a:prstGeom>
%s
%s
</p:spPr>
<p:txBody><a:bodyPr wrap="none" anchor="%s" lIns="%d" tIns="%d" rIns="%d" bIns="%d"><a:noAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody>
</p:sp>
`, s.id, s.id, ptToEMU(s.box.X), ptToEMU(s.box.Y), ptToEMU(s.box.Width), ptToEMU(s.box.Height), s.prst, adj, fillXML, lnXML,
		anchorAttr(s.anchor), ptToEMU(s.insetX), ptToEMU(s.insetY), ptToEMU(s.insetX), ptToEMU(s.insetY), paras)
}

// connectorXML は本物のコネクタ(p:cxnSp)。図形に繋ぐなら a:stCxn・a:endCxn を書く。
func connectorXML(s *shape) string {
	if s.elbow {
		return elbowConnectorXML(s)
	}
	x0, y0 := math.Min(s.x1, s.x2), math.Min(s.y1, s.y2)
	w, h := math.Abs(s.x2-s.x1), math.Abs(s.y2-s.y1)
	flip := ""
	if s.x2 < s.x1 {
		flip += ` flipH="1"`
	}
	if s.y2 < s.y1 {
		flip += ` flipV="1"`
	}
	cxn := ""
	if s.stID > 0 {
		cxn += fmt.Sprintf(`<a:stCxn id="%d" idx="%d"/>`, s.stID, s.stIdx)
	}
	if s.endID > 0 {
		cxn += fmt.Sprintf(`<a:endCxn id="%d" idx="%d"/>`, s.endID, s.endIdx)
	}
	tail := ""
	if s.arrowEnd {
		tail = `<a:tailEnd type="triangle" w="med" len="med"/>`
	}
	return fmt.Sprintf(`<p:cxnSp>
<p:nvCxnSpPr><p:cNvPr id="%d" name="Connector %d"/><p:cNvCxnSpPr>%s</p:cNvCxnSpPr><p:nvPr/></p:nvCxnSpPr>
<p:spPr>
<a:xfrm%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="straightConnector1"><a:avLst/></a:prstGeom>
<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>
</p:spPr>
</p:cxnSp>
`, s.id, s.id, cxn, flip, ptToEMU(x0), ptToEMU(y0), ptToEMU(w), ptToEMU(h), ptToEMU(s.lineWidthPt), hexColor(s.lineColor), tail)
}

// elbowConnectorXML は縦に出て縦に入る曲がったコネクタ。bentConnector3 の道筋は
// 「横 → 縦 → 横」なので、図形を 90°回して「縦 → 横 → 縦」にする。回す前の
// 枠は幅が縦のずれ・高さが横のずれで、中心は両端の中点。終点が右にあるときは
// 回す前に上下を返す(flipV)。
func elbowConnectorXML(s *shape) string {
	dx, dy := s.x2-s.x1, s.y2-s.y1
	cx, cy := (s.x1+s.x2)/2, (s.y1+s.y2)/2
	w, h := dy, math.Abs(dx)
	flip := ""
	if dx > 0 {
		flip = ` flipV="1"`
	}
	adj := s.elbowAdj
	if adj == 0 {
		adj = 0.5
	}
	cxn := ""
	if s.stID > 0 {
		cxn += fmt.Sprintf(`<a:stCxn id="%d" idx="%d"/>`, s.stID, s.stIdx)
	}
	if s.endID > 0 {
		cxn += fmt.Sprintf(`<a:endCxn id="%d" idx="%d"/>`, s.endID, s.endIdx)
	}
	tail := ""
	if s.arrowEnd {
		tail = `<a:tailEnd type="triangle" w="med" len="med"/>`
	}
	return fmt.Sprintf(`<p:cxnSp>
<p:nvCxnSpPr><p:cNvPr id="%d" name="Connector %d"/><p:cNvCxnSpPr>%s</p:cNvCxnSpPr><p:nvPr/></p:nvCxnSpPr>
<p:spPr>
<a:xfrm rot="5400000"%s><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>
<a:prstGeom prst="bentConnector3"><a:avLst><a:gd name="adj1" fmla="val %d"/></a:avLst></a:prstGeom>
<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</a:ln>
</p:spPr>
</p:cxnSp>
`, s.id, s.id, cxn, flip, ptToEMU(cx-w/2), ptToEMU(cy-h/2), ptToEMU(w), ptToEMU(h),
		int(math.Round(adj*100000)), ptToEMU(s.lineWidthPt), hexColor(s.lineColor), tail)
}

// WriteDeck は複数枚のスライドを1つの pptx(zipコンテナ)として w に書く。
// 大きさは1枚目のものを使う。title は文書の題(空なら "boxloom")。
func WriteDeck(w io.Writer, slides []*PPTXBuilder, title string) error {
	if len(slides) == 0 {
		return fmt.Errorf("WriteDeck: no slides")
	}
	p := slides[0]
	n := len(slides)
	if title == "" {
		title = "boxloom"
	}
	zw := zip.NewWriter(w)

	write := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}

	hasNotes := false
	for _, sl := range slides {
		if sl.Notes != "" {
			hasNotes = true
		}
	}
	media := assignMedia(slides)
	var slideTypes, notesTypes, sldIDs, slideRelsList strings.Builder
	// グラフに文書の中の通し番号と、スライドからの関係の Id を振る。
	nCharts := 0
	for _, sl := range slides {
		k := 0
		for j := range sl.Shapes {
			if sl.Shapes[j].kind == skChart {
				nCharts++
				k++
				sl.Shapes[j].chartNum = nCharts
				sl.Shapes[j].chartRID = fmt.Sprintf("rIdChart%d", k) // 画像の rId10〜 と重ならない名前
				slideTypes.WriteString(fmt.Sprintf(`
<Override PartName="/ppt/charts/chart%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`, nCharts))
			}
		}
	}
	for i, sl := range slides {
		slideTypes.WriteString(fmt.Sprintf(`
<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i+1))
		if sl.Notes != "" {
			notesTypes.WriteString(fmt.Sprintf(`
<Override PartName="/ppt/notesSlides/notesSlide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/>`, i+1))
		}
		sldIDs.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+2))
		slideRelsList.WriteString(fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+2, i+1))
	}

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>` + mediaContentTypes(media) + xlsxDefault(nCharts) + `
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
<Override PartName="/ppt/slideLayouts/slideLayout2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
<Override PartName="/ppt/slideLayouts/slideLayout3.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` + slideTypes.String() + `
<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`
	if hasNotes {
		contentTypes += `
<Override PartName="/ppt/notesMasters/notesMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"/>` + notesTypes.String()
	}
	contentTypes += `
</Types>`

	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`

	coreXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>` + escapeXML(title) + `</dc:title><dc:creator>boxloom</dc:creator>
</cp:coreProperties>`

	appXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
<Application>boxloom</Application>
</Properties>`

	notesSz := fmt.Sprintf(`<p:notesSz cx="%d" cy="%d"/>`, ptToEMU(p.HeightPt), ptToEMU(p.WidthPt))
	presentation := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
<p:sldIdLst>%s</p:sldIdLst>
<p:sldSz cx="%d" cy="%d"/>
%s
</p:presentation>`, sldIDs.String(), ptToEMU(p.WidthPt), ptToEMU(p.HeightPt), notesSz)

	presRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>` + slideRelsList.String() + fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`, n+2)
	if hasNotes {
		presRels += fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="notesMasters/notesMaster1.xml"/>`, n+3)
	}
	presRels += `
</Relationships>`

	// テーマの字体:欧文(a:latin)と和文(a:ea)を分けて書く。a:ea が無いと
	// PowerPoint は和文を既定の字体で描き、測った字体と食い違う(docs/fonts.md)。
	fonts := p.Fonts
	if fonts == nil {
		fonts = defaultFontSet()
	}
	fontRefs := fmt.Sprintf(`<a:latin typeface="%s"/><a:ea typeface="%s"/><a:cs typeface=""/>`,
		escapeXML(fonts.Latin.Family), escapeXML(fonts.EastAsian.Family))

	scheme := themeScheme(nil)
	if len(slides) > 0 {
		scheme = themeScheme(slides[0].ColorTheme)
	}
	theme := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="` + escapeXML(scheme.name) + `">
<a:themeElements>
` + scheme.clrSchemeXML() + `
<a:fontScheme name="` + escapeXML(scheme.name) + `">
<a:majorFont>` + fontRefs + `</a:majorFont>
<a:minorFont>` + fontRefs + `</a:minorFont>
</a:fontScheme>
<a:fmtScheme name="` + escapeXML(scheme.name) + `">
<a:fillStyleLst><a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:fillStyleLst>
<a:lnStyleLst><a:ln w="12700"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:ln></a:lnStyleLst>
<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>
</a:fmtScheme>
</a:themeElements>
</a:theme>`

	// マスターと3つのレイアウト(白紙・題だけ・表紙)。置き場の位置は目安で、
	// スライドの図形は自分の位置を持つ(エンジンの座標)。
	W, H := p.WidthPt, p.HeightPt
	titleBox := LBox{X: 24, Y: 24, Width: W - 48, Height: 40}
	ctrBox := LBox{X: 48, Y: H * 0.3, Width: W - 96, Height: 60}
	subBox := LBox{X: 48, Y: H*0.3 + 80, Width: W - 96, Height: 30}
	master := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="` + scheme.bg + `"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>
<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` + phSpXML(2, "Title Placeholder 1", `type="title"`, titleBox, 24, "l") + `</p:spTree>
</p:cSld>
` + scheme.masterClrMap() + `
<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/><p:sldLayoutId id="2147483650" r:id="rId2"/><p:sldLayoutId id="2147483651" r:id="rId3"/></p:sldLayoutIdLst>
</p:sldMaster>`

	masterRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout2.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout3.xml"/>
<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

	layoutXML := func(typ, name, phs string) string {
		return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="` + typ + `" preserve="1">
<p:cSld name="` + name + `"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` + phs + `</p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`
	}
	layouts := []string{
		layoutXML("blank", "Blank", ""),
		layoutXML("titleOnly", "Title Only", phSpXML(2, "Title 1", `type="title"`, titleBox, 24, "l")),
		layoutXML("title", "Title Slide", phSpXML(2, "Title 1", `type="ctrTitle"`, ctrBox, 36, "l")+
			phSpXML(3, "Subtitle 2", `type="subTitle" idx="1"`, subBox, 16, "l")),
	}

	layoutRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

	if err := write("[Content_Types].xml", contentTypes); err != nil {
		return err
	}
	if err := write("_rels/.rels", rootRels); err != nil {
		return err
	}
	if err := write("docProps/core.xml", coreXML); err != nil {
		return err
	}
	if err := write("docProps/app.xml", appXML); err != nil {
		return err
	}
	if err := write("ppt/presentation.xml", presentation); err != nil {
		return err
	}
	if err := write("ppt/_rels/presentation.xml.rels", presRels); err != nil {
		return err
	}
	if err := write("ppt/theme/theme1.xml", theme); err != nil {
		return err
	}
	if err := write("ppt/slideMasters/slideMaster1.xml", master); err != nil {
		return err
	}
	if err := write("ppt/slideMasters/_rels/slideMaster1.xml.rels", masterRels); err != nil {
		return err
	}
	for i, l := range layouts {
		if err := write(fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", i+1), l); err != nil {
			return err
		}
		if err := write(fmt.Sprintf("ppt/slideLayouts/_rels/slideLayout%d.xml.rels", i+1), layoutRels); err != nil {
			return err
		}
	}
	for i, sl := range slides {
		if err := writeSlide(write, i+1, sl); err != nil {
			return err
		}
	}
	for _, m := range media {
		if err := write("ppt/media/"+m.name, string(m.data)); err != nil {
			return err
		}
	}

	if hasNotes {
		notesMaster := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notesMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` + notesMasterPlaceholders(p.WidthPt, p.HeightPt) + `</p:spTree></p:cSld>
<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
</p:notesMaster>`
		notesMasterRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`
		if err := write("ppt/notesMasters/notesMaster1.xml", notesMaster); err != nil {
			return err
		}
		if err := write("ppt/notesMasters/_rels/notesMaster1.xml.rels", notesMasterRels); err != nil {
			return err
		}
	}

	return zw.Close()
}

// writeSlide は num 枚目のスライド(と、あればノート)を書く。
func writeSlide(write func(name, content string) error, num int, p *PPTXBuilder) error {
	var shapesXML strings.Builder
	for i := range p.Shapes {
		shapesXML.WriteString(p.shapeXML(&p.Shapes[i]))
	}

	slide := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
<p:grpSpPr/>
%s
</p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`, shapesXML.String())

	slideRels := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout%d.xml"/>`, slideLayoutNum(p))
	if p.Notes != "" {
		slideRels += fmt.Sprintf(`
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide%d.xml"/>`, num)
	}
	slideRels += slideImageRels(p)
	for _, s := range p.Shapes {
		if s.kind != skChart {
			continue
		}
		slideRels += fmt.Sprintf(`
<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart%d.xml"/>`, s.chartRID, s.chartNum)
		if err := writeChart(write, s.chartNum, s.chart); err != nil {
			return err
		}
	}
	slideRels += `
</Relationships>`

	if err := write(fmt.Sprintf("ppt/slides/slide%d.xml", num), slide); err != nil {
		return err
	}
	if err := write(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", num), slideRels); err != nil {
		return err
	}
	if p.Notes == "" {
		return nil
	}
	notesSlide := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Slide Image Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr/></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
<p:spPr/>
<p:txBody><a:bodyPr/><a:lstStyle/>%s</p:txBody>
</p:sp>
</p:spTree></p:cSld>
<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:notes>`, notesParagraphs(p.Notes))
	notesSlideRels := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="../notesMasters/notesMaster1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="../slides/slide%d.xml"/>
</Relationships>`, num)

	if err := write(fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", num), notesSlide); err != nil {
		return err
	}
	return write(fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", num), notesSlideRels)
}

// notesParagraphs はノートの文を改行ごとに1段落にする(空の行は空の段落)。
func notesParagraphs(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			b.WriteString(`<a:p><a:endParaRPr lang="ja-JP"/></a:p>`)
			continue
		}
		b.WriteString(`<a:p><a:r><a:t>` + escapeXML(line) + `</a:t></a:r></a:p>`)
	}
	return b.String()
}

// notesMasterPlaceholders はノートのページ(縦長。幅=スライドの高さ、高さ=スライドの幅)
// に、上にスライドの縮図・下にノートの文の置き場を置く。これが無いと LibreOffice の
// ノートのページに何も出ない。
func notesMasterPlaceholders(slideW, slideH float64) string {
	pageW, pageH := slideH, slideW
	imgW := pageW * 0.84
	imgH := imgW * slideH / slideW
	x := (pageW - imgW) / 2
	y := pageH * 0.07
	bodyY := y + imgH + pageH*0.04
	bodyH := pageH*0.95 - bodyY
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Slide Image Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1" noRot="1" noChangeAspect="1"/></p:cNvSpPr><p:nvPr><p:ph type="sldImg"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:sp>`+
		`<p:sp><p:nvSpPr><p:cNvPr id="3" name="Notes Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" sz="quarter" idx="1"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ja-JP"/></a:p></p:txBody></p:sp>`,
		ptToEMU(x), ptToEMU(y), ptToEMU(imgW), ptToEMU(imgH),
		ptToEMU(x), ptToEMU(bodyY), ptToEMU(imgW), ptToEMU(bodyH))
}

// deckScheme は PPTX のテーマの配色とマスターの clrMap(docs/template-values.md の
// 「PPTX のテーマへの写し方」)。
type deckScheme struct {
	name     string
	dark     bool // 地が暗い(マスターの clrMap で bg1 を dk1 に向ける)
	bg       string
	slots    [12][2]string // a:clrScheme の子の名前と色(16進)
	fallback bool          // 固定の既定(ColorTheme が nil)
}

// themeScheme は値の表の色からテーマの配色を作る。地(bg)と文字(text_primary)の
// うち暗いほうを dk1、明るいほうを lt1 に、地の側の2つ目(card_alt)と文字の側の
// 2つ目(text_muted)を dk2/lt2 に置く。地が暗いテーマはマスターの clrMap で
// bg1=dk1・tx1=lt1 に向ける(PowerPoint で足した文字が地の上で読める)。
func themeScheme(c *DeckColors) deckScheme {
	if c == nil || len(c.Colors) == 0 {
		return deckScheme{name: "boxloom", bg: "FFFFFF", fallback: true}
	}
	col := func(role, def string) string {
		if v, ok := c.Colors[role]; ok && v != "" {
			return hexColor(v)
		}
		return def
	}
	bg := col("bg", "FFFFFF")
	text := col("text_primary", "000000")
	muted := col("text_muted", text)
	cardAlt := col("card_alt", bg)
	dark := relLuminance(bg) < relLuminance(text)
	s := deckScheme{name: c.Name, dark: dark, bg: bg}
	if s.name == "" {
		s.name = "boxloom"
	}
	dk1, lt1, dk2, lt2 := text, bg, muted, cardAlt
	if dark {
		dk1, lt1, dk2, lt2 = bg, text, cardAlt, muted
	}
	accent := col("accent", "0033CC")
	s.slots = [12][2]string{
		{"dk1", dk1}, {"lt1", lt1}, {"dk2", dk2}, {"lt2", lt2},
		{"accent1", accent},
		{"accent2", col("conditional", accent)},
		{"accent3", col("support", accent)},
		{"accent4", col("danger", accent)},
		{"accent5", col("none", muted)},
		{"accent6", muted},
		{"hlink", accent},
		{"folHlink", muted},
	}
	return s
}

func (s deckScheme) clrSchemeXML() string {
	if s.fallback {
		return `<a:clrScheme name="boxloom">
<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>
<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
<a:dk2><a:srgbClr val="0033CC"/></a:dk2>
<a:lt2><a:srgbClr val="EEECE1"/></a:lt2>
<a:accent1><a:srgbClr val="0033CC"/></a:accent1>
<a:accent2><a:srgbClr val="4472C4"/></a:accent2>
<a:accent3><a:srgbClr val="70AD47"/></a:accent3>
<a:accent4><a:srgbClr val="FFC000"/></a:accent4>
<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5>
<a:accent6><a:srgbClr val="A5A5A5"/></a:accent6>
<a:hlink><a:srgbClr val="0563C1"/></a:hlink>
<a:folHlink><a:srgbClr val="954F72"/></a:folHlink>
</a:clrScheme>`
	}
	var b strings.Builder
	b.WriteString(`<a:clrScheme name="` + escapeXML(s.name) + `">`)
	for _, sl := range s.slots {
		b.WriteString(fmt.Sprintf("\n<a:%s><a:srgbClr val=\"%s\"/></a:%s>", sl[0], sl[1], sl[0]))
	}
	b.WriteString("\n</a:clrScheme>")
	return b.String()
}

// masterClrMap はスライドマスターの clrMap。ノートのマスターは紙に刷る前提で
// いつも明るい向き(bg1=lt1)のまま。
func (s deckScheme) masterClrMap() string {
	if s.dark {
		return `<p:clrMap bg1="dk1" tx1="lt1" bg2="dk2" tx2="lt2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`
	}
	return `<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`
}

// relLuminance は16進の色の相対輝度(WCAG の式)。
func relLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0
	}
	ch := func(i int) float64 {
		var v int
		fmt.Sscanf(hex[i:i+2], "%02x", &v)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(0) + 0.7152*ch(2) + 0.0722*ch(4)
}

// xlsxDefault は埋め込みの xlsx の内容の種類(グラフがあるときだけ)。
func xlsxDefault(nCharts int) string {
	if nCharts == 0 {
		return ""
	}
	return `
<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`
}

// writeChart は n 番目のグラフの部品(c:chartSpace・その関係・埋め込みの xlsx)を書く。
func writeChart(write func(name, content string) error, n int, d *ChartData) error {
	xlsx, err := d.XLSX()
	if err != nil {
		return err
	}
	if err := write(fmt.Sprintf("ppt/charts/chart%d.xml", n), d.ChartXML("rId1")); err != nil {
		return err
	}
	rels := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="../embeddings/Microsoft_Excel_Worksheet%d.xlsx"/>
</Relationships>`, n)
	if err := write(fmt.Sprintf("ppt/charts/_rels/chart%d.xml.rels", n), rels); err != nil {
		return err
	}
	return write(fmt.Sprintf("ppt/embeddings/Microsoft_Excel_Worksheet%d.xlsx", n), string(xlsx))
}
