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
- 印の文字が空。
- `variants` に空の名前・同じ名前が2回。

同梱のファイルが壊れていれば試験(`values_test.go`)が落ちる。

## templates/<名前>.json の形

```json
{
  "name": "light",
  "theme":  { "bg": "FFFFFF", "card": "F7F9FD", "...": "..." },
  "scale":  { "spacingUnit": 4, "typeSizes": [8, 10, 12, 14, 16, 24, 36] },
  "values": { "pageWidth": 960, "pageHeight": 540, "...": 0 },
  "marks":  { "point": "・", "subPoint": "– ", "pro": "＋ ", "con": "－ " },
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
| 余白 | `pagePadding` `coverPadding` `sectionGap` `blockGap` `itemGap` `cardPadding` | `padding` `legendGap` `groupGap` `annotationPaddingX` `annotationPaddingY` `annotationLaneGap` | `padding` `textInsetX` `textInsetY` |
| 字 | `coverTitle` `coverSubtitle` `coverAuthor` `slideTitle` `body` `subPoint` `optionLabel` `optionItem` `small` | `annotationText` `labelText` `smallText` | (字は values の段の名前で引く) |

ページの大きさ・角の丸み・線の太さ・行の高さ・欄の幅・倍率は段階に載せない
(図の寸法で、刻みに合わせても読みやすさは変わらないため)。

- 検査:段階そのもの(刻みが 0 以下、段が空・小さい順でない)と、段から外れた値は誤り。
  誤りの文は近い段の値を添える(例 `values.subPoint: 12.5 is not on the type scale
  [8 10 12 14 16 24 36]; nearest: 12 or 14`)。
- サブシステムの寸法は、そのテンプレートの段階で見る。テンプレートが差し替えない
  ときは既定のファイル(`subsystems/gantt.json` など)の値を見る。段を狭めた
  テンプレートは、既定の Gantt の字が段から外れれば読み込みで断られる。
- dark / light は `spacingUnit` 4、`typeSizes` [8, 10, 12, 14, 16, 24, 36] の7段。
  段の役割はおおむね 36 表紙の題・24 スライドの題・16 小見出し(表紙の副題・
  比較の選択肢の名前)・14 本文・12 従の文(入れ子の箇条・利点と欠点・表紙の作者)・
  10 注(注記・フッター・Gantt の目盛りと行の名前)・8 図の中の細かい字。

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

### theme(役割名 → 色)

`bg` `card` `card_alt` `text_primary` `text_muted` `grid` `support`
`conditional` `none` `danger` `risk` `accent` `on_conditional` の 13 個、ちょうど。

### values(pt。断りの無いものは pt)

| キー | 使うところ |
|---|---|
| `pageWidth` `pageHeight` | ページの大きさ |
| `pagePadding` | 本文のスライドの外周 |
| `coverPadding` | 表紙の外周 |
| `sectionGap` | 題・本文・フッターの間 |
| `blockGap` | 本文のブロックの間、比較のカードの間 |
| `itemGap` | 箇条の項目の間、カードの中の行の間 |
| `coverTitle` `coverSubtitle` `coverAuthor` | 表紙の字の大きさ |
| `slideTitle` | 本文のスライドの題 |
| `body` | 段落・箇条 |
| `subPoint` | 入れ子の箇条 |
| `optionLabel` `optionItem` | 比較の選択肢の名前、利点・欠点 |
| `small` | 注記・フッター |
| `pointIndentEm` | 入れ子の箇条の字下げ。親の字の大きさに掛ける倍率 |
| `cardPadding` `cardRadius` | 比較のカード |
| `subsystemRadius` | サブシステムを載せるカードの角 |
| `coverRuleWidth` `coverRuleHeight` | 表紙の題の下の線 |
| `fillRatioMin` | 本文の埋まりがこの割合(0〜1)未満なら `fill-ratio` の警告 |

### marks(印の文字)

箇条(`point`、入れ子は `subPoint`)、比較の利点(`pro`)・欠点(`con`)の頭に置く
文字。後ろの空きも含めて書く。

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
