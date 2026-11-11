// 値の表(データ)の読み込み。
//
// Memory の方針「値の表・スタイル規則・骨組みはデータ、部品とサブシステムは
// コード」のうち、値の表をここで読む。
//
//   - templates/<名前>.json … テンプレートの色(theme)・大きさ(values)・
//     印の文字(marks)・字体(fonts。寸法は fonts/*.json、fonts.go)・
//     変種(variants)、サブシステムの値の差し替え(subsystems)
//   - subsystems/gantt.json  … Gantt の既定の寸法(テンプレートが差し替えなければ
//     これを使う)
//
// どちらもバイナリに同梱する(embed)。読むときは厳しく見る: 知らないキー・
// 欠けたキー・型の違い・色の形の誤り・0 以下の大きさは、すべて誤りにする
// (黙って既定値で埋めない)。形は docs/template-values.md。

package boxloom

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed templates/*.json subsystems/*.json
var valueFiles embed.FS

// TemplateValues はテンプレートの大きさの値の表(pt。PointIndentEm は字の
// 大きさに掛ける倍率、FillRatioMin は割合)。JSON のキーは json タグの名前で、
// 全部そろっていなければ誤り。
type TemplateValues struct {
	PageW float64 `json:"pageWidth"`
	PageH float64 `json:"pageHeight"`

	PagePadding  float64 `json:"pagePadding" scale:"space"`  // 本文のスライドの外周
	CoverPadding float64 `json:"coverPadding" scale:"space"` // 表紙の外周
	SectionGap   float64 `json:"sectionGap" scale:"space"`   // 見出し・本文・フッターの間
	BlockGap     float64 `json:"blockGap" scale:"space"`     // 本文のブロックの間
	ItemGap      float64 `json:"itemGap" scale:"space"`      // 箇条の項目の間

	CoverTitle    float64 `json:"coverTitle" scale:"type"`
	CoverSubtitle float64 `json:"coverSubtitle" scale:"type"`
	CoverAuthor   float64 `json:"coverAuthor" scale:"type"`
	SlideTitle    float64 `json:"slideTitle" scale:"type"`
	Body          float64 `json:"body" scale:"type"`
	SubPoint      float64 `json:"subPoint" scale:"type"`    // 入れ子の箇条
	OptionLabel   float64 `json:"optionLabel" scale:"type"` // 比較の選択肢の名前
	OptionItem    float64 `json:"optionItem" scale:"type"`  // 比較の利点・欠点
	Small         float64 `json:"small" scale:"type"`       // 注記・フッター

	PointIndentEm float64 `json:"pointIndentEm"` // 入れ子の箇条の字下げ(親の字の大きさの倍)

	CardPadding     float64 `json:"cardPadding" scale:"space"`
	CardRadius      float64 `json:"cardRadius"`
	SubsystemRadius float64 `json:"subsystemRadius"` // サブシステムを載せるカードの角

	CoverRuleW float64 `json:"coverRuleWidth"` // 表紙の題の下の線
	CoverRuleH float64 `json:"coverRuleHeight"`

	FillRatioMin float64 `json:"fillRatioMin"` // 本文の埋まりがこれ未満なら警告
}

// TemplateScale は値の表の段階(見た目のトークンの刻み)。
//
//   - SpacingUnit:余白・間隔(json タグに scale:"space" の付いた値)はこの倍数だけ
//   - TypeSizes:字の大きさ(scale:"type" の付いた値)はこの中の値だけ。小さい順
//
// 値の表(values)と、このテンプレートで使うサブシステムの寸法(Gantt・shapes)の
// どちらにも効く。段から外れた値は読み込みで誤り(docs/template-values.md)。
type TemplateScale struct {
	SpacingUnit float64   `json:"spacingUnit"`
	TypeSizes   []float64 `json:"typeSizes"`
}

// TemplateMarks は部品が文の頭に置く印の文字(スタイル規則)。
type TemplateMarks struct {
	Point    string `json:"point"`
	SubPoint string `json:"subPoint"`
	Pro      string `json:"pro"`
	Con      string `json:"con"`
}

// GanttValues は Gantt サブシステムの寸法(pt)。既定は subsystems/gantt.json。
type GanttValues struct {
	Padding      float64 `json:"padding" scale:"space"`   // 箱の内側の余白
	LegendH      float64 `json:"legendHeight"`            // 凡例の行
	LegendGap    float64 `json:"legendGap" scale:"space"` // 凡例と上の注釈のレーンの間
	GutterGroupW float64 `json:"gutterGroupWidth"`
	GutterSubW   float64 `json:"gutterSubgroupWidth"`
	GutterLabelW float64 `json:"gutterLabelWidth"`
	MilestoneH   float64 `json:"milestoneHeight"`
	AxisLabelH   float64 `json:"axisLabelHeight"`
	GroupGap     float64 `json:"groupGap" scale:"space"`

	MinRowH       float64 `json:"minRowHeight"`     // 文字とバーが重ならない下限
	NaturalRowH   float64 `json:"naturalRowHeight"` // 詰まって見えない高さ
	MaxRowH       float64 `json:"maxRowHeight"`     // これより高くしても読みやすくならない
	NaturalChartW float64 `json:"naturalChartWidth"`
	MinChartW     float64 `json:"minChartWidth"` // これより狭いと目盛りの文字が重なる

	AnnBoxW    float64 `json:"annotationFixedWidth"` // 手で置いた注釈の箱(固定)
	AnnBoxH    float64 `json:"annotationFixedHeight"`
	AnnBoxMinW float64 `json:"annotationMinWidth"` // 自動の注釈の箱の幅の範囲
	AnnBoxMaxW float64 `json:"annotationMaxWidth"`
	AnnPadX    float64 `json:"annotationPaddingX" scale:"space"`
	AnnPadY    float64 `json:"annotationPaddingY" scale:"space"`
	AnnLaneGap float64 `json:"annotationLaneGap" scale:"space"`
	AnnText    float64 `json:"annotationText" scale:"type"`

	LabelText float64 `json:"labelText" scale:"type"` // 凡例・行の名前・目盛り
	SmallText float64 `json:"smallText" scale:"type"` // グループ名・区間の文字・節目
}

// ShapesValues は shapes サブシステムの寸法(pt。DefaultAspect は幅÷高さ)。
// 既定は subsystems/shapes.json。
type ShapesValues struct {
	Padding       float64 `json:"padding" scale:"space"` // 枠の内側の余白(相対座標の 0〜1 はその内側)
	NaturalWidth  float64 `json:"naturalWidth"`          // 幅が分からないときの自然な幅
	DefaultAspect float64 `json:"defaultAspect"`         // aspect を省いたときの内側の縦横比
	MinWidth      float64 `json:"minWidth"`
	MinHeight     float64 `json:"minHeight"`
	TextInsetX    float64 `json:"textInsetX" scale:"space"` // 図形の中の文字の余白
	TextInsetY    float64 `json:"textInsetY" scale:"space"`
	LineWidth     float64 `json:"lineWidth"`    // line・arrow・connector の太さ
	OutlineWidth  float64 `json:"outlineWidth"` // 図形の枠線の太さ
	CornerRadius  float64 `json:"cornerRadius"` // roundRect の角
}

// templateFile は templates/<名前>.json の形。
type templateFile struct {
	Name     string            `json:"name"`
	Theme    map[string]string `json:"theme"`
	Scale    TemplateScale     `json:"scale"`
	Values   TemplateValues    `json:"values"`
	Marks    TemplateMarks     `json:"marks"`
	Fonts    TemplateFonts     `json:"fonts"`
	Variants []string          `json:"variants"`
	// Subsystems はサブシステムの値の差し替え(任意)。書くなら全部の値を書く。
	Subsystems *templateSubsystems `json:"subsystems,omitempty"`
}

type templateSubsystems struct {
	Gantt  *GanttValues  `json:"gantt,omitempty"`
	Shapes *ShapesValues `json:"shapes,omitempty"`
}

// themeRoles はテーマが持たなければならない役割名(コア・デモのテンプレートの
// 部品・Gantt が使うもの)。これ以外の役割名は誤り。
var themeRoles = []string{
	"bg", "card", "card_alt", "text_primary", "text_muted", "grid",
	"support", "conditional", "none", "danger", "risk", "accent", "on_conditional",
}

var hexColorRe = regexp.MustCompile(`^[0-9A-F]{6}$`)

// decodeStrict は data を v に読む。知らないキー・欠けたキー(json タグに
// omitempty の無いもの)・null・後ろに続くゴミは誤り。
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("unexpected data after the top-level value")
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return requireKeys(reflect.TypeOf(v).Elem(), raw, "")
}

// requireKeys は構造体の json タグのキーが raw に全部あるかを見る。
func requireKeys(t reflect.Type, raw any, at string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil // 型の違いは Decode が先に断る
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			child := name
			if at != "" {
				child = at + "." + name
			}
			val, present := obj[name]
			if !present {
				if opts == "omitempty" {
					continue
				}
				return fmt.Errorf("missing key %q", child)
			}
			if val == nil {
				return fmt.Errorf("key %q must not be null", child)
			}
			if err := requireKeys(f.Type, val, child); err != nil {
				return err
			}
		}
	case reflect.Slice:
		arr, _ := raw.([]any)
		for i, e := range arr {
			if err := requireKeys(t.Elem(), e, fmt.Sprintf("%s[%d]", at, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseTemplateFile は templates/<名前>.json の中身を読んで検査する。
func parseTemplateFile(name string, data []byte) (*Template, error) {
	var f templateFile
	if err := decodeStrict(data, &f); err != nil {
		return nil, err
	}
	if f.Name != name {
		return nil, fmt.Errorf("name %q does not match the file name %q", f.Name, name)
	}
	known := map[string]bool{}
	for _, r := range themeRoles {
		known[r] = true
		if _, ok := f.Theme[r]; !ok {
			return nil, fmt.Errorf("theme: missing role %q", r)
		}
	}
	for _, r := range sortedKeys(f.Theme) {
		if !known[r] {
			return nil, fmt.Errorf("theme: unknown role %q (known: %v)", r, themeRoles)
		}
		if !hexColorRe.MatchString(f.Theme[r]) {
			return nil, fmt.Errorf("theme.%s: %q is not a color (6 upper-case hex digits, e.g. 0033CC)", r, f.Theme[r])
		}
	}
	if err := checkPositive("values", f.Values); err != nil {
		return nil, err
	}
	if err := checkScaleDef(f.Scale); err != nil {
		return nil, err
	}
	if err := checkOnScale("values", f.Values, f.Scale); err != nil {
		return nil, err
	}
	if err := checkMarks(f.Marks); err != nil {
		return nil, err
	}
	fonts, err := resolveFonts(f.Fonts)
	if err != nil {
		return nil, err
	}
	variants := map[string]bool{}
	for _, v := range f.Variants {
		if v == "" || variants[v] {
			return nil, fmt.Errorf("variants: %q is empty or appears twice", v)
		}
		variants[v] = true
	}
	t := &Template{
		Name:       name,
		Theme:      Theme(f.Theme),
		Scale:      f.Scale,
		Values:     f.Values,
		Marks:      f.Marks,
		Fonts:      fonts,
		Subsystems: DefaultSubsystems().Clone(),
		Variants:   variants,
		// 名乗る囲みは部品(コード)と対になるので、まだコードの一式を使う。
		Blocks: defaultBlocks(),
	}
	// サブシステムの寸法も、このテンプレートの段に載っていなければならない
	// (差し替えないときは既定の寸法を見る)。
	if f.Subsystems == nil || f.Subsystems.Gantt == nil {
		if err := checkOnScale("subsystems/gantt.json", *defaultGanttValues(), f.Scale); err != nil {
			return nil, err
		}
	}
	if f.Subsystems == nil || f.Subsystems.Shapes == nil {
		if err := checkOnScale("subsystems/shapes.json", *defaultShapesValues(), f.Scale); err != nil {
			return nil, err
		}
	}
	if f.Subsystems != nil && f.Subsystems.Gantt != nil {
		gv := *f.Subsystems.Gantt
		if err := checkPositive("subsystems.gantt", gv); err != nil {
			return nil, err
		}
		if err := checkOnScale("subsystems.gantt", gv, f.Scale); err != nil {
			return nil, err
		}
		must(t.Subsystems.Override("gantt", func(data any) (Subsystem, error) {
			s, err := NewGanttSubsystem(data)
			if err != nil {
				return nil, err
			}
			s.(*GanttSubsystem).V = &gv
			return s, nil
		}))
	}
	// shapes は字の段をこのテンプレートの values から引き、役割名をこのテーマで確かめる。
	var sv *ShapesValues
	if f.Subsystems != nil && f.Subsystems.Shapes != nil {
		v := *f.Subsystems.Shapes
		if err := checkPositive("subsystems.shapes", v); err != nil {
			return nil, err
		}
		if err := checkOnScale("subsystems.shapes", v, f.Scale); err != nil {
			return nil, err
		}
		sv = &v
	}
	sizes, theme := shapeTextSizes(f.Values), t.Theme
	must(t.Subsystems.Override("shapes", func(data any) (Subsystem, error) {
		s, err := NewShapesSubsystem(data)
		if err != nil {
			return nil, err
		}
		ss := s.(*ShapesSubsystem)
		ss.V, ss.TextSizes = sv, sizes
		if err := ss.checkRoles(theme); err != nil {
			return nil, err
		}
		return ss, nil
	}))
	return t, nil
}

// checkPositive は値の表の数が全部 0 より大きいことを見る(打ち間違えた値を
// 早く見つけるため)。
func checkPositive(at string, v any) error {
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if rv.Field(i).Kind() != reflect.Float64 {
			continue
		}
		if x := rv.Field(i).Float(); !(x > 0) {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			return fmt.Errorf("%s.%s: must be greater than 0 (got %v)", at, name, x)
		}
	}
	return nil
}

// checkScaleDef は段階そのものを見る:刻みは 0 より大きく、字の大きさの段は
// 1つ以上で、0 より大きく、小さい順に重ならずに並ぶ。
func checkScaleDef(sc TemplateScale) error {
	if !(sc.SpacingUnit > 0) {
		return fmt.Errorf("scale.spacingUnit: must be greater than 0 (got %v)", sc.SpacingUnit)
	}
	if len(sc.TypeSizes) == 0 {
		return fmt.Errorf("scale.typeSizes: must list at least one size")
	}
	for i, x := range sc.TypeSizes {
		if !(x > 0) {
			return fmt.Errorf("scale.typeSizes[%d]: must be greater than 0 (got %v)", i, x)
		}
		if i > 0 && !(x > sc.TypeSizes[i-1]) {
			return fmt.Errorf("scale.typeSizes[%d]: sizes must be in increasing order without repeats (%v after %v)", i, x, sc.TypeSizes[i-1])
		}
	}
	return nil
}

const scaleEps = 1e-9

// checkOnScale は json タグに scale:"space" / scale:"type" の付いた値が段に
// 載っているかを見る。外れていれば、近い段の値を添えて誤りにする。
func checkOnScale(at string, v any, sc TemplateScale) error {
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		kind := rt.Field(i).Tag.Get("scale")
		if kind == "" {
			continue
		}
		x := rv.Field(i).Float()
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		switch kind {
		case "space":
			q := x / sc.SpacingUnit
			if math.Abs(q-math.Round(q)) > scaleEps {
				lo := math.Floor(q) * sc.SpacingUnit
				return fmt.Errorf("%s.%s: %v is not on the spacing scale (multiples of %v); nearest: %v or %v",
					at, name, x, sc.SpacingUnit, lo, lo+sc.SpacingUnit)
			}
		case "type":
			on := false
			for _, t := range sc.TypeSizes {
				if math.Abs(t-x) <= scaleEps {
					on = true
				}
			}
			if !on {
				return fmt.Errorf("%s.%s: %v is not on the type scale %v; nearest: %s",
					at, name, x, sc.TypeSizes, nearestSizes(x, sc.TypeSizes))
			}
		default:
			panic("unknown scale tag " + kind)
		}
	}
	return nil
}

// nearestSizes は x をはさむ段の値(端なら1つ)を返す。
func nearestSizes(x float64, sizes []float64) string {
	for i, t := range sizes {
		if t > x {
			if i == 0 {
				return fmt.Sprint(t)
			}
			return fmt.Sprintf("%v or %v", sizes[i-1], t)
		}
	}
	return fmt.Sprint(sizes[len(sizes)-1])
}

func checkMarks(m TemplateMarks) error {
	rv, rt := reflect.ValueOf(m), reflect.TypeOf(m)
	for i := 0; i < rt.NumField(); i++ {
		if rv.Field(i).String() == "" {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			return fmt.Errorf("marks.%s: must not be empty", name)
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// TemplateNames は LookupTemplate が引けるテンプレートの名前(同梱した
// templates/*.json から作る。並べたもの。既定は light)。
var TemplateNames = bundledTemplateNames()

func bundledTemplateNames() []string {
	ents, _ := valueFiles.ReadDir("templates")
	var names []string
	for _, e := range ents {
		if n, ok := strings.CutSuffix(e.Name(), ".json"); ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// loadTemplate は同梱したテンプレートを読む。
func loadTemplate(name string) (*Template, error) {
	file := path.Join("templates", name+".json")
	data, err := valueFiles.ReadFile(file)
	if err != nil || strings.ContainsAny(name, "/\\") {
		return nil, unknownTemplateError{name}
	}
	t, err := parseTemplateFile(name, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return t, nil
}

// mustTemplate は同梱したテンプレートを読む(読めなければ壊れたバイナリ)。
func mustTemplate(name string) *Template {
	t, err := loadTemplate(name)
	if err != nil {
		panic(err)
	}
	return t
}

func themeDark() Theme  { return mustTemplate("dark").Theme }
func themeLight() Theme { return mustTemplate("light").Theme }

var (
	ganttDefaultsOnce sync.Once
	ganttDefaults     GanttValues
)

// parseGanttValues は Gantt の値の表を読んで検査する。
func parseGanttValues(data []byte) (GanttValues, error) {
	var gv GanttValues
	if err := decodeStrict(data, &gv); err != nil {
		return gv, err
	}
	return gv, checkPositive("gantt", gv)
}

// defaultGanttValues は subsystems/gantt.json(Gantt の既定の寸法)の写し。
func defaultGanttValues() *GanttValues {
	ganttDefaultsOnce.Do(func() {
		data, err := valueFiles.ReadFile("subsystems/gantt.json")
		if err == nil {
			ganttDefaults, err = parseGanttValues(data)
		}
		if err != nil {
			panic(fmt.Errorf("subsystems/gantt.json: %w", err))
		}
	})
	gv := ganttDefaults
	return &gv
}

var (
	shapesDefaultsOnce sync.Once
	shapesDefaults     ShapesValues
)

// defaultShapesValues は subsystems/shapes.json(shapes の既定の寸法)の写し。
func defaultShapesValues() *ShapesValues {
	shapesDefaultsOnce.Do(func() {
		data, err := valueFiles.ReadFile("subsystems/shapes.json")
		if err == nil {
			err = decodeStrict(data, &shapesDefaults)
		}
		if err == nil {
			err = checkPositive("shapes", shapesDefaults)
		}
		if err != nil {
			panic(fmt.Errorf("subsystems/shapes.json: %w", err))
		}
	})
	sv := shapesDefaults
	return &sv
}
