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
	"strconv"
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

	PagePadding  float64 `json:"pagePadding" scale:"space"`  // 本文・主張のスライドの左右の外周
	PagePaddingY float64 `json:"pagePaddingY" scale:"space"` // 本文・主張のスライドの上下の外周
	CoverPadding float64 `json:"coverPadding" scale:"space"` // 表紙の外周
	SectionGap   float64 `json:"sectionGap" scale:"space"`   // 見出し・本文・フッターの間
	BlockGap     float64 `json:"blockGap" scale:"space"`     // 本文のブロックの間
	ItemGap      float64 `json:"itemGap" scale:"space"`      // 箇条の項目の間
	HeaderGap    float64 `json:"headerGap" scale:"space"`    // 題とリード文の間

	CoverTitle    float64 `json:"coverTitle" scale:"type"`
	CoverSubtitle float64 `json:"coverSubtitle" scale:"type"`
	CoverAuthor   float64 `json:"coverAuthor" scale:"type"`
	SlideTitle    float64 `json:"slideTitle" scale:"type"`
	LeadText      float64 `json:"leadText" scale:"type"` // リード文(題の下の文)
	Body          float64 `json:"body" scale:"type"`
	SubPoint      float64 `json:"subPoint" scale:"type"`     // 入れ子の箇条
	OptionLabel   float64 `json:"optionLabel" scale:"type"`  // 比較の選択肢の名前
	OptionItem    float64 `json:"optionItem" scale:"type"`   // 比較の利点・欠点
	Small         float64 `json:"small" scale:"type"`        // 注記・フッター・図の凡例
	FigureDetail  float64 `json:"figureDetail" scale:"type"` // 図の補足(箱の2段目・段階の下・図の説明)
	CaptionText   float64 `json:"captionText" scale:"type"`  // 表・画像の説明の文

	SectionTitle float64 `json:"sectionTitle" scale:"type"` // 区切り(section)の章の題
	// SectionTracker が true なら、区切りが2つ以上ある文書の区切りのスライドに
	// 章の一覧と現在地を置く(SectionTrackerText はその字の大きさ)。
	SectionTracker     bool    `json:"sectionTracker"`
	SectionTrackerText float64 `json:"sectionTrackerText" scale:"type"`
	// SectionTrackerPadding は章の一覧の題の上下左右の余白(左の帯との間も)。
	SectionTrackerPadding float64 `json:"sectionTrackerPadding" scale:"space"`
	// TocIndent は目次(meta.toc)の節(level 2)の字下げ。
	TocIndent     float64 `json:"tocIndent" scale:"space"`
	StatementText float64 `json:"statementText" scale:"type"` // 主張(statement)の1文
	TakeawayText  float64 `json:"takeawayText" scale:"type"`  // 結論(takeaway)の1文
	StatValue     float64 `json:"statValue" scale:"type"`     // 統計強調(stat)の数
	StatUnit      float64 `json:"statUnit" scale:"type"`      // 数の単位
	StatLabel     float64 `json:"statLabel" scale:"type"`     // 数の説明
	QuoteText     float64 `json:"quoteText" scale:"type"`     // 引用(quote)の言葉
	QuoteMark     float64 `json:"quoteMark" scale:"type"`     // 引用の頭の印
	QuoteSource   float64 `json:"quoteSource" scale:"type"`   // 引用の出どころ

	PointIndentEm float64 `json:"pointIndentEm" unit:"em"` // 入れ子の箇条の字下げ(親の字の大きさの倍)。番号つきの箇条の番号の幅も
	// OrderedStyle は番号つきの箇条(points の ordered)の1段目の番号の形
	// (orderedStyles のどれか。PPTX の a:buAutoNum の type)。入れ子の段は arabicPeriod。
	OrderedStyle string `json:"orderedStyle"`

	CardPadding     float64 `json:"cardPadding" scale:"space"`
	CardRadius      float64 `json:"cardRadius"`
	SubsystemRadius float64 `json:"subsystemRadius"` // サブシステムを載せるカードの角

	CoverRuleW float64 `json:"coverRuleWidth"` // 表紙の題の下の線(区切り・主張の文の上の線も)
	CoverRuleH float64 `json:"coverRuleHeight"`

	StatementInset float64 `json:"statementInset" scale:"space"` // 主張のスライドの文を本文の外周からさらに内へ
	AccentBarWidth float64 `json:"accentBarWidth"`               // 結論の左の帯の幅

	FillRatioMin float64 `json:"fillRatioMin" unit:"ratio"` // 本文の埋まりがこれ未満なら警告

	ColumnTitle      float64 `json:"columnTitle" scale:"type"`      // 並列のまとまりの題
	ColumnGap        float64 `json:"columnGap" scale:"space"`       // まとまりの間
	ColumnMainWeight float64 `json:"columnMainWeight" unit:"ratio"` // 主のまとまり(role: main)の幅の比(ほかは 1)

	TableText      float64 `json:"tableText" scale:"type"`          // 表のセルの字
	TableCellPadX  float64 `json:"tableCellPaddingX" scale:"space"` // セルの左右の余白
	TableCellPadY  float64 `json:"tableCellPaddingY" scale:"space"` // セルの上下の余白
	TableRuleWidth float64 `json:"tableRuleWidth"`                  // 行の間の線の太さ
	// TableHeaderRuleWidth は見出しの行の下の線の太さ(見出しの下に別の線を引く
	// テンプレートだけが使う。report)。
	TableHeaderRuleWidth float64 `json:"tableHeaderRuleWidth"`
	// CalloutMark は結論の丸い矢印の直径と、引用の人の印の大きさ(その印を描く
	// テンプレートだけが使う。report)。
	CalloutMark float64 `json:"calloutMark"`

	ImageMinHeight float64 `json:"imageMinHeight"`           // 画像を縮めてよい下限の高さ
	CaptionGap     float64 `json:"captionGap" scale:"space"` // 表・画像と説明の文の間
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
	// SlideSource はスライドの出典(source)の頭。
	SlideSource string `json:"slideSource"`
	// Date・YearMonth は文書の日付(meta.date)の書き方。{YYYY} {MM} {M} {DD} {D} を
	// 日付の数に置き換える。YearMonth は日の無い日付(YYYY-MM)に使う。
	Date      string `json:"date"`
	YearMonth string `json:"yearMonth"`
	Point     string `json:"point"`
	SubPoint  string `json:"subPoint"`
	Pro       string `json:"pro"`
	Con       string `json:"con"`
	Quote     string `json:"quote"`       // 引用の言葉の頭
	Source    string `json:"quoteSource"` // 引用の出どころの頭
	Unit      string `json:"statUnit"`    // 統計強調の数と単位の間(空でもよい)
	// TableContinued は続けた表(continues)の各枚に付ける印。{n} を何枚目か、{m} を
	// 全部の枚数に置き換える(例 "{n}/{m}")。
	TableContinued string `json:"tableContinued"`
	// Toc は目次のスライド(meta.toc)の題。
	Toc string `json:"toc"`
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

	// 描く処理の細部の寸法(#70 でコードから移した。値は移す前のまま)。
	// ページのリズムに属する余白・間隔と段で揃える箱の大きさは scale:"space"。
	// 図の中の細部の寸法は、線の太さ・角と同じく段の外(印なし。0 以下は
	// checkPositive が断る。docs/template-values.md の「subsystems/gantt.json の形」)。
	LegendSwatchW   float64 `json:"legendSwatchWidth"`             // 凡例の色見本の幅
	LegendSwatchH   float64 `json:"legendSwatchHeight"`            // 凡例の色見本の高さ
	LegendSwatchTop float64 `json:"legendSwatchTop"`               // 色見本の上端を凡例の行の上端から下げる分
	LegendSwatchGap float64 `json:"legendSwatchGap" scale:"space"` // 色見本と文の間
	LegendLabelUp   float64 `json:"legendLabelRaise"`              // 凡例の文の箱を行の上端から上げる分
	LegendLabelGap  float64 `json:"legendLabelGap"`                // 凡例の文の後ろ(この後に padding を空けて次の項目)

	MilestoneMinW    float64 `json:"milestoneMinWidth" scale:"space"` // 節目の文字の箱の最小の幅
	MilestonePadX    float64 `json:"milestonePaddingX" scale:"space"` // 節目の文字の箱の左右の余白
	MilestoneAxisGap float64 `json:"milestoneAxisGap" scale:"space"`  // 節目の段と目盛りの文字の間
	MilestoneGap     float64 `json:"milestoneLabelGap"`               // 同じ段に並べる節目の文字どうしの最小の隙間

	TickLabelW     float64 `json:"tickLabelWidth" scale:"space"`     // 目盛りの文字の箱の幅(日付の真上に中央揃え)
	TickLabelH     float64 `json:"tickLabelHeight"`                  // 目盛りの文字の箱の高さ
	TickSubOffset  float64 `json:"tickSubLabelOffset" scale:"space"` // 目盛りの副ラベルを主の文字の上端から下げる分
	TickSubLabelW  float64 `json:"tickSubLabelWidth" scale:"space"`  // 目盛りの副ラベルの箱の幅
	TickSubLabelH  float64 `json:"tickSubLabelHeight" scale:"space"` // 目盛りの副ラベルの箱の高さ
	AxisGap        float64 `json:"axisGap"`                          // 目盛りの文字の行と軸の線の間、軸の線と本体の間
	TickDotRadius  float64 `json:"tickDotRadius"`                    // 軸の上の目盛りの点
	BreakMarkW     float64 `json:"breakMarkWidth"`                   // 区間圧縮の印(//)の1本の横の幅
	BreakMarkH     float64 `json:"breakMarkHeight"`                  // 同じく縦の高さ
	BreakMarkSpace float64 `json:"breakMarkSpacing"`                 // 2本の線の間(中心どうし)
	BreakMarkHalo  float64 `json:"breakMarkHaloWidth"`               // 印の下に地の色で引く太い線(軸を切って見せる)

	GroupBoxGap     float64 `json:"groupBoxGap" scale:"space"`        // グループ名の箱とサブグループの列の間
	GroupInsetX     float64 `json:"groupInsetX"`                      // グループ名の箱の中の左右の余白
	GroupRadius     float64 `json:"groupRadius"`                      // グループ名の箱の角
	SubInsetL       float64 `json:"subgroupInsetLeft"`                // サブグループの文字の箱の左の余白
	SubInsetR       float64 `json:"subgroupInsetRight" scale:"space"` // 同じく右の余白
	RowLabelGap     float64 `json:"rowLabelGap" scale:"space"`        // 行の名前と時間軸の間
	BarInset        float64 `json:"barInset"`                         // 行の高さと帯・注記の箱の高さの差(上下に半分ずつ)
	BarRadius       float64 `json:"barRadius"`                        // 帯・注記の箱の角
	SegLabelInsetL  float64 `json:"segmentLabelInsetLeft"`            // 帯の中の文の左の余白
	SegLabelInsetR  float64 `json:"segmentLabelInsetRight"`           // 帯の中の文の右の余白
	NoteInsetX      float64 `json:"noteInsetX"`                       // 注記の箱の中の左右の余白
	OpenEndDotX     float64 `json:"openEndDotOffset"`                 // 終わりの無い帯の後ろの点の列の、帯の右端から最初の点まで
	OpenEndDotStep  float64 `json:"openEndDotStep"`                   // 同じく点の間
	OpenEndDotR     float64 `json:"openEndDotRadius"`                 // 同じく点の大きさ
	ExtNoteLabelH   float64 `json:"extNoteLabelHeight"`               // 延伸ノートの文字の箱の高さ
	ExtNoteLabelGap float64 `json:"extNoteLabelGap"`                  // 延伸ノートの文字の箱と線の間

	AnnFixedPadX float64 `json:"annotationFixedPaddingX" scale:"space"` // 手で置いた注釈の箱の中の余白
	AnnFixedPadY float64 `json:"annotationFixedPaddingY" scale:"space"`
	AnnRadius    float64 `json:"annotationRadius"`              // 注釈の箱の角
	AnnGap       float64 `json:"annotationGap" scale:"space"`   // 同じレーンの注釈の箱の間の最小の隙間
	AnnShift     float64 `json:"annotationShift" scale:"space"` // 置けないときにずらす刻み(箱の幅にこれを足す)
	AnchorDotR   float64 `json:"anchorDotRadius"`               // 注釈が指す先の点
	LeaderGap    float64 `json:"leaderGap"`                     // 引き出し線が文字を避けるときの文字との隙間

	ThinLineW      float64 `json:"thinLineWidth"`            // 目盛りの縦の破線・グループの境・注記の破線の枠
	LineW          float64 `json:"lineWidth"`                // 軸・仮の帯の破線の枠・延伸ノートの線・注釈の箱の枠
	LeaderLineW    float64 `json:"leaderLineWidth"`          // 引き出し線・区間圧縮の印の線
	EmphasisLineW  float64 `json:"emphasisLineWidth"`        // 強調した注釈の縦の線
	LegendOutlineW float64 `json:"legendSwatchOutlineWidth"` // 凡例の色見本の枠
	LegendRadius   float64 `json:"legendSwatchRadius"`       // 凡例の色見本の角

	// DateFormat は目盛りと節目の日付の書き方(IR から作った Gantt)。YYYY・YY・
	// M・MM を年と月に置き換え、ほかの文字はそのまま(例 "YYYY/M" → 2026/12)。
	DateFormat string `json:"dateFormat"`

	// 軸と帯の描き方・高さの使い方(#89。既定は従来の描き方)。
	//   - AxisStyle:"ticks"(書いた目盛りだけ)か "monthGrid"(span の月ごとの列と、
	//     年と月の2段の見出し。書いた目盛り・節目は印として重ねる)。
	//   - BarShape:"rect"(角の丸い矩形)か "chevron"(右に尖った山形。面は状態の色を
	//     ChevronFillOpacity で薄くし、文は字の色)。
	//   - FillHeight:true なら Measure の Max を上限なし(0)にし、余った高さを注記の行を
	//     除く行に等しく配る(行の高さは MaxRowH まで)。
	AxisStyle          string  `json:"axisStyle"`
	BarShape           string  `json:"barShape"`
	FillHeight         bool    `json:"fillHeight"`
	GridRowH           float64 `json:"gridRowHeight"`      // 月の格子の年の行・月の行の高さ
	GridYearFormat     string  `json:"gridYearFormat"`     // 年の行の文字(YYYY・YY を置き換える)
	GridMonthFormat    string  `json:"gridMonthFormat"`    // 月の行の文字(MM・M を置き換える)
	ChevronFillOpacity float64 `json:"chevronFillOpacity"` // 山形の帯の面の不透明度(0〜1)
}

// monthGrid は月の格子で描くか。
func (gv *GanttValues) monthGrid() bool { return gv.AxisStyle == "monthGrid" }

// chevron は帯を山形で描くか。
func (gv *GanttValues) chevron() bool { return gv.BarShape == "chevron" }

// checkGanttStyle は軸と帯の描き方の値を見る(知らない名前・範囲の外・書き方の誤り)。
func checkGanttStyle(at string, gv GanttValues) error {
	if gv.AxisStyle != "ticks" && gv.AxisStyle != "monthGrid" {
		return fmt.Errorf("%s.axisStyle: %q is not ticks or monthGrid", at, gv.AxisStyle)
	}
	if gv.BarShape != "rect" && gv.BarShape != "chevron" {
		return fmt.Errorf("%s.barShape: %q is not rect or chevron", at, gv.BarShape)
	}
	if gv.ChevronFillOpacity > 1 {
		return fmt.Errorf("%s.chevronFillOpacity: must be at most 1 (got %v)", at, gv.ChevronFillOpacity)
	}
	if !strings.Contains(gv.GridYearFormat, "YY") || strings.Contains(gv.GridYearFormat, "M") ||
		strings.Contains(strings.ReplaceAll(strings.ReplaceAll(gv.GridYearFormat, "YYYY", ""), "YY", ""), "Y") {
		return fmt.Errorf("%s.gridYearFormat: write the year as YYYY or YY and no month, got %q", at, gv.GridYearFormat)
	}
	if !strings.Contains(gv.GridMonthFormat, "M") || strings.Contains(gv.GridMonthFormat, "Y") {
		return fmt.Errorf("%s.gridMonthFormat: write the month as M or MM and no year, got %q", at, gv.GridMonthFormat)
	}
	return nil
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
	// 図のサブシステム(figures.go)
	Process  *ProcessValues  `json:"process,omitempty"`
	OrgChart *OrgChartValues `json:"orgchart,omitempty"`
	Matrix   *MatrixValues   `json:"matrix,omitempty"`
	Funnel   *FunnelValues   `json:"funnel,omitempty"`
	Phases   *PhasesValues   `json:"phases,omitempty"`
	// 数値のグラフ(chart.go)
	Chart *ChartValues `json:"chart,omitempty"`
	// 輪に並んだ段(cycle.go)
	Cycle *CycleValues `json:"cycle,omitempty"`
}

// themeRoles はテーマが持たなければならない役割名(コア・デモのテンプレートの
// 部品・Gantt が使うもの)。これ以外の役割名は誤り。
var themeRoles = []string{
	"bg", "card", "card_alt", "text_primary", "text_muted", "grid",
	"support", "conditional", "none", "danger", "risk", "accent", "on_conditional",
}

// distinctRoles は互いに違う色でなければならない役割名。どれも意味を色で
// 見分けさせるもの(強調・Gantt や図の状態・補足の文字)で、同じ色だと
// 「条件つき」が「強調」に見えるように意味が読めなくなる。PPTX のテーマの
// accent1〜6 にもこの順に写す(pptx の theme)。
var distinctRoles = []string{"accent", "conditional", "support", "danger", "none", "text_muted"}

// checkDistinctRoles は distinctRoles の色が重ならないことを確かめる
// (大文字小文字は hexColorRe で揃っている)。
func checkDistinctRoles(theme map[string]string) error {
	seen := map[string]string{}
	for _, r := range distinctRoles {
		c := theme[r]
		if prev, ok := seen[c]; ok {
			return fmt.Errorf("theme.%s: %s is the same color as theme.%s; the roles %v carry different meanings and must all differ", r, c, prev, distinctRoles)
		}
		seen[c] = r
	}
	return nil
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
	if err := checkDistinctRoles(f.Theme); err != nil {
		return nil, err
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
	if _, ok := orderedStyles[f.Values.OrderedStyle]; !ok {
		return nil, fmt.Errorf("values.orderedStyle: %q is not a numbering style (known: %v)", f.Values.OrderedStyle, sortedKeys(orderedStyles))
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
		components: standardComponents.clone(),
		skeleton:   standardSkeleton.clone(),
		Variants:   variants,
		// 囲みは型の語彙が名乗る(テンプレートは型を隠さない)。
		Blocks: blockTypes.fences(),
		Slides: defaultSlides(),
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
		if err := checkDateFormat("subsystems.gantt", gv.DateFormat); err != nil {
			return nil, err
		}
		if err := checkGanttStyle("subsystems.gantt", gv); err != nil {
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
	// 図のサブシステム(process・orgchart・matrix・funnel)も字の段をこのテンプレートの
	// values から引き、寸法をこの段で確かめる。
	if err := templateFigures(t, &f); err != nil {
		return nil, err
	}
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
		if rt.Field(i).Name == "Unit" {
			continue // 数と単位を詰める(空)のは正しい選び方
		}
		if rv.Field(i).String() == "" {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			return fmt.Errorf("marks.%s: must not be empty", name)
		}
	}
	if err := checkDateMark("date", m.Date, true); err != nil {
		return err
	}
	if err := checkDateMark("yearMonth", m.YearMonth, false); err != nil {
		return err
	}
	for _, ph := range []string{"{n}", "{m}"} {
		if !strings.Contains(m.TableContinued, ph) {
			return fmt.Errorf("marks.tableContinued: %q needs %s (the page number {n} and the number of pages {m} of a continued table)", m.TableContinued, ph)
		}
	}
	return nil
}

// replaceNM は印の {n} と {m} を数にする。
func replaceNM(f string, n, m int) string {
	return strings.NewReplacer("{n}", strconv.Itoa(n), "{m}", strconv.Itoa(m)).Replace(f)
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
	// 自分の組み方(骨組みの部品・部品の差し替え)を持つテンプレート(report.go)。
	// 標準でない型の組み方を自分で登録したものは、下の同梱の組み方より先に入る。
	if reg, ok := bundledTemplateParts[name]; ok {
		if err := reg(t); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	}
	// 同梱のテンプレートは、コアの標準でない型の組み方も持つ(pairs.go)。
	registerBundledComponents(t.components)
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
	if err := checkPositive("gantt", gv); err != nil {
		return gv, err
	}
	if err := checkDateFormat("gantt", gv.DateFormat); err != nil {
		return gv, err
	}
	return gv, checkGanttStyle("gantt", gv)
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

// dateTokens は日付の書き方(marks.date・marks.yearMonth)に書ける置き換えの印。
var dateTokens = []string{"{YYYY}", "{MM}", "{M}", "{DD}", "{D}"}

// checkDateMark は日付の書き方を見る。年と月の印は必ず要り、日の印は withDay の
// ときだけ要る(無いときは書けない)。知らない {…} は誤り。
func checkDateMark(name, f string, withDay bool) error {
	rest := f
	for _, t := range dateTokens {
		rest = strings.ReplaceAll(rest, t, "")
	}
	if strings.ContainsAny(rest, "{}") {
		return fmt.Errorf("marks.%s: %q has an unknown {…} (known: %s)", name, f, strings.Join(dateTokens, " "))
	}
	has := func(ts ...string) bool {
		for _, t := range ts {
			if strings.Contains(f, t) {
				return true
			}
		}
		return false
	}
	if !has("{YYYY}") || !has("{MM}", "{M}") {
		return fmt.Errorf("marks.%s: %q needs {YYYY} and {M} (or {MM})", name, f)
	}
	if withDay && !has("{DD}", "{D}") {
		return fmt.Errorf("marks.%s: %q needs {D} (or {DD})", name, f)
	}
	if !withDay && has("{DD}", "{D}") {
		return fmt.Errorf("marks.%s: %q is for dates without a day and cannot use {D} or {DD}", name, f)
	}
	return nil
}

// formatDate は meta.date(YYYY-MM-DD か YYYY-MM)をテンプレートの書き方にする。
// 形が違えば書いたとおりを返す(形は読み込みと Schema が見る)。
func (m TemplateMarks) formatDate(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) < 2 || len(parts) > 3 {
		return s
	}
	n := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return s
		}
		n[i] = v
	}
	f := m.YearMonth
	if len(parts) == 3 {
		f = m.Date
	}
	r := strings.NewReplacer("{YYYY}", fmt.Sprintf("%04d", n[0]), "{MM}", fmt.Sprintf("%02d", n[1]), "{M}", strconv.Itoa(n[1]))
	f = r.Replace(f)
	if len(parts) == 3 {
		f = strings.NewReplacer("{DD}", fmt.Sprintf("%02d", n[2]), "{D}", strconv.Itoa(n[2])).Replace(f)
	}
	return f
}
