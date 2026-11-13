// shapes サブシステム: 生の図形への逃げ道。
//
// 既存のパターン(箇条・比較・Gantt …)で表せない図(架空のシステム構成図など)を、
// サブシステムの枠(キャンバス)の中だけで、図形を並べて描く。最後の手段なので、
// 自由にできることを次のように絞ってある(docs/subsystem.md の8節)。
//   - 座標と大きさは枠の内側(padding を除いた矩形)に対する 0〜1 の相対。
//     x+w・y+h も 1 以下。外れたら誤り。
//   - 色は役割名(テーマの役割名)だけ。16進の色は書けない。
//   - 文字の大きさは数ではなく、テンプレートの値の表の段の名前(textStyle)で引く。
//   - 図形どうしの重なりは誤りにせず、check の警告(shapes-overlap)にする。
//
// IR の data の形は schema/subsystems/shapes.schema.json。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// shapeKinds は item の kind。boxed は矩形の場所(x・y・w・h)を持つもの。
var (
	shapeBoxedKinds = []string{"rect", "roundRect", "ellipse", "text"}
	shapeKindsAll   = []string{"rect", "roundRect", "ellipse", "text", "line", "arrow", "connector"}
)

// shapeTextStyles は textStyle に書ける段の名前(テンプレートの values のキー)。
// 本文の部品が使う段だけで、表紙・題の段(coverTitle・slideTitle …)は許さない
// (図の中の文字がスライドの題と競わないように)。小さい順。
var shapeTextStyles = []string{"small", "subPoint", "optionItem", "body", "optionLabel"}

const defaultShapeTextStyle = "small"

// 役割名を省いたときの色。
const (
	defaultShapeFill  = "card_alt"
	defaultShapeText  = "text_primary"
	defaultShapeLine  = "text_muted" // line・arrow・connector の色
	ellipseSiteFactor = 2            // 楕円の接続点は8つで、上・左・下・右は 0・2・4・6
)

// shapeAllowedKeys は kind ごとに書けるキー。
var shapeAllowedKeys = map[string][]string{
	"rect":      {"id", "kind", "x", "y", "w", "h", "fill", "line", "text", "textStyle", "textColor"},
	"roundRect": {"id", "kind", "x", "y", "w", "h", "fill", "line", "text", "textStyle", "textColor"},
	"ellipse":   {"id", "kind", "x", "y", "w", "h", "fill", "line", "text", "textStyle", "textColor"},
	"text":      {"id", "kind", "x", "y", "w", "h", "text", "textStyle", "color"},
	"line":      {"kind", "from", "to", "color"},
	"arrow":     {"kind", "from", "to", "color"},
	"connector": {"kind", "from", "to", "arrow", "color"},
}

var shapeRequiredKeys = map[string][]string{
	"rect":      {"x", "y", "w", "h"},
	"roundRect": {"x", "y", "w", "h"},
	"ellipse":   {"x", "y", "w", "h"},
	"text":      {"x", "y", "w", "h", "text"},
	"line":      {"from", "to"},
	"arrow":     {"from", "to"},
	"connector": {"from", "to"},
}

// shapeRoleKeys は役割名を書くキー。
var shapeRoleKeys = []string{"fill", "line", "textColor", "color"}

// roleNameRe は役割名の形。16進の色(0033CC・#0033cc)はこの形に合わない
// (数字で始まるか # を含む)か、合っても役割名の表に無いので断られる。
var roleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// hexColorLikeRe は16進の色に見えるもの(小文字の deadbe も役割名の形に合うので別に断る)。
var hexColorLikeRe = regexp.MustCompile(`^#?[0-9A-Fa-f]{6}$`)

// ShapeItem は1つの図形・線(色は役割名のまま。省いたものは既定で埋めてある)。
type ShapeItem struct {
	ID, Kind   string
	X, Y, W, H float64    // boxed
	From, To   [2]float64 // line・arrow
	FromID     string     // connector
	ToID       string     // connector
	Arrow      bool       // connector(arrow は常に true)
	Fill, Line string     // 図形の塗りと枠線(Line が空なら枠線なし)
	TextColor  string     // 図形・text の文字の色
	Color      string     // line・arrow・connector の色
	Text       Inline     // 図形・text の文字
	TextStyle  string     // 字の段の名前
}

func (it *ShapeItem) boxed() bool { return contains(shapeBoxedKinds, it.Kind) }

// label は警告の文に使う名前(id、無ければ文の1行目、無ければ kind)。
func (it *ShapeItem) label() string {
	if it.ID != "" {
		return it.ID
	}
	if t := strings.SplitN(it.Text.Plain(), "\n", 2)[0]; t != "" {
		return t
	}
	return it.Kind
}

// roles は item が使う役割名(空でないもの)。
func (it *ShapeItem) roles() []string {
	var out []string
	for _, r := range []string{it.Fill, it.Line, it.TextColor, it.Color} {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

// ShapesData は IR の data を読んだもの。
type ShapesData struct {
	Aspect float64 // 0 なら書いていない
	Items  []ShapeItem
}

// ShapesSubsystem は ShapesData を描くサブシステム。
type ShapesSubsystem struct {
	Data *ShapesData
	// V は寸法の値の表。nil なら既定(subsystems/shapes.json)。
	V *ShapesValues
	// TextSizes は字の段の名前 → 大きさ(pt)。テンプレートの values から入れる。
	// nil なら既定のテンプレート(light)の values。
	TextSizes map[string]float64
}

// NewShapesSubsystem は登録簿の Factory。data は *ShapesData か IR の data。
func NewShapesSubsystem(data any) (Subsystem, error) {
	switch v := data.(type) {
	case *ShapesData:
		return &ShapesSubsystem{Data: v}, nil
	case json.RawMessage:
		d, err := shapesFromIR(v)
		if err != nil {
			return nil, err
		}
		return &ShapesSubsystem{Data: d}, nil
	}
	return nil, fmt.Errorf("shapes: expected *ShapesData or IR data, got %T", data)
}

func (s *ShapesSubsystem) Name() string { return "shapes" }

func (s *ShapesSubsystem) vals() *ShapesValues {
	if s.V != nil {
		return s.V
	}
	return defaultShapesValues()
}

func (s *ShapesSubsystem) textSize(style string) float64 {
	sizes := s.TextSizes
	if sizes == nil {
		sizes = shapeTextSizes(mustTemplate("light").Values)
	}
	return sizes[style]
}

// shapeTextSizes は values から字の段の表を作る。
func shapeTextSizes(v TemplateValues) map[string]float64 {
	return map[string]float64{
		"small": v.Small, "subPoint": v.SubPoint, "optionItem": v.OptionItem,
		"body": v.Body, "optionLabel": v.OptionLabel,
	}
}

// checkRoles は役割名がテーマにあるかを見る(テンプレートの Factory が呼ぶ)。
func (s *ShapesSubsystem) checkRoles(th Theme) error {
	for i := range s.Data.Items {
		for _, r := range s.Data.Items[i].roles() {
			if _, ok := th[r]; !ok {
				return fmt.Errorf("shapes: items[%d]: theme has no role %q (known: %v)", i, r, sortedKeys(th))
			}
		}
	}
	return nil
}

// --- 大きさ ---

// inner は枠の内側(相対座標の 0〜1 が指す矩形)。
func (s *ShapesSubsystem) inner(box LBox) LBox {
	p := s.vals().Padding
	return LBox{X: box.X + p, Y: box.Y + p, Width: math.Max(0, box.Width-2*p), Height: math.Max(0, box.Height-2*p)}
}

// itemBox は boxed の item の絶対の矩形。
func itemBox(in LBox, it *ShapeItem) LBox {
	return LBox{X: in.X + it.X*in.Width, Y: in.Y + it.Y*in.Height, Width: it.W * in.Width, Height: it.H * in.Height}
}

func shapePrst(kind string) string {
	if kind == "text" {
		return "rect"
	}
	return kind
}

// itemRuns は文を見た目の塊にする(強調はアクセントの色の太字。テンプレートの
// 部品と同じ見せ方)。roles が nil なら色は空(大きさを測るだけのとき)。
func itemRuns(in Inline, color string, roles Roles) []TextRun {
	c := func(role string) string {
		if roles == nil {
			return ""
		}
		return roles.Color(role)
	}
	var out []TextRun
	var walk func(in Inline, color string, bold bool)
	walk = func(in Inline, color string, bold bool) {
		for _, p := range in {
			switch p.Type {
			case "":
				out = append(out, TextRun{Text: p.Text, Color: color, Bold: bold})
			case "br":
				out = append(out, TextRun{Text: "\n", Color: color, Bold: bold})
			case "code":
				out = append(out, TextRun{Text: p.Text, Color: c("text_muted"), Bold: bold})
			case "em":
				walk(p.Content, c("accent"), true)
			case "link":
				walk(p.Content, c("accent"), bold)
			}
		}
	}
	walk(in, c(color), false)
	return out
}

// textNeed は item の文を、絶対の矩形 b に置いたときに要る大きさと、使える大きさ。
func (s *ShapesSubsystem) textNeed(it *ShapeItem, b LBox) (needW, needH, haveW, haveH float64) {
	v := s.vals()
	size := s.textSize(it.TextStyle)
	runs := itemRuns(it.Text, "", nil)
	if it.Kind == "text" {
		haveW, haveH = b.Width, b.Height
	} else {
		tr := shapeTextRect(it.Kind, b, 0)
		haveW, haveH = tr.Width-2*v.TextInsetX, tr.Height-2*v.TextInsetY
	}
	lines := wrapRuns(runs, size, math.Max(haveW, 0))
	for _, l := range lines {
		needW = math.Max(needW, naturalRunsWidth(l, size))
	}
	needH = float64(len(lines)) * size * lineHeightMult()
	return
}

// Measure は自然・最小・上限の大きさを申告する。
//   - 自然な大きさ: 幅は分かっていればそれ、無ければ naturalWidth。高さは内側の
//     縦横比が aspect(省けば defaultAspect)になる高さ。
//   - 最小: minHeight と、どの図形の文も収まる高さ(図形の高さは内側の高さに
//     比例するので、文の要る高さ ÷ 相対の高さ)の大きいほう。
//   - 上限: aspect を書いたときだけ自然な高さ(作者が決めた形を崩さない)。書いて
//     いなければ上限なし(余りを grow で受け取る)。
func (s *ShapesSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	aspect := s.Data.Aspect
	if aspect <= 0 {
		aspect = v.DefaultAspect
	}
	w := v.NaturalWidth
	switch {
	case c.Width > 0:
		w = c.Width
	case c.Height > 0:
		w = math.Max(0, c.Height-2*v.Padding)*aspect + 2*v.Padding
	}
	innerW := math.Max(0, w-2*v.Padding)
	natH := innerW/aspect + 2*v.Padding

	minInnerH := 0.0
	for i := range s.Data.Items {
		it := &s.Data.Items[i]
		if !it.boxed() || len(it.Text) == 0 {
			continue
		}
		// 高さを 1pt にして要る高さを測る(要る高さは幅だけで決まる)。
		b := LBox{Width: it.W * innerW, Height: it.H}
		_, needH, _, _ := s.textNeed(it, b)
		frac := it.H
		if it.Kind == "text" {
			minInnerH = math.Max(minInnerH, needH/frac)
			continue
		}
		k := 1.0
		if it.Kind == "ellipse" {
			k = 0.70710678
		}
		minInnerH = math.Max(minInnerH, (needH+2*v.TextInsetY)/(k*frac))
	}
	minH := math.Max(v.MinHeight, minInnerH+2*v.Padding)
	if natH < minH {
		natH = minH
	}
	m := Measurement{
		Natural: Size{Width: w, Height: natH},
		Min:     Size{Width: v.MinWidth, Height: minH},
	}
	if s.Data.Aspect > 0 {
		m.Max = Size{Width: w, Height: natH}
		m.FixedAspect = true
	}
	return m
}

// --- 描く ---

// connectorSites は boxed の item の上・左・下・右の接続点の番号と位置。
func connectorSite(kind string, b LBox, side int) (idx int, x, y float64) {
	idx = side
	if kind == "ellipse" {
		idx = side * ellipseSiteFactor
	}
	switch side {
	case 0:
		return idx, b.X + b.Width/2, b.Y
	case 1:
		return idx, b.X, b.Y + b.Height/2
	case 2:
		return idx, b.X + b.Width/2, b.Y + b.Height
	}
	return idx, b.X + b.Width, b.Y + b.Height/2
}

// connectorAngleMin は、コネクタが辺に入る(出る)角度の下限(度)。これより浅いと
// 線が辺をかすめて、角で終わったように見える(#41)。向かい合う辺の組がこの角度を
// 満たせばその組を使い、どの組でも満たせなければ check が shapes-connector-angle
// の警告を出す(図形の位置を変えないと直らないので、作者に知らせる)。
const connectorAngleMin = 30.0

// sideNormals は辺(0=上・1=左・2=下・3=右)の外向きの向き。
var sideNormals = [4][2]float64{{0, -1}, {-1, 0}, {0, 1}, {1, 0}}

// connectorSides は a から b へ繋ぐときに使う辺(0=上・1=左・2=下・3=右)と、
// その組で線が辺に入る浅いほうの角度(度)。
//
// 辺の中点どうしを結ぶ直線は、辺に浅い角度で入ると辺をかすめ、角で終わったように
// 見える(#41)。そこで次の順に選ぶ:
//  1. 向かい合う辺の組(右→左・下→上など)のうち、両端の角度の浅いほうが最も
//     大きいもの。それが connectorAngleMin 以上なら使う。
//  2. 足りなければ、隣り合う辺の組(右→上など)も含めて、浅いほうの角度が最も
//     大きい組を使う。
//
// 使えるのは、両端の辺が相手のほうを向いている組だけ(線が自分の図形の中を
// 通らない)。角度が同じなら向かい合う組、次に短いほうを選ぶ。
func connectorSides(ka, kb string, a, b LBox) (sa, sb int, angle float64) {
	type cand struct {
		sa, sb   int
		angle, l float64
		opposite bool
	}
	var best, bestOpp *cand
	better := func(x, y *cand) bool { // x が y より良いか
		if y == nil {
			return true
		}
		if math.Abs(x.angle-y.angle) > 1e-6 {
			return x.angle > y.angle
		}
		if x.opposite != y.opposite {
			return x.opposite
		}
		return x.l < y.l
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			_, x1, y1 := connectorSite(ka, a, i)
			_, x2, y2 := connectorSite(kb, b, j)
			dx, dy := x2-x1, y2-y1
			l := math.Hypot(dx, dy)
			if l == 0 {
				continue
			}
			na, nb := sideNormals[i], sideNormals[j]
			outA := (na[0]*dx + na[1]*dy) / l  // a の辺から外へ出る成分
			outB := -(nb[0]*dx + nb[1]*dy) / l // b の辺へ外から入る成分
			if outA <= 1e-9 || outB <= 1e-9 {
				continue
			}
			c := &cand{sa: i, sb: j, l: l, opposite: j == (i+2)%4,
				angle: math.Min(math.Asin(math.Min(outA, 1)), math.Asin(math.Min(outB, 1))) * 180 / math.Pi}
			if better(c, best) {
				best = c
			}
			if c.opposite && better(c, bestOpp) {
				bestOpp = c
			}
		}
	}
	if bestOpp != nil && bestOpp.angle >= connectorAngleMin {
		return bestOpp.sa, bestOpp.sb, bestOpp.angle
	}
	if best != nil {
		return best.sa, best.sb, best.angle
	}
	// 図形が重なっていて、相手を向いた辺が無い。以前と同じく中心の向きで選ぶ。
	if math.Abs((b.X+b.Width/2)-(a.X+a.Width/2)) >= math.Abs((b.Y+b.Height/2)-(a.Y+a.Height/2)) {
		if b.X+b.Width/2 >= a.X+a.Width/2 {
			return 3, 1, 0
		}
		return 1, 3, 0
	}
	if b.Y+b.Height/2 >= a.Y+a.Height/2 {
		return 2, 0, 0
	}
	return 0, 2, 0
}

func (s *ShapesSubsystem) Render(ctx RenderContext, box LBox) {
	v := s.vals()
	in := s.inner(box)
	p := ctx.Out
	type placed struct {
		id   int
		kind string
		b    LBox
	}
	byID := map[string]placed{}
	pt := func(q [2]float64) (float64, float64) { return in.X + q[0]*in.Width, in.Y + q[1]*in.Height }

	// 図形と線を書いた順に描き、コネクタは最後に(図形の上に)描く。
	for i := range s.Data.Items {
		it := &s.Data.Items[i]
		switch it.Kind {
		case "rect", "roundRect", "ellipse":
			b := itemBox(in, it)
			o := ShapeOpts{Prst: it.Kind, Fill: ctx.Roles.Color(it.Fill), OutlineWidthPt: v.OutlineWidth,
				FontSize: s.textSize(it.TextStyle), InsetX: v.TextInsetX, InsetY: v.TextInsetY}
			if it.Line != "" {
				o.Outline = ctx.Roles.Color(it.Line)
			}
			if it.Kind == "roundRect" {
				o.Radius = v.CornerRadius
			}
			if len(it.Text) > 0 {
				o.Runs = itemRuns(it.Text, it.TextColor, ctx.Roles)
			}
			id := p.AddShape(b, o)
			if it.ID != "" {
				byID[it.ID] = placed{id, it.Kind, b}
			}
		case "text":
			b := itemBox(in, it)
			p.AddRichTextBox(b, itemRuns(it.Text, it.TextColor, ctx.Roles), s.textSize(it.TextStyle), "left", "top")
			if it.ID != "" {
				byID[it.ID] = placed{p.Shapes[len(p.Shapes)-1].id, it.Kind, b}
			}
		case "line", "arrow":
			x1, y1 := pt(it.From)
			x2, y2 := pt(it.To)
			p.AddConnector(x1, y1, x2, y2, v.LineWidth, ctx.Roles.Color(it.Color), it.Kind == "arrow", ConnectorEnd{}, ConnectorEnd{})
		}
	}
	for i := range s.Data.Items {
		it := &s.Data.Items[i]
		if it.Kind != "connector" {
			continue
		}
		a, okA := byID[it.FromID]
		b, okB := byID[it.ToID]
		if !okA || !okB {
			continue // 読み込みで断っているので来ない
		}
		sa, sb, _ := connectorSides(a.kind, b.kind, a.b, b.b)
		ia, x1, y1 := connectorSite(a.kind, a.b, sa)
		ib, x2, y2 := connectorSite(b.kind, b.b, sb)
		p.AddConnector(x1, y1, x2, y2, v.LineWidth, ctx.Roles.Color(it.Color), it.Arrow,
			ConnectorEnd{a.id, ia}, ConnectorEnd{b.id, ib})
	}
}

// --- 描く前の検査 ---

// shapesOverlap は相対の矩形が面積を持って重なるか(辺が接するだけなら重ならない)。
func shapesOverlap(a, b *ShapeItem) bool {
	const eps = 1e-9
	return a.X+eps < b.X+b.W && b.X+eps < a.X+a.W && a.Y+eps < b.Y+b.H && b.Y+eps < a.Y+a.H
}

// Lint は図形どうしの重なり(shapes-overlap)と、図形の文が収まらないこと
// (shapes-text-overflow)と、コネクタが辺に浅くしか繋げないこと
// (shapes-connector-angle)を警告にする。線・矢印・コネクタは重なりを見ない。
func (s *ShapesSubsystem) Lint(box LBox) []string {
	var out []string
	items := s.Data.Items
	for i := range items {
		if !items[i].boxed() {
			continue
		}
		for j := 0; j < i; j++ {
			if items[j].boxed() && shapesOverlap(&items[i], &items[j]) {
				out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] shapes: items[%d] (%q) overlaps items[%d] (%q)",
					i, items[i].label(), j, items[j].label()))
			}
		}
	}
	in := s.inner(box)
	for i := range items {
		it := &items[i]
		if !it.boxed() || len(it.Text) == 0 {
			continue
		}
		needW, needH, haveW, haveH := s.textNeed(it, itemBox(in, it))
		const eps = 0.5
		if needW > haveW+eps || needH > haveH+eps {
			out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] shapes: items[%d] (%q): text does not fit (needs %.1fx%.1fpt, has %.1fx%.1fpt)",
				i, it.label(), needW, needH, math.Max(haveW, 0), math.Max(haveH, 0)))
		}
	}
	// コネクタがどの辺の組でも浅くしか繋げない(辺をかすめて角で終わったように見える)。
	byID := map[string]int{}
	for i := range items {
		if items[i].boxed() && items[i].ID != "" {
			byID[items[i].ID] = i
		}
	}
	for i := range items {
		it := &items[i]
		if it.Kind != "connector" {
			continue
		}
		ja, okA := byID[it.FromID]
		jb, okB := byID[it.ToID]
		if !okA || !okB {
			continue
		}
		ba, bb := itemBox(in, &items[ja]), itemBox(in, &items[jb])
		if shapesOverlap(&items[ja], &items[jb]) {
			continue // 重なりは shapes-overlap が知らせる
		}
		if _, _, ang := connectorSides(items[ja].Kind, items[jb].Kind, ba, bb); ang < connectorAngleMin {
			out = append(out, fmt.Sprintf("[SUBSYSTEM WARNING] shapes: items[%d] (connector %s -> %s): meets a side at a shallow angle (%.0f degrees < %.0f), so it looks like it ends at a corner",
				i, it.FromID, it.ToID, ang, connectorAngleMin))
		}
	}
	return out
}

// --- IR から読む ---

type shapesIR struct {
	Aspect *float64          `json:"aspect"`
	Items  []json.RawMessage `json:"items"`
}

type shapeItemIR struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	X         *float64        `json:"x"`
	Y         *float64        `json:"y"`
	W         *float64        `json:"w"`
	H         *float64        `json:"h"`
	From      json.RawMessage `json:"from"`
	To        json.RawMessage `json:"to"`
	Arrow     bool            `json:"arrow"`
	Fill      string          `json:"fill"`
	Line      string          `json:"line"`
	Color     string          `json:"color"`
	Text      Inline          `json:"text"`
	TextStyle string          `json:"textStyle"`
	TextColor string          `json:"textColor"`
}

// shapesFromIR は IR の data を読む。形の誤りに加えて、組み立てられないもの
// (範囲の外の座標・無い id へのコネクタ・id の重複)も断る。役割名がテーマに
// あるかはテンプレートの Factory が見る(テーマを知っているのはテンプレート)。
func shapesFromIR(raw json.RawMessage) (*ShapesData, error) {
	var in shapesIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("shapes: %w", err)
	}
	d := &ShapesData{}
	if in.Aspect != nil {
		if !(*in.Aspect > 0) {
			return nil, fmt.Errorf("shapes: aspect: must be greater than 0 (got %v)", *in.Aspect)
		}
		d.Aspect = *in.Aspect
	}
	if len(in.Items) == 0 {
		return nil, fmt.Errorf("shapes: items needs at least one item")
	}
	ids := map[string]int{}
	for i, r := range in.Items {
		at := fmt.Sprintf("items[%d]", i)
		it, err := shapeItemFromIR(r)
		if err != nil {
			return nil, fmt.Errorf("shapes: %s%w", at, err)
		}
		if it.ID != "" {
			if j, dup := ids[it.ID]; dup {
				return nil, fmt.Errorf("shapes: %s.id: id %q is already used by items[%d]", at, it.ID, j)
			}
			ids[it.ID] = i
		}
		d.Items = append(d.Items, *it)
	}
	for i := range d.Items {
		it := &d.Items[i]
		if it.Kind != "connector" {
			continue
		}
		for _, end := range []struct{ key, id string }{{"from", it.FromID}, {"to", it.ToID}} {
			j, ok := ids[end.id]
			if !ok {
				return nil, fmt.Errorf("shapes: items[%d].%s: no shape has id %q", i, end.key, end.id)
			}
			if !d.Items[j].boxed() {
				return nil, fmt.Errorf("shapes: items[%d].%s: %q is not a shape", i, end.key, end.id)
			}
		}
		if it.FromID == it.ToID {
			return nil, fmt.Errorf("shapes: items[%d].to: a connector must join two different shapes (both ends are %q)", i, it.FromID)
		}
	}
	return d, nil
}

// shapeItemFromIR は1つの item を読む。誤りの文は「.key: …」か「: …」で始める
// (呼ぶ側が items[i] を前に付ける)。
func shapeItemFromIR(r json.RawMessage) (*ShapeItem, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(r, &keys); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	var kind string
	if err := json.Unmarshal(keys["kind"], &kind); err != nil || !contains(shapeKindsAll, kind) {
		return nil, fmt.Errorf(".kind: unknown kind %s (known: %s)", bytes.TrimSpace(keys["kind"]), strings.Join(shapeKindsAll, ", "))
	}
	allowed := shapeAllowedKeys[kind]
	for _, k := range sortedRawKeys(keys) {
		if !contains(allowed, k) {
			return nil, fmt.Errorf(`: json: unknown field %q (a %s takes: %s)`, k, kind, strings.Join(allowed, ", "))
		}
	}
	for _, k := range shapeRequiredKeys[kind] {
		if _, ok := keys[k]; !ok {
			return nil, fmt.Errorf(".%s: a %s needs %s", k, kind, k)
		}
	}
	var v shapeItemIR
	if err := strictUnmarshal(r, &v); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	it := &ShapeItem{ID: v.ID, Kind: kind, Text: v.Text, TextStyle: v.TextStyle,
		Fill: v.Fill, Line: v.Line, TextColor: v.TextColor, Color: v.Color, Arrow: v.Arrow || kind == "arrow"}
	if _, ok := keys["text"]; ok && len(it.Text) == 0 {
		return nil, fmt.Errorf(".text: must not be empty")
	}
	if it.ID != "" && !reID.MatchString(it.ID) {
		return nil, fmt.Errorf(".id: %q is not an id", it.ID)
	}
	if it.TextStyle == "" {
		it.TextStyle = defaultShapeTextStyle
	} else if !contains(shapeTextStyles, it.TextStyle) {
		return nil, fmt.Errorf(".textStyle: unknown text style %q (known: %s)", it.TextStyle, strings.Join(shapeTextStyles, ", "))
	}
	for _, k := range shapeRoleKeys {
		raw, ok := keys[k]
		if !ok {
			continue
		}
		var s string
		json.Unmarshal(raw, &s)
		if !roleNameRe.MatchString(s) || hexColorLikeRe.MatchString(s) {
			return nil, fmt.Errorf(".%s: %q is not a role name (colors are written as theme role names, e.g. accent)", k, s)
		}
	}
	switch kind {
	case "rect", "roundRect", "ellipse":
		if it.Fill == "" {
			it.Fill = defaultShapeFill
		}
		if it.TextColor == "" {
			it.TextColor = defaultShapeText
		}
	case "text":
		// text の文字の色は color で書く。
		it.TextColor, it.Color = v.Color, ""
		if it.TextColor == "" {
			it.TextColor = defaultShapeText
		}
	default:
		if it.Color == "" {
			it.Color = defaultShapeLine
		}
	}

	if it.boxed() {
		for _, f := range []struct {
			key string
			p   *float64
			dst *float64
		}{{"x", v.X, &it.X}, {"y", v.Y, &it.Y}, {"w", v.W, &it.W}, {"h", v.H, &it.H}} {
			if *f.p < 0 || *f.p > 1 {
				return nil, fmt.Errorf(".%s: %v is outside 0..1", f.key, *f.p)
			}
			*f.dst = *f.p
		}
		if it.W <= 0 || it.H <= 0 {
			return nil, fmt.Errorf(": w and h must be greater than 0")
		}
		const eps = 1e-9
		if it.X+it.W > 1+eps {
			return nil, fmt.Errorf(".w: x+w = %v is beyond the frame (must be at most 1)", it.X+it.W)
		}
		if it.Y+it.H > 1+eps {
			return nil, fmt.Errorf(".h: y+h = %v is beyond the frame (must be at most 1)", it.Y+it.H)
		}
		return it, nil
	}
	if kind == "connector" {
		if err := json.Unmarshal(v.From, &it.FromID); err != nil {
			return nil, fmt.Errorf(".from: a connector joins shapes by id: %w", err)
		}
		if err := json.Unmarshal(v.To, &it.ToID); err != nil {
			return nil, fmt.Errorf(".to: a connector joins shapes by id: %w", err)
		}
		return it, nil
	}
	for _, f := range []struct {
		key string
		raw json.RawMessage
		dst *[2]float64
	}{{"from", v.From, &it.From}, {"to", v.To, &it.To}} {
		var pt []float64
		if err := json.Unmarshal(f.raw, &pt); err != nil || len(pt) != 2 {
			return nil, fmt.Errorf(".%s: must be a point [x, y]", f.key)
		}
		for _, c := range pt {
			if c < 0 || c > 1 {
				return nil, fmt.Errorf(".%s: %v is outside 0..1", f.key, c)
			}
		}
		*f.dst = [2]float64{pt[0], pt[1]}
	}
	if it.From == it.To {
		return nil, fmt.Errorf(".to: a %s needs two different points", kind)
	}
	return it, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sortedRawKeys(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
