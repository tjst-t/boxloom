// 作者(人と AI)に見せる「書けるもの」の表。
//
//   - 意味ブロック(囲み)は、テンプレートが名乗る(Template.Blocks)。いまの
//     テンプレート(dark・light)はどちらも既定の一式(defaultBlocks)を名乗る。
//     Markdown のパーサーはまだテンプレートを知らずに読むので、既定の一式で検査する。
//   - サブシステムは、登録簿の SubsystemDecl で Schema と Markdown の約束事を名乗る。
//   - 文書の頭・文中の記法はコアの記法(docs/content-syntax.md の3章)。
//
// `boxloom schema`(schema_cmd.go)はここから1つの JSON を組む。

package boxloom

import (
	"embed"
	"encoding/json"
	"sort"
)

// schemas/ はリポジトリ直下の schema/ の写し(go:embed は上の階層を読めないため)。
// schema/ を直したら `go generate` で写し直す。写しがずれると
// TestEmbeddedSchemasInSync が落ちる。
//
//go:generate sh -c "rm -rf schemas && cp -r ../schema schemas"
//go:embed schemas
var schemaFS embed.FS

func embeddedSchema(name string) json.RawMessage {
	b, err := schemaFS.ReadFile("schemas/" + name)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(b)
}

// BlockDecl は1つの囲み(:::名前)が名乗るもの。
type BlockDecl struct {
	Name string `json:"name"`
	// Label は名前の後ろに書く文の意味(空なら書けない)。
	Label string `json:"label,omitempty"`
	// LabelRequired は Label が必須か。
	LabelRequired bool `json:"labelRequired,omitempty"`
	// Inside は置ける場所(親の囲みの名前)。空なら本文に置く。
	Inside []string `json:"inside,omitempty"`
	// Contains は中に書けるもの(囲みの名前、または "本文"・"箇条")。
	Contains []string `json:"contains"`
	// MinChildren は Contains の囲みが最低いくつ要るか(0 は決まりなし)。
	MinChildren int `json:"minChildren,omitempty"`
	// Attributes は {名前=値} で書ける属性。
	Attributes []string `json:"attributes"`
	Meaning    string   `json:"meaning"`
	// IR はできる IR の場所。
	IR string `json:"ir"`
}

// defaultBlocks は第一版の囲み(docs/content-syntax.md の3.4)。名前の順。
func defaultBlocks() []BlockDecl {
	return []BlockDecl{
		{Name: "column", Label: "まとまりの題(任意)", Inside: []string{"columns"},
			Contains: []string{"段落", "箇条", "表", "画像(1枚)", "note", "```chart"}, Attributes: []string{"role"},
			Meaning: "並列のまとまりの1つ。role=main は主のまとまり(ほかがそれを補う。テンプレートが広く取る)。幅を数で書く口は無い",
			IR:      "columns.columns[](title・role・body)"},
		{Name: "columns", Contains: []string{"column"}, MinChildren: 2, Attributes: []string{"variant", "flow"},
			Meaning: "2〜3個のまとまりを並べて読ませる(段組みの指定ではなく「並列」という意味)。画像やグラフと文(読み取り)を横に並べるのもこれ。{flow=true} は左のまとまりが右につながる(前提→結論、発見→対応)という意味で、印はテンプレートが描く",
			IR:      "columns"},
		{Name: "compare", Contains: []string{"option"}, MinChildren: 2, Attributes: []string{"variant"},
			Meaning: "選択肢の比較", IR: "compare"},
		{Name: "cons", Inside: []string{"option"}, Contains: []string{"箇条"}, Attributes: []string{},
			Meaning: "悪い点", IR: "compare.options[].cons"},
		{Name: "note", Contains: []string{"本文"}, Attributes: []string{"variant"},
			Meaning: "注記・但し書き", IR: "note"},
		{Name: "option", Label: "選択肢の名前", LabelRequired: true, Inside: []string{"compare"},
			Contains: []string{"pros", "cons"}, MinChildren: 1, Attributes: []string{},
			Meaning: "比較の1つの選択肢", IR: "compare.options[](label が名前)"},
		{Name: "pros", Inside: []string{"option"}, Contains: []string{"箇条"}, Attributes: []string{},
			Meaning: "良い点", IR: "compare.options[].pros"},
		{Name: "quote", Label: "出どころ(誰の・いつの言葉か)", LabelRequired: true, Contains: []string{"段落1つ"}, Attributes: []string{"variant"},
			Meaning: "他人の言葉と出どころ(docs/patterns.md の R11)。中の段落が引用した言葉。自分の結論には使わない(それは takeaway)",
			IR:      "quote(content が言葉、source が見出しの文)"},
		{Name: "stat", Contains: []string{"value"}, MinChildren: 1, Attributes: []string{"variant"},
			Meaning: "1〜4個の数を、単位と短い説明つきで強く見せる(R3)。5つ以上・推移は系列なので table・chart",
			IR:      "stat"},
		{Name: "takeaway", Contains: []string{"段落1つ"}, Attributes: []string{"variant"},
			Meaning: "本文の中で言いたいことを1文で目立たせる(R2)。スライド全体が1文なら # 題 {type=statement}",
			IR:      "takeaway"},
		{Name: "value", Label: "数(書いたとおりの文字。例 12・−40・3 → 12。数字を含む)", LabelRequired: true, Inside: []string{"stat"},
			Contains: []string{"段落1つ"}, Attributes: []string{"unit"},
			Meaning: "stat の1つの数。属性 unit が単位(空白を含まない)、中の段落が数の説明。stat の中に1〜4個",
			IR:      "stat.items[](value が見出しの文、unit が属性、label が中の段落)"},
	}
}

// SlideDecl は1つのスライドの種類が名乗るもの(「# 題 {type=名前}」)。
type SlideDecl struct {
	Name string `json:"name"`
	// Heading は # の行の文の意味。
	Heading string `json:"heading"`
	// Body は # の行の後ろに書けるもの。
	Body    []string `json:"body"`
	Meaning string   `json:"meaning"`
	IR      string   `json:"ir"`
}

// defaultSlides はテンプレートの骨組みがあるスライドの種類(docs/patterns.md の3.1)。
// 表紙は文書の頭から作るので、ここには無い(type=cover は誤り)。
func defaultSlides() []SlideDecl {
	return []SlideDecl{
		{Name: "content", Heading: "題(短い見出し。1行に収める)", Body: []string{"本文", "lead", "notes", "source"},
			Meaning: "本文のスライド。type を省くとこれ。言いたいことの文は :::lead に。{marker=A} で隅の印(空白を含まない2字まで。いまどのパターン・要因の話か。描くかはテンプレート)", IR: "{type: content, title, lead?, marker?, body}"},
		{Name: "section", Heading: "章・節・項目の題", Body: []string{"notes"},
			Meaning: "章の変わり目(R1)。{level=1〜3}(1 章・2 節・3 節の中の項目。既定 1)。親の無い level は誤り。本文は書けない(目次は文書の頭の toc: true で作る)", IR: "{type: section, title, level?}"},
		{Name: "statement", Heading: "言いたいことの1文", Body: []string{"notes", "source"},
			Meaning: "このスライドで言うことが1文だけ(R2)。根拠はほかのスライドに置く。本文は書けない", IR: "{type: statement, content}"},
	}
}

// slideTypeNames は「# 題 {type=…}」に書ける名前(名前の順)。
func slideTypeNames() []string {
	var out []string
	for _, d := range defaultSlides() {
		out = append(out, d.Name)
	}
	sort.Strings(out)
	return out
}

// coreBlocks はテンプレートに依らずコアが持つ囲み(本文のブロックではなく、スライドの
// 欄になるもの。置き場所はスライドの一番外側)。
func coreBlocks() []BlockDecl {
	return []BlockDecl{
		{Name: "notes", Contains: []string{"段落", "箇条"}, Attributes: []string{},
			Meaning: "発表者ノート。スライドの本文の一番外側に1枚に1つ。最初の # より前なら表紙のノート。見た目には何も足さない",
			IR:      "スライドの notes(文字だけの文字列)"},
		{Name: "lead", Contains: []string{"段落1つ"}, Attributes: []string{},
			Meaning: "リード文。このスライドで言いたいことを述べる文(1〜2行)。本文のスライドの一番外側に1枚に1つ。テンプレートが題の直下に置く。題は短い見出しにし、結論はここに書く",
			IR:      "スライドの lead(文中の値)"},
		{Name: "source", Contains: []string{"段落1つ"}, Attributes: []string{},
			Meaning: "このスライドの出典(出どころ・母数・時点)。スライドの本文の一番外側に1枚に1つ。本文と主張のスライドに書ける。テンプレートが本文の下・フッターの上に置く。但し書きは note、引用の出どころは :::quote の見出しの文",
			IR:      "スライドの source(文中の値)"},
	}
}

// knownFences は Markdown のパーサーが受け付ける囲みの名前(コアと型の語彙の囲み)。
func knownFences() []string {
	return blockNames(append(coreBlocks(), blockTypes.fences()...))
}

func blockNames(bs []BlockDecl) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Name)
	}
	sort.Strings(out)
	return out
}

// ganttDecl は Gantt サブシステムが名乗るもの。
func ganttDecl() SubsystemDecl {
	return SubsystemDecl{
		Schema: embeddedSchema("subsystems/gantt.schema.json"),
		Markdown: []string{
			"中身は YAML で、キーは IR の data と同じ。知らないキーは誤り",
			"日付は YYYY-MM か YYYY-MM-DD の文字列。範囲は `A .. B` か `{from: A, to: B}`",
			"状態は意味役割名(support・conditional・none・danger)だけ。色は書けない",
			"注釈は位置や上下を持たない。付ける先を target(グループか行の id)で示す",
			"文の値(label・text・凡例の説明)は本文と同じ文中の記法で読む。YAML の列で書くと各行を意味のある改行でつなぐ",
			"グループの label は列で書くと行の配列になる",
			"YAML は部分集合:参照(& *)・タグ(!)・複数文書・複数行の値・ブロックの文字列(| >)は誤り。真偽値は true / false だけ",
			"値の中に「: 」を書くときは \"…\" で囲む",
		},
	}
}

// shapesDecl は shapes サブシステムが名乗るもの。
func shapesDecl() SubsystemDecl {
	return SubsystemDecl{
		Schema: embeddedSchema("subsystems/shapes.schema.json"),
		Markdown: []string{
			"既存のパターン(箇条・比較・Gantt)で表せない図のときだけ使う最後の手段",
			"中身は YAML で、キーは IR の data と同じ。知らないキーは誤り",
			"座標と大きさは枠の内側に対する 0〜1 の数(引用しない)。x+w・y+h は 1 以下",
			"線の点は流れの書き方の列で書く: from: [0.1, 0.5]",
			"色はテーマの役割名だけ(fill・line・textColor・color)。16進の色は書けない",
			"文字の大きさは textStyle に段の名前(small・subPoint・optionItem・body・optionLabel)。数は書けない",
			"connector の from・to は図形の id。重なった図形は boxloom check が shapes-overlap の警告にする。コネクタが辺に浅くしか繋げない位置関係は shapes-connector-angle の警告",
			"文の値(text)は本文と同じ文中の記法で読む。YAML の列で書くと各行を意味のある改行でつなぐ",
		},
	}
}

// frontMatterKey は文書の頭のキー(docs/content-syntax.md の3.1)。
type frontMatterKey struct {
	Key      string `json:"key"`
	Meaning  string `json:"meaning"`
	Required bool   `json:"required,omitempty"`
	IR       string `json:"ir"`
}

func frontMatterKeys() []frontMatterKey {
	return []frontMatterKey{
		{"title", "文書の題(表紙に使う)。文中の記法で読む", true, "meta.title"},
		{"subtitle", "副題", false, "meta.subtitle"},
		{"author", "作者", false, "meta.author"},
		{"date", "日付(YYYY-MM-DD か YYYY-MM の文字列)。表紙に出す(書き方はテンプレート)", false, "meta.date"},
		{"confidential", "機密区分の短い文(例 社外秘)。表紙とフッターに出す。文中の記法で読む", false, "meta.confidential"},
		{"disclaimer", "文書全体の免責・但し書き(1回だけ書く)。表紙に出す。文中の記法で読む", false, "meta.disclaimer"},
		{"template", "テンプレートの名前", false, "template"},
		{"cover", "表紙を出すか(true / false、既定 true)", false, "先頭の {type: cover} のスライド"},
		{"toc", "表紙の直後に目次を置くか(true / false、既定 false)。level 1・2 の区切りを頁番号つきで並べる(目次のスライドは組み立てで作る)", false, "meta.toc"},
	}
}

// frontMatterNames は文書の頭に書けるキーの名前(名前の順)。
func frontMatterNames() []string {
	var out []string
	for _, k := range frontMatterKeys() {
		out = append(out, k.Key)
	}
	sort.Strings(out)
	return out
}

// bodySyntax は本文で使える書き方(3.3)。
type bodySyntaxItem struct {
	Syntax  string `json:"syntax"`
	Meaning string `json:"meaning"`
	IR      string `json:"ir"`
}

func bodySyntax() []bodySyntaxItem {
	return []bodySyntaxItem{
		{"段落", "文", "paragraph"},
		{"- 項目 / 1. 項目", "箇条(入れ子は2段まで)。1. は番号つきの箇条(番号の形はテンプレート。1 から始める。同じ段に - と混ぜない)", "points(入れ子は {content, items}。1. なら ordered: true)"},
		{"**…**", "強調(太字という見た目ではなく、強調という意味)", "インラインの em"},
		{"`…`", "コード・識別子", "インラインの code"},
		{"[文](URL)", "リンク", "インラインの link"},
		{"行末の \\", "意味のある改行", "インラインの br"},
		{"| 見出し | … |\n| --- | … |\n| セル | … |", "表(GFM の表)。1行目が見出し。列の配置の指定(:--: など)は誤り。表のすぐ次の行の「: 説明」は表の説明。その行の末尾に {total=last} を書くと最後の行が合計の行、{groups=first} を書くと先頭の列が行の群(群の最初の行にだけ名前を書き、続きの行の先頭のセルは空)、{points=列名} を書くとその列のセルを半角の ; で区切った箇条として読む(文の ; は \\;。複数の列は {points=列名1,列名2})。長い表は作者がスライドごとに分け、続きのスライドの表は見出しの行を書かずに区切りの行から始め、説明の行に {continues} を書く(すぐ前のスライドの最後の表に続く。見出しの行・列の数・groups・points は最初の表に従い、各枚に見出しの行と「n/m」の印が付き、列の幅がそろう)", "table"},
		{"![代わりの文](path \"説明\")", "画像。画像だけの段落に1枚なら image、2〜6枚を並べると gallery。代わりの文は必須。path は文書からの相対パス(PNG・JPEG)", "image / gallery"},
	}
}

// markdownRules は記法全体の約束事(2章・3章の要点)。
func markdownRules() []string {
	return []string{
		"`# 題` が1枚のスライドの始まり。行末に {type=種類 variant=名前} を書ける。種類は slides(省けば content)。区切りは {type=section level=2} のように階層(1〜3、既定 1)を書ける。本文のスライドは {marker=A} で隅の印(空白を含まない2字まで)を書ける。section・statement は # の行の文だけを持ち、本文は書けない(:::notes と、statement は :::source も書ける)。1枚目の # より前に本文を書くのは誤り",
		"`---`(水平線)はスライドの区切りではなく誤り",
		"小見出し(##)は使えない(IR に小見出しは無い。まとまりの題は囲みの見出しの文で書く)。### 以下の見出しも使えない",
		"表のセルの中はインライン(強調・コード・リンク)だけ。どの行も見出しと同じ数のセル",
		"画像は文の中には書けない。URL・絶対パス・文書のフォルダの外(..)は誤り",
		"囲みは :::名前 で開き、同じ数のコロンの ::: で閉じる。閉じの省略は誤り。中に囲みを入れるときは外側のコロンを1つ増やす。入れ子は3段まで",
		"囲みの書き方は `:::名前 見出しの文 {属性=値}`。名乗っていない囲み・属性は誤り",
		"サブシステムは言語名つきのコードブロック(```名前)で、中身は YAML。言語名の後ろに {variant=名前} を書ける",
		"斜体(*…*)・取り消し線・脚注・生の HTML・サブシステムでないコードブロックは誤り",
		"見た目(色・大きさ・位置)は書けない。見た目はテンプレートが決める",
		"段落の中の改行は次の行とつながる(つなぎ目の両側が英数字のときだけ空白を入れる)",
	}
}
