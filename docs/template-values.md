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
| `boxloom/values.go` | 読み込みと検査 |

どちらも `go:embed` でバイナリに同梱する(外のファイルを指定して読む口はまだ無い)。
テンプレートの一覧はこのフォルダのファイルから作るので、`templates/` に
ファイルを1つ足せば `-template <名前>` で選べる。

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
- 印の文字が空。
- `variants` に空の名前・同じ名前が2回。

同梱のファイルが壊れていれば試験(`values_test.go`)が落ちる。

## templates/<名前>.json の形

```json
{
  "name": "light",
  "theme":  { "bg": "FFFFFF", "card": "F7F9FD", "...": "..." },
  "values": { "pageWidth": 960, "pageHeight": 540, "...": 0 },
  "marks":  { "point": "・", "subPoint": "– ", "pro": "＋ ", "con": "－ " },
  "variants": [],
  "subsystems": { "gantt": { "...": "subsystems/gantt.json と同じ形" } }
}
```

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

### variants

このテンプレートが用意した変種の名前(IR の `variant`)。dark / light は空。

### subsystems.gantt(任意)

書くと、このテンプレートで作る Gantt だけがその寸法で測られ、描かれる
(登録簿の `Override` で差し替える。既定の表は変わらない)。形は次の節と同じで、
全部のキーが要る。dark / light は書いていない(既定を使う)。

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

描く処理の中の細かい数(バーと行の間の 3pt、凡例の色見本の大きさなど)は、
描き方の一部としてコード(`gantt.go`)に残した。

## コードに残したもの

- **骨組みと部品**(`template.go`):表紙・本文のスライドの並び、ブロックの組み方。
- **従来のデモ**(`go run .`、`main.go` の `buildSlideSpec`):v11 の再現なので、
  題の大きさなどの数はそのまま。色はテンプレートのファイル(dark / light)から引く。
- `fill-ratio` の上限(100%)。はみ出しはテンプレートの方針ではなくコアの検査なので。

## 確かめたこと

- 移す前と後で、従来のデモ2枚と、examples の IR・Markdown(直下のもの)を dark / light で
  作った PPTX(合わせて14件)の中身が完全に同じ(展開して `diff -r`)。`schema` の出力と、
  `check` の例への結果も同じ。
- 値を変えると出力が変わる(`body` を変えると本文のスライドが変わる)。
- `values_test.go`:同梱のファイルが読める/知らないキー・欠けたキー・null・型の違い・
  0・役割名の欠けと余り・色の形・空の印・名前の食い違い・Gantt の値の一部だけ・
  知らないサブシステム・後ろに続くもの、をどれも断る/`subsystems.gantt` の差し替えが
  そのテンプレートの Gantt の高さだけを変える。
