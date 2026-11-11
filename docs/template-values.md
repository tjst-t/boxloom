# テンプレートの値の表(データのファイル)

テンプレートの色・字の大きさ・余白・印の文字、Gantt の寸法を、Go のコードから
JSON のファイルに移した。方針(CLAUDE.md)の「値の表・スタイル規則・骨組みは
データ、部品とサブシステムはコード」のうち、**値の表**(と印の文字という小さな
スタイル規則)がデータになった。骨組み(表紙・本文のスライドの並び)と部品
(箇条・比較など)はまだコード(`template.go`)。

| ファイル | 中身 |
|---|---|
| `boxloom/templates/<名前>.json` | テンプレート1つ(`dark.json`・`light.json`)。名前はファイル名 |
| `boxloom/subsystems/gantt.json` | Gantt の既定の寸法。テンプレートが差し替えなければこれ |
| `boxloom/subsystems/shapes.json` | shapes(生の図形への逃げ道)の既定の寸法 |
| `boxloom/subsystems/process.json`・`orgchart.json`・`matrix.json`・`funnel.json` | 箱と線の図(`docs/subsystem.md` の11節)の既定の寸法 |
| `boxloom/subsystems/chart.json` | 数値のグラフ(`docs/subsystem.md` の12節)の既定の寸法 |
| `boxloom/values.go` | 読み込みと検査 |

どちらも `go:embed` でバイナリに同梱する。テンプレートの一覧はこのフォルダの
ファイルから作るので、`templates/` にファイルを1つ足せば `-template <名前>` で選べる。

## 外の値の表を使う(Backlog #31)

自分たちの色と字体で使いたいときは、同梱のものと同じ形の JSON を作り、
`-template` にファイルのパスを渡す(`build`・`check`・`schema`・`fonts` のどれでも)。

```
go run ./cmd/boxloom build -template ./my-brand.json deck.bx.md
go run ./cmd/boxloom schema -template ./my-brand.json   # そのテンプレートが名乗るものだけ
```

- `-template` の値が `.json` で終わるか、パスの区切り(`/`・`\`)を含めばファイル、
  そうでなければ同梱の名前として引く。
- 検査は同梱のものと**同じ**(次の節)。`name` はファイル名(拡張子を除いた最後の
  部分)と同じでなければならない。IR の `template` より `-template` が優先する。
- 誤りは5章の形の1件(`code` は `template-values`、`file` は値の表のファイル、
  `path` は値の表の中の場所、`line` はその行の目安)。`check` は値の表に誤りが
  あれば文書を検査せず、その1件だけを返す(終了コード 1)。

  ```
  error: ./my-brand.json:36 /values/blockGap: template-values: values.blockGap: 13 is not on the spacing scale (multiples of 4); nearest: 12 or 16
  ```
- 囲み・部品・骨組みはまだコードなので、外の値の表で変えられるのは値の表の中身
  (色・余白・字の大きさ・字体・印・サブシステムの寸法)だけ。囲みの一覧は同梱の
  テンプレートと同じ。
- ライブラリでは `ParseTemplate(file, data)` で読み、`BuildOptions.Values`・
  `CheckOptions.Values`・`SchemaTemplate` に渡す(`docs/library.md`)。
- 例は `examples/templates/forest.json`(架空の緑の配色)。

## 読み込みの検査

黙って既定値で埋めない。次はすべて誤りで、テンプレートを引いたとき
(`build`・`check`)に `templates/light.json: missing key "values.body"` の形で出る。

- 知らないキー(どの階層でも)。
- 欠けたキー。`subsystems` だけは書かなくてよい(書くなら中の値は全部)。
- `null`、型の違い(数のところに文字など)、最上位の値のあとに続くもの。
- `name` がファイル名と違う。
- テーマの役割名が決まった 13 個(下)と1つでも違う(欠け・余り)。色は
  大文字の16進6桁(`0033CC`)。`#` や小文字は断る。
- 大きさ(数)が 0 以下。
- 段階(`scale`)の誤り、または段階から外れた値(次の節)。
- 意味の違う役割名が同じ色(`accent` `conditional` `support` `danger` `none`
  `text_muted` の6つは互いに違う色。次の節)。
- 印の文字が空(数と単位の間の `statUnit` だけは空でよい)。
- 日付の書き方(`marks.date`・`marks.yearMonth`)の印の誤り(次の節の marks)。
- `variants` に空の名前・同じ名前が2回。

同梱のファイルが壊れていれば試験(`values_test.go`)が落ちる。

## templates/<名前>.json の形

```json
{
  "name": "light",
  "theme":  { "bg": "FFFFFF", "card": "F7F9FD", "...": "..." },
  "scale":  { "spacingUnit": 4, "typeSizes": [8, 10, 12, 14, 16, 18, 24, 36] },
  "values": { "pageWidth": 960, "pageHeight": 540, "...": 0 },
  "marks":  { "slideSource": "出典:", "date": "{YYYY}年{M}月{D}日", "yearMonth": "{YYYY}年{M}月",
              "point": "・", "subPoint": "– ", "pro": "＋ ", "con": "－ ", "...": "..." },
  "fonts":  { "latin": "Noto Sans CJK JP", "eastAsian": "Noto Sans CJK JP",
              "fallbackFonts": ["BIZ UDPGothic", "…"], "widthMargin": 1.05, "fixedLineSpacing": true },
  "variants": [],
  "subsystems": { "gantt": { "...": "subsystems/gantt.json と同じ形" } }
}
```

### scale(段階。見た目のトークンの刻み)

余白と字の大きさを、自由な数ではなく決まった段階からしか選べないようにする
(スイス・タイポグラフィのグリッドの考え方。テンプレートを足す人の下振れを防ぐ)。

| キー | 意味 |
|---|---|
| `spacingUnit` | 余白・間隔の刻み(pt)。下の「余白」の値はこの倍数だけ |
| `typeSizes` | 字の大きさの段(pt)。小さい順に重ならずに並べる。下の「字」の値はこの中の値だけ |

段階に載せる値(Go の json タグに `scale:"space"` / `scale:"type"` が付いたもの):

| 種類 | values | subsystems/gantt.json | subsystems/shapes.json |
|---|---|---|---|
| 余白 | `pagePadding` `pagePaddingY` `coverPadding` `sectionGap` `blockGap` `itemGap` `headerGap` `cardPadding` `statementInset` `columnGap` `tableCellPaddingX` `tableCellPaddingY` `captionGap` | `padding` `legendGap` `groupGap` `annotationPaddingX` `annotationPaddingY` `annotationLaneGap` | `padding` `textInsetX` `textInsetY` |
| 字 | `coverTitle` `coverSubtitle` `coverAuthor` `slideTitle` `leadText` `body` `subPoint` `optionLabel` `optionItem` `small` `figureDetail` `captionText` `sectionTitle` `sectionTrackerText` `statementText` `takeawayText` `statValue` `statUnit` `statLabel` `quoteText` `quoteMark` `quoteSource` `columnTitle` `tableText` | `annotationText` `labelText` `smallText` | (字は values の段の名前で引く) |

ページの大きさ・角の丸み・線の太さ・行の高さ・欄の幅・倍率は段階に載せない
(図の寸法で、刻みに合わせても読みやすさは変わらないため)。

- 検査:段階そのもの(刻みが 0 以下、段が空・小さい順でない)と、段から外れた値は誤り。
  誤りの文は近い段の値を添える(例 `values.subPoint: 12.5 is not on the type scale
  [8 10 12 14 16 24 36]; nearest: 12 or 14`)。
- サブシステムの寸法は、そのテンプレートの段階で見る。テンプレートが差し替えない
  ときは既定のファイル(`subsystems/gantt.json` など)の値を見る。段を狭めた
  テンプレートは、既定の Gantt の字が段から外れれば読み込みで断られる。
- dark / light は `spacingUnit` 4、`typeSizes` [8, 10, 12, 14, 16, 18, 24, 36] の8段。
  段の役割はおおむね 36 表紙の題・区切りの題・主張の1文・統計強調の数・引用の印、
  24 スライドの題、18 リード文、16 本文と小見出し(表紙の副題・比較の選択肢の名前・結論・
  引用の言葉・数の単位・図の箱の題)、14 入れ子の箇条・表の文字、12 従の文(利点と欠点・
  表紙の作者・数の説明・引用の出どころ)、10 注(注記・フッター・Gantt の目盛りと行の名前)、8 図の中の細かい字。
  文字のパターン(#17〜#19)のために段は足していない(36 を数と主張に使う)。

**段階に寄せたとき(Backlog #29)に変えた値**(dark / light 共通):

| 値 | 前 | 後 |
|---|---|---|
| `itemGap` | 6 | 8 |
| `cardPadding` | 14 | 16 |
| `coverAuthor` | 11.5 | 12 |
| `subPoint` | 12.5 | 12 |
| `optionLabel` | 15 | 16 |
| `optionItem` | 13 | 12 |
| `small` | 10.5 | 10 |

**余白を見直したときに変えた値**(dark / light / forest 共通)。要素の間が詰まっている
わりにページの下が空いて見え、ページの外周も狭かったため。余白は「近いものは近く、
まとまりの間は広く」の順(項目 12 < ブロック 20 ≦ 題と本文 20 < 左右の外周 40)に揃えた。
あわせて題は短い見出し(1行)にし、言いたいことの文はリード文(`lead`)に分けた:

| 値 | 前 | 後 |
|---|---|---|
| `pagePadding`(左右) | 24 | 40 |
| `pagePaddingY`(上下。新しく足した) | (24) | 24 |
| `sectionGap` | 16 | 20 |
| `blockGap` | 16 | 20 |
| `itemGap` | 8 | 12 |
| `headerGap`(題とリード文の間。新しく足した) | — | 8 |
| `leadText`(リード文。新しく足した) | — | 16 |
| `cardPadding` | 16 | 20 |
| `coverPadding` | 48 | 64 |

本文の領域は、リード文も出典も無いスライドで 414pt → 406pt(左右が広がり上下はそのまま、
gap が増えた分)。1行のリード文で約 30pt、2行で約 52pt 狭くなる。

**字の大きさを見直したときに変えた値**(dark / light / forest 共通)。箇条の字が
リード文より小さく、項目の間が字に比べて広く見えたため。本文を一段上げ、項目の間を
詰め(行の高さ 21.6pt に対して 8pt)、リード文は本文より一段上に置いた。段に 18 を足した:

| 値 | 前 | 後 |
|---|---|---|
| `typeSizes` | 8〜16・24・36 | 18 を足す |
| `leadText` | 16 | 18 |
| `body`(本文・図の箱の題) | 14 | 16 |
| `subPoint`(入れ子の箇条・図の箱の補足) | 12 | 14 |
| `tableText` | 12 | 14 |
| `itemGap` | 12 | 8 |

1行のリード文で本文の領域は約 32pt、2行で約 57pt 狭くなる。本文の1行は 18.9pt → 21.6pt。

リード文・主張・結論・引用(文として読む段落)は、行の数を増やさない範囲で、文の
切れ目(。!?)、次に読点(、,)の後ろで折る(`TextRun.Phrase`、`boxloom/text.go`)。
| gantt `annotationPaddingY` | 6 | 4 |
| gantt `annotationText` | 8.5 | 8 |
| gantt `labelText` | 9 | 10 |
| shapes `textInsetX` | 6 | 8 |

前後の画像は `verification-screenshots/scales_compare_*.png`(LibreOffice、左が前・右が後)。
配置の骨組みは同じで、箇条の間が少し開き、Gantt の目盛りと行の名前が少し大きくなった。
はみ出しの例(`examples/ir/check-warnings/layout-overflow.json`)は、はみ出しが 47.8pt から
75.1pt に増えた(箇条の間が開いたため。もともとはみ出させる例)。ほかの例の警告の
種類と数は変わらない。従来のデモ(`go run ./cmd/boxloom`)も Gantt の既定の寸法を使うので、Gantt の
字と注釈の箱が変わる。

**図の補足の字を上げたとき(Backlog #45)に変えた値**(dark / light 共通):

| 値 | 前 | 後 |
|---|---|---|
| 図の補足(新しい `figureDetail`。以前は `small`) | 10 | 12 |
| 表・画像の説明(新しい `captionText`。以前は `small`) | 10 | 12 |
| gantt `annotationText` | 8 | 10 |
| marks `statUnit` | 半角の空白 | 空 |
| light の theme `conditional` | `0033CC`(`accent` と同じ) | `9A6700` |

結論や数が投影で読めない細字に書かれないように、図の補足と説明を従の文(12)に、
Gantt の注釈を注(10)に上げた。Gantt の区間の文・節目・グループ名(`smallText` 8)は
上げていない:10 にすると `examples/ir/gantt-full.json` が本文に収まらなくなる
(行が高くなり 46.9pt はみ出す)。密な Gantt の中の字は 8 のまま、値の表で差し替えられる。
例の警告の数は変わらない(20 の組み合わせで `check` の件数が同じ)。

### theme(役割名 → 色)

`bg` `card` `card_alt` `text_primary` `text_muted` `grid` `support`
`conditional` `none` `danger` `risk` `accent` `on_conditional` の 13 個、ちょうど。

**意味の違う役割名は違う色(Backlog #45)。** `accent`(見てほしい1か所)・
`conditional`・`support`・`danger`・`none`(状態)・`text_muted`(補足の文字)の6つは、
どの2つも同じ16進にできない。読み込みで `theme.conditional: 0033CC is the same color as
theme.accent` の形の誤り。同じ色だと、図の「条件つき」の丸や「要調整」の枠が強調に
見え、意味が読めなくなる(以前の light は `conditional` と `accent` がどちらも
`0033CC` だった)。PPTX のテーマの accent1〜6 にもこの6つを写す(次の節)。
light の `conditional` は `9A6700`(黄土。白の文字 `on_conditional` を載せても読める濃さ)。

#### PPTX のテーマへの写し方(Backlog #43)

theme の色は、スライドの図形に直接入るほか、PPTX のテーマ(`ppt/theme/theme1.xml` の
`a:clrScheme`)とスライドマスターにも写す。PowerPoint で人が新しく足した文字・図形・
表が、テンプレートの色で描かれるようにするため。テーマ・配色・字体の組の名前
(`a:theme`・`a:clrScheme`・`a:fontScheme`・`a:fmtScheme` の `name`)はテンプレートの
名前(外の値の表ならファイル名から取った name)。

| テーマの枠 | 地が明るいテーマ(light・forest) | 地が暗いテーマ(dark) |
|---|---|---|
| `dk1` | `text_primary` | `bg` |
| `lt1` | `bg` | `text_primary` |
| `dk2` | `text_muted` | `card_alt` |
| `lt2` | `card_alt` | `text_muted` |
| `accent1` | `accent` | 同じ |
| `accent2` | `conditional` | 同じ |
| `accent3` | `support` | 同じ |
| `accent4` | `danger` | 同じ |
| `accent5` | `none` | 同じ |
| `accent6` | `text_muted` | 同じ |
| `hlink` | `accent` | 同じ |
| `folHlink` | `text_muted` | 同じ |

- **明るいか暗いか**は `bg` と `text_primary` の相対輝度(WCAG の式)で決める。`bg` の
  ほうが暗ければ暗いテーマ。テンプレートに印は持たせない(色から決まるので食い違わない)。
- **dk/lt は「暗い色・明るい色」の枠**なので、暗いテーマでは地の色を `dk1` に置き、
  スライドマスターの `p:clrMap` を `bg1="dk1" tx1="lt1" bg2="dk2" tx2="lt2"` に向ける
  (PowerPoint の暗いテーマと同じ作り)。これで「背景 1」「テキスト 1」が地と文字の色に
  なり、新しく足した文字が暗い地の上でも読める。明るいテーマは既定の向き
  (`bg1="lt1" tx1="dk1"`)のまま。
- **ノートのマスター**の `clrMap` はいつも明るい向き。ノートは白い紙に刷る前提のため。
- スライドマスターの背景は `bg` の色。スライドの見た目は変わらない(各スライドが
  自分の図形で描くため。LibreOffice の絵で、従来の PPTX と画素まで同じことを確かめた)。
- `risk`・`grid`・`card`・`on_conditional` はテーマの枠に写さない(12枠に入らない。
  スライドの図形には今までどおり入る)。
- 試験は `boxloom/theme_colors_test.go`(light・dark・examples/templates/forest.json)。

### values(pt。断りの無いものは pt)

| キー | 使うところ |
|---|---|
| `pageWidth` `pageHeight` | ページの大きさ |
| `pagePadding` | 本文・主張のスライドの左右の外周 |
| `pagePaddingY` | 本文・主張のスライドの上下の外周(題の上・フッターの下) |
| `coverPadding` | 表紙の外周 |
| `sectionGap` | 題(とリード文)・本文・出典・フッターの間 |
| `blockGap` | 本文のブロックの間、比較のカードの間 |
| `itemGap` | 箇条の項目の間、カードの中の行の間 |
| `headerGap` | 題とリード文の間 |
| `coverTitle` `coverSubtitle` `coverAuthor` | 表紙の字の大きさ |
| `slideTitle` | 本文のスライドの題(短い見出し。折り返すと `title-wrap` の警告) |
| `leadText` | リード文(`lead`。題の直下、色は `text_primary`、太字にしない) |
| `body` | 段落・箇条 |
| `subPoint` | 入れ子の箇条 |
| `optionLabel` `optionItem` | 比較の選択肢の名前、利点・欠点 |
| `small` | 注記・フッター・図の凡例 |
| `figureDetail` | 図の補足:process・orgchart の箱の2段目、process の段階の下の文と図の説明(dark / light は 12) |
| `captionText` | 表・画像・画像の並びの説明の文(dark / light は 12。色は `text_muted`) |
| `sectionTitle` | 区切り(section)の章の題 |
| `sectionTracker` | 真偽。`true` なら、区切りが2つ以上ある文書の区切りのスライドに、章の一覧と現在地を題の下に置く(いまの章は帯がアクセント・題が `text_primary` の太字、ほかは帯が `grid`・題が `text_muted`)。`false` なら置かない |
| `sectionTrackerText` | 章の一覧の字の大きさ |
| `statementText` | 主張(statement)の1文 |
| `takeawayText` | 結論(takeaway)の1文 |
| `statValue` `statUnit` `statLabel` | 統計強調(stat)の数・単位・説明 |
| `quoteText` `quoteMark` `quoteSource` | 引用(quote)の言葉・頭の印・出どころ |
| `pointIndentEm` | 入れ子の箇条の字下げ。親の字の大きさに掛ける倍率 |
| `cardPadding` `cardRadius` | 比較のカード |
| `subsystemRadius` | サブシステムを載せるカードの角 |
| `coverRuleWidth` `coverRuleHeight` | 表紙の題の下の線。区切りの題・主張の1文の上の線にも使う |
| `statementInset` | 主張のスライドの文を、本文のスライドの外周(`pagePadding`)よりさらに内へ下げる幅 |
| `accentBarWidth` | 結論(takeaway)の左のアクセントの帯の幅 |
| `fillRatioMin` | 本文の埋まりがこの割合(0〜1)未満なら `fill-ratio` の警告 |
| `columnTitle` | 並列のまとまりの題(dark / light は 16) |
| `columnGap` | 並列のまとまりの間(32) |
| `columnMainWeight` | 主のまとまり(`role: main`)の幅の比。ほかのまとまりは 1(2。倍率なので段階に載せない) |
| `tableText` | 表のセルの字(12。見出しの行は太字) |
| `tableCellPaddingX` `tableCellPaddingY` | セルの左右・上下の余白(8・4) |
| `tableRuleWidth` | 表の行の間の線の太さ(1。線の色は役割名 `grid`、見出しの行の塗りは `card_alt`、本体は `card`) |
| `imageMinHeight` | 画像を縮めてよい下限の高さ(96。はみ出しの検査はこの高さで測る) |
| `captionGap` | 表・画像と説明の文の間(4。説明の文の字は `captionText`) |

### marks(印の文字)

箇条(`point`、入れ子は `subPoint`)、比較の利点(`pro`)・欠点(`con`)の頭に置く
文字、引用の言葉の頭(`quote`)と出どころの頭(`quoteSource`)、統計強調の数と単位の
間(`statUnit`。dark / light は空=「71%」「38社」と詰める。空でもよいのはこれだけ)。
後ろの空きも含めて書く。

`point`・`subPoint`・`pro`・`con` は、PPTX では本文の文字にせず、段落の箇条の印
(`a:buChar`。色は印の色、字体は印の文字で選ぶ:U+2000 未満は `latin`、以降は
`eastAsian`)として書く。本文は印の幅(後ろの空きを含めて測ったとおり)だけ字下げ
した位置から始まり、折り返した行は印の下に戻る(測りと同じ)。印を文字にすると、
描く側が和文と欧文の間に足す空き(LibreOffice の「アジア言語と欧文の間隔」)が
「・」と数字・英字の間にだけ入り、数字で始まる箇条だけ頭がずれて見えたため(#42)。
印の後ろの空白は `buChar` に入れず、字下げの幅にだけ数える。印の太さは指定
できない(描く側の決まりに従う)。

スライドの出典(`source`)の頭に置く文字は `slideSource`(dark / light は「出典:」)。
文書の日付(`meta.date`)の見せ方は `date`(日のある日付)と `yearMonth`(`YYYY-MM` の
日付)で、`{YYYY}`(4桁の年)・`{MM}`・`{M}`(0 を付ける・付けない月)・`{DD}`・`{D}`
(日)を数に置き換える。dark / light は `{YYYY}年{M}月{D}日` と `{YYYY}年{M}月`。
読み込みで、`{YYYY}` と月の印が無い・`date` に日の印が無い・`yearMonth` に日の印がある・
知らない `{…}` があるのは誤り。

出典・日付・機密区分・免責・表の合計の行の置き場所(骨組み。コード):

| 意味 | dark / light |
|---|---|
| リード文 | 題の直下(`leadText`・`text_primary`、題との間は `headerGap`)。本文の領域はこの分だけ狭くなる |
| スライドの出典 | 本文の下・フッターの上(`small`・`text_muted`、間は `sectionGap`)。本文の領域はこの分だけ狭くなる |
| 日付 | 表紙の作者の下(`coverAuthor`・`text_muted`) |
| 機密区分 | 表紙の日付の下と、本文・主張のスライドのフッターの作者の右(`small`・太字・`text_primary`) |
| 免責 | 表紙の一番下(`small`・`text_muted`) |
| 表の合計の行 | 太字、上の線を `text_primary` に(`tableRuleWidth`)。PPTX の `a:tblPr` に `lastRow="1"` |

### fonts(字体)

欧文の字体(`latin`)・和文の字体(`eastAsian`)・置き換わり得る字体
(`fallbackFonts`)・幅の余裕(`widthMargin`)・行の送りを固定するか
(`fixedLineSpacing`)。字体は寸法(`boxloom/fonts/*.json`)を持つものだけ選べ、
無ければ誤り。和文の字体は仮名と漢字を持つ字体に限る。`widthMargin` は 1〜1.5。
意味・測った数字・推奨する値は `docs/fonts.md`。

### variants

このテンプレートが用意した変種の名前(IR の `variant`)。dark / light は空。

### subsystems.gantt(任意)

書くと、このテンプレートで作る Gantt だけがその寸法で測られ、描かれる
(登録簿の `Override` で差し替える。既定の表は変わらない)。形は次の節と同じで、
全部のキーが要る。dark / light は書いていない(既定を使う)。

### subsystems.shapes(任意)

`subsystems.gantt` と同じく、書くとこのテンプレートの shapes だけがその寸法になる。
形は下の「subsystems/shapes.json の形」で、全部のキーが要る。dark / light は書いていない。

shapes の文字の大きさは、作者が `textStyle` に書く段の名前で values から引く:
`small`・`subPoint`・`optionItem`・`body`・`optionLabel`(`docs/subsystem.md` の8節)。
テーマに無い役割名を shapes が使うと、このテンプレートの組み立てが断る。

### subsystems.process・orgchart・matrix・funnel(任意)

同じく、書くとこのテンプレートのその図だけがその寸法になる。全部のキーが要り、`padding`・
間隔・文字の余白(Go の json タグに `scale:"space"` の付いたもの)は段階の刻みに載って
いなければ誤り。差し替えないときは既定のファイルの値を、そのテンプレートの段階で見る。
dark / light は書いていない。図の字の大きさはファイルに持たず、values の `body`(題)・
`subPoint`(2段目・セル・注記)・`small`(補足・説明・凡例)から引く。キーの意味は
各ファイルと Go の型(`ProcessValues`・`OrgChartValues`・`MatrixValues`・`FunnelValues`)の
注釈に書いた。最小・自然・上限の3つ組(`minBoxHeight`・`naturalBoxHeight`・
`maxBoxHeight` など)は Measure の Min・Natural・Max になる。

process・orgchart・funnel の状態の凡例(`docs/subsystem.md` の11節)の寸法も各ファイルに
ある:`legendGap`(図と凡例の間)・`legendItemGap`(項目の間)・`legendSwatchWidth`・
`legendSwatchHeight`(見本の箱)・`legendSwatchGap`(見本と文の間)。funnel の `rateGap` は
帯の列と転換率の列の間。matrix の `emphasisLineWidth` は推す列の枠線の太さで、
`naturalRowHeight` 36・`maxRowHeight` 44 は1行の文のセルが間延びしない高さ。

### subsystems.chart(任意)

同じく、書くとこのテンプレートのグラフだけがその寸法になる。全部のキーが要り、`padding`
(段階の刻みに載ること)のほかは 0 より大きい数。グラフの字の大きさは values の
`subPoint`、色はテーマの役割名(`accent`・`text_muted`・`none`・`grid`・`bg`)から引く。

| キー | 意味 |
|---|---|
| `naturalWidth` `minWidth` | 幅が分からないときの自然な幅、最小の幅 |
| `minPlotHeight` | 値の軸の長さ(縦の形)・円の直径の最小。Measure の Min のもと |
| `naturalAspect` | 縦の形・円・散布図の自然な高さ(幅に対する割合) |
| `minBarSlot` `naturalBarSlot` `maxBarSlot` | 横棒の1項目の高さ(Min・Natural・Max) |
| `columnMaxPoints` | 時系列を縦棒で描く点の数の上限(越えたら折れ線) |
| `gapWidth` `histogramGapWidth` | 棒の間(棒の幅に対する %)、ヒストグラムの棒の間 |
| `lineWidth` `markerSize` `scatterMarkerSize` | 折れ線の太さ、折れ線・散布図の点の大きさ(pt) |
| `axisLineWidth` `gridLineWidth` `pieOutlineWidth` | 項目の軸の線・目盛りの線・円の扇の境の太さ |
| `labelMaxLines` `barLabelFraction` | 項目の名前を折り返してよい行の数、横棒の名前に使える幅の割合(`chart-labels` の基準) |
| `padding` | アプリがグラフの内側に取る余白の見積もり(Measure と `chart-labels` に使う) |

## subsystems/gantt.json の形

| キー | 意味 |
|---|---|
| `padding` | 箱の内側の余白 |
| `legendHeight` `legendGap` | 凡例の行の高さ、凡例と上の注釈のレーンの間 |
| `gutterGroupWidth` `gutterSubgroupWidth` `gutterLabelWidth` | 左の見出しの列の幅(グループ・サブグループ・行の名前) |
| `milestoneHeight` `axisLabelHeight` | 節目の行、目盛りの文字の行 |
| `groupGap` | グループの間 |
| `minRowHeight` `naturalRowHeight` `maxRowHeight` | 行の高さの下限・自然な高さ・上限(Measure の Min・Natural・Max になる) |
| `naturalChartWidth` `minChartWidth` | 日付軸の部分の自然な幅・最小の幅 |
| `annotationFixedWidth` `annotationFixedHeight` | 手で置いた注釈(Direction が up/down)の箱 |
| `annotationMinWidth` `annotationMaxWidth` | 自動の注釈の箱の幅の範囲 |
| `annotationPaddingX` `annotationPaddingY` | 自動の注釈の箱の中の余白 |
| `annotationLaneGap` | 注釈のレーンとチャートの間 |
| `annotationText` | 注釈の字の大きさ |
| `labelText` `smallText` | 凡例・行の名前・目盛りの字、グループ名・区間の文字・節目の字 |
| `dateFormat` | IR から作った Gantt の目盛りと節目の日付の書き方(文字列)。`YYYY`・`YY`・`MM`・`M` を年と月に置き換え、ほかの文字はそのまま。既定 `YYYY/M`(`2026/12`)。月(`M` か `MM`)を含まない・年を `YYYY`/`YY` 以外で書いたものは誤り。Go で組んだ従来のデモの目盛りは手で書いた文字のまま |

## subsystems/shapes.json の形

| キー | 意味 |
|---|---|
| `padding` | 枠の内側の余白。相対座標の 0〜1 はその内側を指す |
| `naturalWidth` | 幅が分からないときの自然な幅 |
| `defaultAspect` | `aspect` を省いたときの内側の縦横比(幅÷高さ) |
| `minWidth` `minHeight` | 最小の大きさ(高さは、図形の文が収まる高さとの大きいほう) |
| `textInsetX` `textInsetY` | 図形の中の文字の余白 |
| `lineWidth` | line・arrow・connector の太さ |
| `outlineWidth` | 図形の枠線の太さ |
| `cornerRadius` | roundRect の角 |

描く処理の中の細かい数(バーと行の間の 3pt、凡例の色見本の大きさなど)は、
描き方の一部としてコード(`gantt.go`)に残した。

## コードに残したもの

- **骨組みと部品**(`template.go`):表紙・本文のスライドの並び、ブロックの組み方。
- **従来のデモ**(`go run ./cmd/boxloom`、`demo.go` の `buildSlideSpec`):v11 の再現なので、
  題の大きさなどの数はそのまま。色はテンプレートのファイル(dark / light)から引く。
- `fill-ratio` の上限(100%)。はみ出しはテンプレートの方針ではなくコアの検査なので。
- 描く処理の中の細かい数(Gantt のバーの余白・凡例の色見本・手で置いた注釈の箱の中の
  余白など)は段階の検査の外にある。

## 確かめたこと

- 移す前と後で、従来のデモ2枚と、examples の IR・Markdown(直下のもの)を dark / light で
  作った PPTX(合わせて14件)の中身が完全に同じ(展開して `diff -r`)。`schema` の出力と、
  `check` の例への結果も同じ。
- 値を変えると出力が変わる(`body` を変えると本文のスライドが変わる)。
- `values_test.go`:同梱のファイルが読める/知らないキー・欠けたキー・null・型の違い・
  0・役割名の欠けと余り・色の形・空の印・名前の食い違い・Gantt の値の一部だけ・
  知らないサブシステム・後ろに続くもの、をどれも断る/`subsystems.gantt` の差し替えが
  そのテンプレートの Gantt の高さだけを変える。
