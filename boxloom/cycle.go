// cycle サブシステム: 輪に並んだ段(拠点づくりの過程・改善の循環など)。
//
// 段を時計回りに、上から始まるドーナツの扇形(本物の図形 blockArc)で並べる。
// 段の題は扇の中に、番号の丸(文を持つ楕円)は扇の真ん中の角度で輪の外の縁に
// 跨がせて置く。補足(detail)があれば、輪の外の、その段の向きに置く。扇の塗りは
// 基調色(accent)の濃淡で、最初の段が一番濃く、後の段ほど地の色(bg)に寄る。
// 強調(emphasis)の段があるときは、強調の段だけを基調色にし、ほかを淡くする。
// 状態(status)は扇の枠線の色。扇の中の文の色は、塗りとの差が大きいほう
// (on_conditional か text_primary)。
//
// 図の形は輪で決まるので縦横比を保つ(FixedAspect)。割り当てられた枠が図より
// 広ければ、余りは spare-width で知らせる(columns で文を横に置けば埋まる)。
// 設計は docs/subsystem.md の11節。IR の data の形は schema/subsystems/cycle.schema.json。

package boxloom

import (
	"fmt"
	"math"
	"strconv"
)

// CycleValues は cycle の寸法(pt。割合・角度は単位なし)。既定は subsystems/cycle.json。
type CycleValues struct {
	Padding         float64 `json:"padding" scale:"space"`
	RingFraction    float64 `json:"ringFraction"` // 輪の太さ(外の半径に対する割合、0.9 まで)
	GapAngle        float64 `json:"gapAngle"`     // 扇と扇の間の角度(度)
	NumberDiameter  float64 `json:"numberDiameter"`
	LabelGap        float64 `json:"labelGap" scale:"space"` // 番号の丸と補足の間
	LabelMinWidth   float64 `json:"labelMinWidth"`          // 補足の枠の幅の下限(枠が狭いときここまで折り返す)
	LabelMaxWidth   float64 `json:"labelMaxWidth"`          // 補足の枠の幅の上限
	NaturalDiameter float64 `json:"naturalDiameter"`        // 大きさの手がかりが無いときの輪の直径
	MinDiameter     float64 `json:"minDiameter"`            // これより小さい輪は読めない
	// OutsideMinDiameter は題を輪の外に置くときの輪の直径の下限(扇に文が無いので
	// minDiameter より小さくできる)。
	OutsideMinDiameter float64 `json:"outsideMinDiameter"`
	ShadeRange         float64 `json:"shadeRange"` // 最後の段の淡さ(基調色を地の色に寄せる割合)
	MutedShade         float64 `json:"mutedShade"` // 強調があるとき、強調でない段の淡さ
	OutlineWidth       float64 `json:"outlineWidth"`
	LegendGap          float64 `json:"legendGap" scale:"space"`
	LegendItemGap      float64 `json:"legendItemGap" scale:"space"`
	LegendSwatchW      float64 `json:"legendSwatchWidth"`
	LegendSwatchH      float64 `json:"legendSwatchHeight"`
	LegendSwatchG      float64 `json:"legendSwatchGap" scale:"space"`
}

func (v *CycleValues) legendValues() figureLegendValues {
	return figureLegendValues{gap: v.LegendGap, itemGap: v.LegendItemGap, swatchW: v.LegendSwatchW, swatchH: v.LegendSwatchH,
		swatchGap: v.LegendSwatchG, outlineW: v.OutlineWidth}
}

// minR は輪の外の半径の下限(題を外に置くときは outsideMinDiameter)。
func (v *CycleValues) minR(outside bool) float64 {
	if outside {
		return math.Min(v.OutsideMinDiameter, v.MinDiameter) / 2
	}
	return v.MinDiameter / 2
}

func (v *CycleValues) ring() float64 { return math.Max(0.05, math.Min(v.RingFraction, 0.9)) }

// CycleData は IR の data を読んだもの。
type CycleData struct {
	Steps  []FigureNode
	Start  int // 最初の段の番号(0 か 1)
	Legend []LegendItem
}

// 段の数の範囲。2段は往復で輪にならず、7段からは扇が細くて題が読めない。
const (
	cycleMinSteps = 3
	cycleMaxSteps = 6
)

// CycleSubsystem は CycleData を描くサブシステム。
type CycleSubsystem struct {
	Data *CycleData
	V    *CycleValues // nil なら既定(subsystems/cycle.json)
	Text *FigureText  // nil なら既定のテンプレート(light)の段

	titleR *float64 // titleRadius の測り値(一度だけ測る)
}

func NewCycleSubsystem(data any) (Subsystem, error) {
	if d, ok := data.(*CycleData); ok {
		return &CycleSubsystem{Data: d}, nil
	}
	raw, err := rawFigureData("cycle", data)
	if err != nil {
		return nil, err
	}
	d, err := cycleFromIR(raw)
	if err != nil {
		return nil, err
	}
	return &CycleSubsystem{Data: d}, nil
}

func (s *CycleSubsystem) Name() string { return "cycle" }

func (s *CycleSubsystem) vals() *CycleValues {
	if s.V != nil {
		return s.V
	}
	return defaultCycleValues()
}

func (s *CycleSubsystem) text() FigureText {
	if s.Text != nil {
		return *s.Text
	}
	return defaultFigureText()
}

func (s *CycleSubsystem) legend() ([]LegendItem, []string) {
	used := map[string]bool{}
	for _, st := range s.Data.Steps {
		used[st.Status] = true
	}
	return figureLegendItems(used, s.Data.Legend)
}

func (s *CycleSubsystem) hasDetails() bool {
	for _, st := range s.Data.Steps {
		if len(st.Detail) > 0 {
			return true
		}
	}
	return false
}

// --- 形 ---

// cycleAngle は段 i の扇の真ん中の角度(ラジアン。右が 0、時計回り。上から始める)。
func cycleAngle(i, n int) float64 {
	return -math.Pi/2 + (float64(i)+0.5)*2*math.Pi/float64(n)
}

// labelSizes は補足の枠の大きさ(幅の上限 cap で折り返す。補足の無い段は 0)。
// 輪の大きさに依らない。outside(題を輪の外に置く)なら枠は題と補足の両方を持つ
// (outsideLabelSizes)。
func (s *CycleSubsystem) labelSizes(cap float64, outside ...bool) []Size {
	if len(outside) > 0 && outside[0] {
		return s.outsideLabelSizes(cap)
	}
	tx := s.text()
	out := make([]Size, len(s.Data.Steps))
	for i, st := range s.Data.Steps {
		if len(st.Detail) == 0 {
			continue
		}
		runs := itemRuns(st.Detail, "", nil)
		w := math.Max(math.Min(runsNaturalWidth(runs, tx.Detail), cap), runsMinWidth(runs, tx.Detail))
		_, h := runsBox(runs, tx.Detail, w)
		// 枠は1字ぶん広くする(アプリの字の幅が測った値より広くても、1行の補足が
		// 折り返さないように。文は輪の側に揃えるので、余りは輪の反対側に出る)。
		out[i] = Size{Width: w + tx.Detail, Height: h}
	}
	return out
}

// outsideLabelSizes は題を輪の外に置くときの枠の大きさ:題(tx.Sub)の段落と、あれば
// 補足(tx.Detail)の段落を、幅の上限 cap で折り返して縦に重ねる。
func (s *CycleSubsystem) outsideLabelSizes(cap float64) []Size {
	tx := s.text()
	out := make([]Size, len(s.Data.Steps))
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		tr := s.titleRuns(st, "", nil)
		nat := runsNaturalWidth(tr, tx.Sub)
		minW := math.Max(runsMinWidth(tr, tx.Sub), runsChunkWidth(tr, tx.Sub))
		var dr []TextRun
		if len(st.Detail) > 0 {
			dr = itemRuns(st.Detail, "", nil)
			nat = math.Max(nat, runsNaturalWidth(dr, tx.Detail))
			minW = math.Max(minW, runsMinWidth(dr, tx.Detail))
		}
		w := math.Max(math.Min(nat, cap), minW)
		out[i] = Size{Width: w + tx.Sub, Height: s.outsideLabelHeight(tr, dr, w)}
	}
	return out
}

// outsideLabelHeight は題と補足を幅 w で折り返した高さ。
func (s *CycleSubsystem) outsideLabelHeight(title, detail []TextRun, w float64) float64 {
	tx := s.text()
	_, h := runsBox(title, tx.Sub, w)
	if len(detail) > 0 {
		_, dh := runsBox(detail, tx.Detail, w)
		h += dh
	}
	return h
}

// cycleLabel は補足の枠(中心からの相対か、置いた後の絶対の座標)と文の置き方。
type cycleLabel struct {
	box           LBox
	align, anchor string
}

// cycleShape は外の半径 r のときの形(中心が原点)。
type cycleShape struct {
	labels                 []cycleLabel
	minX, minY, maxX, maxY float64 // 輪・番号の丸・補足の外側(余白を除く)
}

// labelSideThreshold より向きが横(縦)に寄っていれば、補足を右か左(上か下)に寄せる。
// 間の向き(真上・真下の段)は真ん中に揃える。
const labelSideThreshold = 0.25

func (s *CycleSubsystem) shape(r float64, sizes []Size, outside ...bool) cycleShape {
	v := s.vals()
	out := len(outside) > 0 && outside[0]
	n := len(s.Data.Steps)
	e := r + v.NumberDiameter/2 // 番号の丸は外の縁に跨がる
	sh := cycleShape{minX: -e, minY: -e, maxX: e, maxY: e, labels: make([]cycleLabel, n)}
	d := e + v.LabelGap
	for i := 0; i < n; i++ {
		w, h := sizes[i].Width, sizes[i].Height
		if w == 0 {
			continue
		}
		a := cycleAngle(i, n)
		c, sn := math.Cos(a), math.Sin(a)
		px, py := d*c, d*sn
		l := cycleLabel{}
		switch {
		case c > labelSideThreshold:
			l.box.X, l.align = px, "left"
		case c < -labelSideThreshold:
			l.box.X, l.align = px-w, "right"
		default:
			l.box.X, l.align = px-w/2, "center"
		}
		switch {
		case sn < -labelSideThreshold:
			l.box.Y, l.anchor = py-h, "bottom"
		case sn > labelSideThreshold:
			l.box.Y, l.anchor = py, "top"
		default:
			l.box.Y, l.anchor = py-h/2, "middle"
		}
		l.box.Width, l.box.Height = w, h
		if out {
			// 題を外に置く枠は、輪と番号の丸を囲む正方形(と間 LabelGap)に掛からない
			// ところまで、その段の向きに外へ押し出す。
			l.box = pushOutOfSquare(l.box, c, sn, e+v.LabelGap)
		}
		sh.labels[i] = l
		sh.minX = math.Min(sh.minX, l.box.X)
		sh.minY = math.Min(sh.minY, l.box.Y)
		sh.maxX = math.Max(sh.maxX, l.box.X+w)
		sh.maxY = math.Max(sh.maxY, l.box.Y+h)
	}
	return sh
}

// pushOutOfSquare は枠 b を向き (c, sn) に、中心が原点で半辺 half の正方形と重ならない
// ところまで動かす(はじめから重ならなければそのまま)。
func pushOutOfSquare(b LBox, c, sn, half float64) LBox {
	hits := func(t float64) bool {
		x, y := b.X+t*c, b.Y+t*sn
		return x < half && x+b.Width > -half && y < half && y+b.Height > -half
	}
	if !hits(0) {
		return b
	}
	lo, hi := 0.0, 2*half+b.Width+b.Height
	for k := 0; k < 40; k++ {
		mid := (lo + hi) / 2
		if hits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	b.X += hi * c
	b.Y += hi * sn
	return b
}

// size は外の半径 r のときの図の幅と高さ(余白と凡例を含む)。
func (s *CycleSubsystem) size(r float64, sizes []Size, outside ...bool) (w, h float64) {
	v, tx := s.vals(), s.text()
	sh := s.shape(r, sizes, outside...)
	w = sh.maxX - sh.minX + 2*v.Padding
	h = sh.maxY - sh.minY + 2*v.Padding
	items, _ := s.legend()
	l := layoutFigureLegend(items, math.Max(w-2*v.Padding, 0), tx.Small, v.legendValues())
	for _, p := range l.part(v.legendValues()) {
		h += p.nat
	}
	return w, h
}

// labelCap は幅 limit の枠で補足を折り返す幅:上限のままで最小の輪が入ればそのまま、
// 入らなければ下限まで狭める。
func (s *CycleSubsystem) labelCap(limit float64, outside ...bool) float64 {
	v := s.vals()
	out := len(outside) > 0 && outside[0]
	if !s.hasDetails() && !out {
		return v.LabelMaxWidth
	}
	fits := func(cap float64) bool {
		w, _ := s.size(v.minR(out), s.labelSizes(cap, out), out)
		return w <= limit
	}
	if limit <= 0 || fits(v.LabelMaxWidth) {
		return v.LabelMaxWidth
	}
	lo, hi := v.LabelMinWidth, v.LabelMaxWidth
	if out {
		// 外の題は折れない塊(和文の4字)まで狭められる(outsideLabelSizes が塊より
		// 狭くしない)。
		lo = 0
	}
	if !fits(lo) {
		return lo
	}
	for i := 0; i < 30; i++ {
		mid := (lo + hi) / 2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// radiusFor は図の幅(axis 0)か高さ(axis 1)が limit に収まる一番大きい外の半径
// (最小の直径の半分を下回らない)。
func (s *CycleSubsystem) radiusFor(limit float64, axis int, sizes []Size, outside ...bool) float64 {
	lo := s.vals().minR(len(outside) > 0 && outside[0])
	at := func(r float64) float64 {
		w, h := s.size(r, sizes, outside...)
		if axis == 0 {
			return w
		}
		return h
	}
	if at(lo) >= limit {
		return lo
	}
	hi := math.Max(limit, lo*2)
	for i := 0; i < 50; i++ {
		mid := (lo + hi) / 2
		if at(mid) <= limit {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// Measure: 幅を受けたらその幅での高さ、高さを受けたらその高さでの幅(どちらも
// 輪の大きさを決めてから測る)。形を保つので FixedAspect。最小は題が読める一番
// 小さい形(minLayout:扇に入らなければ、題を輪の外に置くのに要る幅を含む)。
func (s *CycleSubsystem) Measure(c Constraint) Measurement {
	v := s.vals()
	var lay cycleLayout
	switch {
	case c.Width > 0:
		lay = s.layoutFor(c.Width, 0, 0)
	case c.Height > 0:
		lay = s.layoutFor(0, c.Height, 0)
	default:
		// 題が自然の輪の扇に入らなければ、入る大きさの輪を自然の大きさにする。それでも
		// (広げる上限まで)入らなければ、題を輪の外に置いた自然の大きさの輪にする。
		lay = cycleLayout{r: math.Max(v.NaturalDiameter/2, s.titleRadius()), sizes: s.labelSizes(v.LabelMaxWidth)}
		if !s.titlesFit(lay.r) {
			lay = cycleLayout{r: v.NaturalDiameter / 2, sizes: s.labelSizes(v.LabelMaxWidth, true), outside: true}
		}
	}
	w, h := s.size(lay.r, lay.sizes, lay.outside)
	mw, mh := s.minSize(c.Width)
	return Measurement{
		Natural:     Size{Width: w, Height: h},
		Min:         Size{Width: mw, Height: mh},
		FixedAspect: true,
	}
}

// minLayout は題が読める一番小さい形(Measure の Min)。題が最小の輪の扇に入れば
// 最小の輪と下限まで折り返した補足。入らなければ、題が扇に入る輪(titleRadius)と、
// 題を輪の外に置いた最小の輪(題は折れない塊まで折り返す)のうち幅の狭いほう
// (列の中では幅が足りなくなりやすいので、幅で選ぶ)。扇に入る輪が無ければ外。
func (s *CycleSubsystem) minLayout() cycleLayout {
	v := s.vals()
	if s.titlesFit(v.MinDiameter / 2) {
		return cycleLayout{r: v.MinDiameter / 2, sizes: s.labelSizes(v.LabelMinWidth)}
	}
	out := cycleLayout{r: v.minR(true), sizes: s.labelSizes(0, true), outside: true}
	if r := s.titleRadius(); s.titlesFit(r) {
		in := cycleLayout{r: r, sizes: s.labelSizes(v.LabelMinWidth)}
		iw, _ := s.size(in.r, in.sizes)
		ow, _ := s.size(out.r, out.sizes, true)
		if iw <= ow {
			return in
		}
	}
	return out
}

// minSize は Min:幅は minLayout の幅。高さは題が読める形のうち一番低いもの(幅
// width を受けたらその幅で折り返す。題を外に置く形は、折り返しを広くするほど低い)。
// 幅と高さで別の形を取る(狭い列では幅、低い列では高さが効く)。
func (s *CycleSubsystem) minSize(width float64) (w, h float64) {
	v := s.vals()
	ml := s.minLayout()
	w, h = s.size(ml.r, ml.sizes, ml.outside)
	if !ml.outside && ml.r == v.MinDiameter/2 {
		return w, h
	}
	cap := v.LabelMaxWidth
	if width > 0 {
		cap = s.labelCap(math.Max(width, w), true)
	}
	if _, oh := s.size(v.minR(true), s.labelSizes(cap, true), true); oh < h {
		h = oh
	}
	return w, h
}

// columnMinWidth は並列のまとまりの中で要る幅(columnFloor):題が読める一番小さい
// 形の幅。まとまりがこれより狭いと、題は扇にも輪の外にも入らない。
func (s *CycleSubsystem) columnMinWidth() float64 {
	ml := s.minLayout()
	w, _ := s.size(ml.r, ml.sizes, ml.outside)
	return w
}

// cycleLayout は輪の大きさと枠の大きさ、題を輪の外に置くか。
type cycleLayout struct {
	r       float64
	sizes   []Size
	outside bool
}

// layoutFor は幅 width・高さ height(0 なら制約なし。どちらかは正)の枠での形。まず
// 題を扇の中に置いた一番大きい輪を取り、題が扇に収まればそれにする。収まらなければ
// 題を輪の外(番号の丸の外、その段の向き)に置き、輪を小さくして題の置き場を取る。
// 外に置いても枠に入らなければ(最小の輪でもはみ出す)、扇の中に置いたまま(Lint が
// 題の警告を出す)。slack は扇に収まるかを測るときの輪の半径のゆとり(Measure で測った
// 枠に置き直すときの二分探索の誤差で、扇の中と外が入れ替わらないように)。
func (s *CycleSubsystem) layoutFor(width, height, slack float64) cycleLayout {
	v := s.vals()
	radius := func(sizes []Size, out bool) float64 {
		r := math.Inf(1)
		if width > 0 {
			r = s.radiusFor(width, 0, sizes, out)
		}
		if height > 0 {
			r = math.Min(r, s.radiusFor(height, 1, sizes, out))
		}
		return r
	}
	capFor := func(out bool) float64 {
		if width > 0 {
			return s.labelCap(width, out)
		}
		return v.LabelMaxWidth
	}
	in := cycleLayout{sizes: s.labelSizes(capFor(false))}
	in.r = radius(in.sizes, false)
	if s.titlesFit(in.r + slack) {
		return in
	}
	out := cycleLayout{sizes: s.labelSizes(capFor(true), true), outside: true}
	out.r = radius(out.sizes, true)
	w, h := s.size(out.r, out.sizes, true)
	if (width > 0 && w > width+0.01) || (height > 0 && h > height+0.01) {
		return in
	}
	return out
}

// cyclePlaced は box の中に置いた図(絶対の座標)。
type cyclePlaced struct {
	cx, cy, r float64
	outside   bool         // 題を輪の外の枠(labels)に置いた
	labels    []cycleLabel // 補足の無い段は幅 0
	titles    []LBox       // 扇の中の題の枠
	titleRoom []LBox       // 題が扇に収まらなかった段だけ、扇の中に取れた枠(幅 0 なら収まった)
	numbers   []LBox       // 番号の丸
	legend    LBox
	legendL   figureLegendLayout
}

func (s *CycleSubsystem) place(box LBox) *cyclePlaced {
	v, tx := s.vals(), s.text()
	lay := s.layoutFor(box.Width, box.Height, cyclePlaceSlack)
	r := lay.r
	sh := s.shape(r, lay.sizes, lay.outside)
	pl := &cyclePlaced{r: r, outside: lay.outside}
	pl.cx = box.X + v.Padding - sh.minX
	pl.cy = box.Y + v.Padding - sh.minY
	n := len(s.Data.Steps)
	for i, l := range sh.labels {
		l.box.X += pl.cx
		l.box.Y += pl.cy
		pl.labels = append(pl.labels, l)
		a := cycleAngle(i, n)
		d := v.NumberDiameter
		pl.numbers = append(pl.numbers, LBox{X: pl.cx + r*math.Cos(a) - d/2, Y: pl.cy + r*math.Sin(a) - d/2, Width: d, Height: d})
		if lay.outside {
			pl.titles = append(pl.titles, LBox{})
			pl.titleRoom = append(pl.titleRoom, LBox{})
			continue
		}
		tb, room, ok := s.titleBox(i, pl.cx, pl.cy, r)
		pl.titles = append(pl.titles, tb)
		if ok {
			room = LBox{}
		}
		pl.titleRoom = append(pl.titleRoom, room)
	}
	items, _ := s.legend()
	w := sh.maxX - sh.minX
	pl.legendL = layoutFigureLegend(items, w, tx.Small, v.legendValues())
	if pl.legendL.h > 0 {
		pl.legend = LBox{X: box.X + v.Padding, Y: pl.cy + sh.maxY + v.LegendGap, Width: w, Height: pl.legendL.h}
	}
	return pl
}

// cyclePlaceSlack は place が扇に題が収まるかを測るときの半径のゆとり(pt)。
const cyclePlaceSlack = 0.25

// titleBox は段 i の扇の中の題の枠(輪の中心が原点ではなく絶対の座標)。題を1行・
// 2行…と増やしながら、中心の候補(titleCandidates)ごとに、扇(輪の内と外の縁・扇の境目から titleInset 内側)と番号の丸を
// 避けて置ける一番広い横長の枠を探し、題がその行の数に収まる(最後の行が1〜2字だけに
// ならない)最初の枠を返す。枠は折れない塊(runsChunkWidth:欧文の語・和文の4字)より
// 狭くしない(題が1字ずつ縦に折れないように)。
// どれにも収まらなければ ok が false で、一番広い枠(塊の幅より狭ければ塊の幅に広げ、
// 扇の真ん中に置いたもの)を返す。room は扇の中に取れた枠(Lint が警告に書く)。
func (s *CycleSubsystem) titleBox(i int, cx, cy, r float64) (box, room LBox, ok bool) {
	v, tx := s.vals(), s.text()
	n := len(s.Data.Steps)
	a := cycleAngle(i, n)
	t := r * v.ring()
	rm := r - t/2
	half := math.Pi/float64(n) - v.GapAngle*math.Pi/360
	numX, numY := r*math.Cos(a), r*math.Sin(a)
	numR := v.NumberDiameter/2 + titleInset
	inside := func(x, y float64) bool {
		d := math.Hypot(x, y)
		if d < r-t+titleInset || d > r-titleInset {
			return false
		}
		if math.Hypot(x-numX, y-numY) < numR {
			return false
		}
		// 扇の境目からの距離(弧の向きに測る)。
		da := math.Remainder(math.Atan2(y, x)-a, 2*math.Pi)
		return math.Abs(da) <= half && d*math.Sin(half-math.Abs(da)) >= titleInset
	}
	fits := func(x0, y0, w, h float64) bool {
		const k = 12
		for j := 0; j <= k; j++ {
			f := float64(j) / k
			for _, p := range [][2]float64{{x0 + f*w, y0}, {x0 + f*w, y0 + h}, {x0, y0 + f*h}, {x0 + w, y0 + f*h}} {
				if !inside(p[0], p[1]) {
					return false
				}
			}
		}
		return true
	}
	widest := func(px, py, h float64) float64 {
		lo, hi := 0.0, 2*r
		if !fits(px, py-h/2, 0, h) {
			return 0
		}
		for k := 0; k < 30; k++ {
			mid := (lo + hi) / 2
			if fits(px-mid/2, py-h/2, mid, h) {
				lo = mid
			} else {
				hi = mid
			}
		}
		return lo
	}
	runs := s.titleRuns(&s.Data.Steps[i], "", nil)
	chunk := runsChunkWidth(runs, tx.Sub)
	lineH := tx.Sub * lineHeightMult()
	var best LBox
	var loose LBox // 行の数には収まるが最後の行が短いもの
	for lines := 1; lines <= 4; lines++ {
		h := float64(lines) * lineH
		for _, c := range titleCandidates {
			rc, ac := rm-c[0]*t, a+c[1]*half
			px, py := rc*math.Cos(ac), rc*math.Sin(ac)
			w := widest(px, py, h)
			box := LBox{X: cx + px - w/2, Y: cy + py - h/2, Width: w, Height: h}
			if w > best.Width*1.0001 {
				best = box
			}
			if w < chunk {
				continue
			}
			got := wrapRuns(runs, tx.Sub, w)
			if len(got) > lines {
				continue
			}
			if len(got) < 2 || len([]rune(runsText(got[len(got)-1]))) > orphanRunes {
				return box, box, true
			}
			if loose.Width == 0 {
				loose = box
			}
		}
		if loose.Width > 0 {
			return loose, loose, true
		}
	}
	room = best
	if best.Width < chunk {
		// 扇に入らない。1字ずつ折れないよう塊の幅に広げ、扇の真ん中に置く。
		_, h := runsBox(runs, tx.Sub, chunk)
		px, py := rm*math.Cos(a), rm*math.Sin(a)
		best = LBox{X: cx + px - chunk/2, Y: cy + py - h/2, Width: chunk, Height: h}
	}
	return best, room, false
}

// titlesFit は外の半径 r のとき、どの段の題も扇の中に収まるか。
func (s *CycleSubsystem) titlesFit(r float64) bool {
	for i := range s.Data.Steps {
		if _, _, ok := s.titleBox(i, 0, 0, r); !ok {
			return false
		}
	}
	return true
}

// titleRadius は全ての段の題が扇に収まる一番小さい外の半径(最小の直径の半分から、
// 自然の直径の cycleTitleGrow 倍の半分まで探す)。最小の輪で収まれば 0。上限でも
// 収まらなければ上限(題が長すぎる。Lint が警告する)。形の大きさだけで決まるので
// 一度だけ測る。
func (s *CycleSubsystem) titleRadius() float64 {
	if s.titleR != nil {
		return *s.titleR
	}
	v := s.vals()
	lo, hi := v.MinDiameter/2, v.NaturalDiameter*cycleTitleGrow/2
	r := 0.0
	switch {
	case s.titlesFit(lo):
	case !s.titlesFit(hi):
		r = hi
	default:
		for k := 0; k < 20; k++ {
			mid := (lo + hi) / 2
			if s.titlesFit(mid) {
				hi = mid
			} else {
				lo = mid
			}
		}
		r = hi
	}
	s.titleR = &r
	return r
}

// cycleTitleGrow は、題を扇に収めるために自然の直径を広げる上限(自然の直径の倍)。
// これより大きい輪はスライドに入らないので、題を短くするよう警告に任せる。
const cycleTitleGrow = 1.75

// titleCandidates は題の枠の中心の候補(輪の太さに対する内への寄せ、扇の半分の角度に
// 対する回し)。番号の丸から離れる順に試す。
var titleCandidates = [][2]float64{
	{0, 0}, {0.1, 0}, {0.2, 0}, {0, 0.25}, {0, -0.25}, {0.1, 0.25}, {0.1, -0.25},
	{0.3, 0}, {0, 0.45}, {0, -0.45}, {0.15, 0.45}, {0.15, -0.45},
}

// titleInset は扇の縁・番号の丸と題の枠の間(pt)。決まりの数:枠は文字の外形より
// 少し広いので、縁に文字が触れないためのゆとり(見た目の寸法ではない)。
const titleInset = 3.0

// cycleColors は段ごとの扇の塗り(基調色の濃淡)。
func (s *CycleSubsystem) cycleColors(roles Roles) []string {
	v := s.vals()
	accent, bg := roles.Color("accent"), roles.Color("bg")
	n := len(s.Data.Steps)
	anyEmph := false
	for _, st := range s.Data.Steps {
		anyEmph = anyEmph || st.Emphasis
	}
	out := make([]string, n)
	for i, st := range s.Data.Steps {
		switch {
		case anyEmph && st.Emphasis:
			out[i] = accent
		case anyEmph:
			out[i] = mixHex(accent, bg, v.MutedShade)
		default:
			out[i] = mixHex(accent, bg, v.ShadeRange*float64(i)/float64(n-1))
		}
	}
	return out
}

// mixHex は色 a を色 b に t(0〜1)の割合で寄せた色(16進の RRGGBB)。読めない色は a のまま。
func mixHex(a, b string, t float64) string {
	t = math.Max(0, math.Min(1, t))
	pa, oka := parseHexRGB(a)
	pb, okb := parseHexRGB(b)
	if !oka || !okb {
		return a
	}
	var out [3]int
	for k := 0; k < 3; k++ {
		out[k] = int(math.Round(float64(pa[k])*(1-t) + float64(pb[k])*t))
	}
	return fmt.Sprintf("%02X%02X%02X", out[0], out[1], out[2])
}

func parseHexRGB(c string) ([3]int, bool) {
	c = hexColor(c)
	var out [3]int
	if len(c) != 6 {
		return out, false
	}
	for k := 0; k < 3; k++ {
		n, err := strconv.ParseUint(c[2*k:2*k+2], 16, 8)
		if err != nil {
			return out, false
		}
		out[k] = int(n)
	}
	return out, true
}

func contrastRatio(a, b string) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// minTextContrast は文と地の最低のコントラスト比(WCAG 2 の AA、本文の字)。
const minTextContrast = 4.5

// readableRole は地 fill の上に置く文の役割名を選ぶ規則(1か所)。prefer を順に見て
// 地とのコントラスト比が minTextContrast 以上の最初の役割を返す。どれも届かなければ
// prefer と others のうちで比が最も大きい役割。値の表に on 系の役割があれば、呼ぶ側が
// それを prefer の先頭に置く。
func readableRole(fill string, roles Roles, prefer []string, others ...string) string {
	for _, r := range prefer {
		if contrastRatio(fill, roles.Color(r)) >= minTextContrast {
			return r
		}
	}
	best, bestCR := "", -1.0
	for _, r := range append(append([]string{}, prefer...), others...) {
		if cr := contrastRatio(fill, roles.Color(r)); cr > bestCR {
			best, bestCR = r, cr
		}
	}
	return best
}

// onFillRole は塗り fill の上の文の役割名:on_conditional と text_primary のうち、
// 塗りとの差(コントラスト比)が大きいほう。
func onFillRole(fill string, roles Roles) string {
	if contrastRatio(fill, roles.Color("on_conditional")) >= contrastRatio(fill, roles.Color("text_primary")) {
		return "on_conditional"
	}
	return "text_primary"
}

// ooxmlAngle は角度(ラジアン、右が 0、時計回り)を OOXML の 60000 分の1度(0〜360度)にする。
func ooxmlAngle(a float64) int {
	deg := math.Mod(a*180/math.Pi, 360)
	if deg < 0 {
		deg += 360
	}
	return int(math.Round(deg*60000)) % 21600000
}

func (s *CycleSubsystem) number(i int) string { return strconv.Itoa(s.Data.Start + i) }

// titleRuns は扇の中の題の文(強調の段は太字)。
func (s *CycleSubsystem) titleRuns(st *FigureNode, color string, roles Roles) []TextRun {
	runs := itemRuns(st.Title, color, roles)
	if st.Emphasis {
		for k := range runs {
			runs[k].Bold = true
		}
	}
	return runs
}

func (s *CycleSubsystem) Render(ctx RenderContext, box LBox) {
	v, tx := s.vals(), s.text()
	pl := s.place(box)
	p := ctx.Out
	n := len(s.Data.Steps)
	colors := s.cycleColors(ctx.Roles)
	ring := LBox{X: pl.cx - pl.r, Y: pl.cy - pl.r, Width: 2 * pl.r, Height: 2 * pl.r}
	gap := v.GapAngle * math.Pi / 180
	step := 2 * math.Pi / float64(n)
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		a0 := -math.Pi/2 + float64(i)*step + gap/2
		a1 := -math.Pi/2 + float64(i+1)*step - gap/2
		o := ShapeOpts{Prst: "blockArc", Fill: colors[i], OutlineWidthPt: v.OutlineWidth, Adj: []ShapeGuide{
			{"adj1", ooxmlAngle(a0)}, {"adj2", ooxmlAngle(a1)}, {"adj3", int(math.Round(v.ring() * 50000))},
		}}
		if st.Status != "" {
			o.Outline = ctx.Roles.Color(st.Status)
		}
		p.AddShape(ring, o)
	}
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		if pl.outside {
			p.AddShape(pl.numbers[i], ShapeOpts{Prst: "ellipse", Fill: ctx.Roles.Color("bg"), Outline: colors[i],
				OutlineWidthPt: v.OutlineWidth, FontSize: tx.Sub,
				Runs: []TextRun{{Text: s.number(i), Color: ctx.Roles.Color("text_primary"), Bold: true}}})
			l := pl.labels[i]
			o := ShapeOpts{Prst: "rect", FontSize: tx.Sub, Align: l.align, Anchor: l.anchor,
				Runs: boldRuns(s.titleRuns(st, "text_primary", ctx.Roles))}
			if len(st.Detail) > 0 {
				o.Paras, o.ParaSizes = [][]TextRun{itemRuns(st.Detail, "text_muted", ctx.Roles)}, []float64{tx.Detail}
			}
			p.AddShape(l.box, o)
			continue
		}
		p.AddShape(pl.titles[i], ShapeOpts{Prst: "rect", FontSize: tx.Sub,
			Runs: s.titleRuns(st, onFillRole(colors[i], ctx.Roles), ctx.Roles)})
		p.AddShape(pl.numbers[i], ShapeOpts{Prst: "ellipse", Fill: ctx.Roles.Color("bg"), Outline: colors[i],
			OutlineWidthPt: v.OutlineWidth, FontSize: tx.Sub,
			Runs: []TextRun{{Text: s.number(i), Color: ctx.Roles.Color("text_primary"), Bold: true}}})
		if l := pl.labels[i]; l.box.Width > 0 {
			p.AddShape(l.box, ShapeOpts{Prst: "rect", FontSize: tx.Detail, Align: l.align, Anchor: l.anchor,
				Runs: itemRuns(st.Detail, "text_muted", ctx.Roles)})
		}
	}
	drawFigureLegend(ctx, pl.legend.X, pl.legend.Y, pl.legendL, tx.Small, v.legendValues())
}

// Lint は扇の中の題・輪の外の補足が枠に収まらないことと、状態の凡例の文が無いことを
// 警告にする。
func (s *CycleSubsystem) Lint(box LBox) []string {
	tx := s.text()
	pl := s.place(box)
	var out []string
	if pl.outside {
		for i := range s.Data.Steps {
			st := &s.Data.Steps[i]
			l := pl.labels[i].box
			tr := s.titleRuns(st, "", nil)
			var dr []TextRun
			if len(st.Detail) > 0 {
				dr = itemRuns(st.Detail, "", nil)
			}
			needW := runsMinWidth(tr, tx.Sub)
			if len(dr) > 0 {
				needW = math.Max(needW, runsMinWidth(dr, tx.Detail))
			}
			needH := s.outsideLabelHeight(tr, dr, math.Max(l.Width-tx.Sub, needW))
			if !textFits(needW, needH, l.Width, l.Height) {
				out = append(out, overflowWarning("cycle", fmt.Sprintf("steps[%d]", i), st.Title, needW, needH, l.Width, l.Height))
			}
		}
		_, missing := s.legend()
		return append(out, legendMissingWarnings("cycle", missing)...)
	}
	for i := range s.Data.Steps {
		st := &s.Data.Steps[i]
		tb := pl.titles[i]
		runs := s.titleRuns(st, "", nil)
		needW := runsChunkWidth(runs, tx.Sub)
		_, needH := runsBox(runs, tx.Sub, tb.Width)
		if room := pl.titleRoom[i]; room.Width > 0 || room.Height > 0 {
			// 扇に入らない(題は塊の幅で扇の真ん中に置いた)。扇の中に取れた枠を書く。
			out = append(out, overflowWarning("cycle", fmt.Sprintf("steps[%d]", i), st.Title, needW, needH, room.Width, room.Height))
		} else if !textFits(needW, needH, tb.Width, tb.Height) {
			out = append(out, overflowWarning("cycle", fmt.Sprintf("steps[%d]", i), st.Title, needW, needH, tb.Width, tb.Height))
		}
		if l := pl.labels[i].box; l.Width > 0 {
			dr := itemRuns(st.Detail, "", nil)
			needW := runsMinWidth(dr, tx.Detail)
			_, needH := runsBox(dr, tx.Detail, l.Width-tx.Detail)
			if !textFits(needW, needH, l.Width, l.Height) {
				out = append(out, overflowWarning("cycle", fmt.Sprintf("steps[%d].detail", i), st.Detail, needW, needH, l.Width, l.Height))
			}
		}
	}
	_, missing := s.legend()
	return append(out, legendMissingWarnings("cycle", missing)...)
}

// --- IR から読む ---

type cycleIR struct {
	Steps  []figureNodeIR `json:"steps"`
	Start  *int           `json:"start"`
	Legend orderedLegend  `json:"legend"`
}

func cycleFromIR(raw []byte) (*CycleData, error) {
	var in cycleIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("cycle: %w", err)
	}
	if len(in.Steps) < cycleMinSteps || len(in.Steps) > cycleMaxSteps {
		return nil, fmt.Errorf("cycle: steps needs %d to %d steps (has %d)", cycleMinSteps, cycleMaxSteps, len(in.Steps))
	}
	d := &CycleData{Start: 1}
	if in.Start != nil {
		if *in.Start != 0 && *in.Start != 1 {
			return nil, fmt.Errorf("cycle: start: must be 0 or 1 (got %d)", *in.Start)
		}
		d.Start = *in.Start
	}
	for i, st := range in.Steps {
		nd, err := st.node(fmt.Sprintf("steps[%d]", i))
		if err != nil {
			return nil, fmt.Errorf("cycle: %w", err)
		}
		d.Steps = append(d.Steps, nd)
	}
	lg, err := figureLegendIR("cycle", in.Legend)
	if err != nil {
		return nil, err
	}
	d.Legend = lg
	return d, nil
}

func cycleDecl() SubsystemDecl {
	return figureDecl("subsystems/cycle.schema.json",
		"輪に並んだ段(繰り返す過程)。段は3〜6つで、上から時計回りにドーナツの扇形で並ぶ。扇の塗りは基調色の濃淡",
		"各段の番号は start(0 か 1、既定 1)から振る。題は扇の中に入るので短い名前(2〜8字)にする。detail は輪の外の、その段の向きに置く",
		"段ごとの長い説明は図に詰めず、::::columns で図の横のまとまりに書く(cycle は :::column の中に置ける)",
		"status を使ったら legend: {conditional: 意味, …} で枠線の色の意味を書く(凡例が図の下に出る。無ければ figure-legend の警告)",
	)
}
