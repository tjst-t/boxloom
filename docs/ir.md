# 意味の中間表現(IR)

Boxloom のコンテンツは、どの記法で書いても最後は **IR(意味の中間表現)** になり、
テンプレートは IR だけを見る。IR は JSON で、形は JSON Schema(draft 2020-12)で
決める。

> 状態: 第一版の下書き(版 `0.1`)。`docs/content-syntax-comparison.md` の共通の
> お題3枚(表紙・論点と比較・Gantt)をすべて表せる範囲で作った。Go 版は
> IR を読んで PPTX を書ける(`go run ./cmd/boxloom build`、`docs/build.md`)。Boxloom Markdown
> から IR へのパーサーもある(`go run ./cmd/boxloom parse`、`docs/content-syntax.md` の8章)。

## ファイル

| 場所 | 中身 |
|---|---|
| `schema/core.schema.json` | コア:文書・スライド・ブロック・インライン・サブシステムの差し込み口 |
| `schema/subsystems/gantt.schema.json` | Gantt サブシステムのブロック |
| `schema/subsystems/shapes.schema.json` | shapes サブシステム(生の図形への逃げ道。`docs/subsystem.md` の8節)のブロック |
| `schema/subsystems/process.schema.json`・`orgchart`・`matrix`・`funnel` | 箱と線の図のサブシステム(`docs/subsystem.md` の11節)のブロック |
| `schema/subsystems/chart.schema.json` | 数値のグラフ(`docs/subsystem.md` の12節)のブロック。比べ方 `compare`・項目・系列・単位・強調だけを持つ |
| `schema/boxloom-ir.schema.json` | 検査の入口。コアに同梱のサブシステムを差し込んだもの |
| `examples/ir/demo-deck.json` | お題3枚を IR で書いた例 |
| `examples/ir/gantt-full.json` | デモの Gantt 全体(3グループ・9行・注釈4つ)を IR で書いた例 |
| `examples/ir/shapes.json` | shapes で描いた架空のシステム構成図の例 |
| `examples/ir/text-patterns.json` | 文字のパターン(区切り・主張・結論・統計強調・引用)の例(`examples/md/text-patterns.bx.md` の IR) |
| `examples/ir/figures.json` | process・orgchart・matrix・funnel の例(`examples/md/figures.bx.md` と同じ IR) |
| `examples/ir/blocks.json` | 並列のまとまり・表・画像・画像の並びの例(画像は `examples/ir/images/`。`tools/gen_example_images.go` で作った架空の図) |
| `examples/ir/charts.json` | chart の例。比べ方の5分類ごと(`examples/md/charts.bx.md` と同じ IR) |
| `examples/ir/invalid/` | 断られるべき例(色・位置・見た目の装飾などを1か所だけ足したもの) |
| `tools/validate_ir.py` | 検査のコマンド |

```sh
pip install jsonschema                         # 4.18 以降
python3 tools/validate_ir.py deck.json         # 誤りを JSON で出す(正しければ exit 0)
python3 tools/validate_ir.py --self-test       # 例が通り、断られるべきものが断られるか
```

誤りは `docs/content-syntax.md` の5章の形で返す(Go の `boxloom check`・Markdown の
パーサーと同じ形)。`path` は IR の中の場所(JSON Pointer)で、AI に渡せばその場所を
直せる。IR を直接検査するので `line`・`column` は無い。`code` はいつも `schema` で、
当たった Schema の規則は `hint` に、選べる値が決まっていれば `expected` に入れる。

```json
{"file": "deck.json",
 "path": "/slides/2/body/0/data/groups/0/rows/1/segments/0/status",
 "code": "schema",
 "message": "'#34C08F' is not one of ['support', 'conditional', 'none', 'danger']",
 "expected": ["support", "conditional", "none", "danger"],
 "hint": "Schema の規則: enum"}
```

## 形の要点

```
文書   { boxloom: "0.1", template?, meta?, slides: [スライド] }
スライド  { type: "cover",     variant?, title?, subtitle?, author?, notes? }
         { type: "content",   variant?, title, body: [ブロック], notes? }
         { type: "section",   variant?, title, notes? }          章の区切り(本文なし)
         { type: "statement", variant?, content, notes? }        言うことが1文だけ(本文なし)
ブロック  paragraph | points | compare | note | takeaway | stat | quote | columns | table | image | gallery | subsystem   (どれも type で選ぶ)
         takeaway { content }                                     本文の中の結論の1文
         stat     { items: [{ value: "3 → 12", unit?: "機種", label }] }   1〜4個
         quote    { content, source }                             出どころは必須
並列     { type: "columns", columns: [{ title?, role?: "main", body: [paragraph|points|note|table|image] }] }  (2〜3個)
表       { type: "table", header: [セル], rows: [[セル]], caption? }   (セルはインライン。どの行も見出しと同じ数)
画像     { type: "image", src, alt, caption? }   /  { type: "gallery", images: [{ src, alt, caption? }] }  (2〜6枚)
インライン "文字列"  か  ["文字列", {type: em|code|link|br, …}, …]
サブシステム { type: "subsystem", subsystem: "gantt" | "shapes" | "process" | "orgchart" | "matrix" | "funnel" | "chart", variant?, data: {…} }
```

`notes` は発表者ノート。文字だけの文字列で、改行(`\n`)が段落の区切り。PPTX では
そのスライドのノートに1行1段落で入る(空の行は空の段落)。ノートに入るのは作者の
文だけで、組み立ての警告は check・build の出力にだけ出る(`docs/build.md`)。Markdown では
`:::notes` で書く(`docs/content-syntax.md` の3.6)。

例(論点のスライドの一部):

```json
{ "type": "points",
  "items": [
    ["第1世代(Board A)は ", {"type": "em", "content": "2029年12月"}, " にハードウェア保守が切れる"],
    "Loom 4 は Store X 非対応のため、第1世代は移行先になり得ない" ] }
```

## 設計判断

### 1. 意味だけを持ち、知らないキーは断る

すべての object に `additionalProperties: false` を付けた。`color`・`align`・`y`
のようなキーを足すと、その場所で断られる。分離を規約ではなく **検査で** 守るため。
Markdown のように「何でも何かとして通る」記法を入口にしても、IR に落とした時点で
漏れ込みが見つかる。

例外として、文書の根だけに `$schema` を許した(エディタや AI に Schema を
知らせるため。検査には使わない)。

### 2. 種類は `type` で選び、`oneOf` ではなく `if/then` で振り分ける

スライド・ブロック・インライン要素は、どれも `type` の値で種類を決める。
Schema では `oneOf` を使わず、`type` の値ごとの `if/then` で振り分けた。

`oneOf` だと、1か所の誤りが「どの候補にも合わない」になり、誤りの文が的外れになる
(下書きの段階で、スライドに `color` を足した誤りが `'cover' was expected` と
出た)。`if/then` にすると「content スライドの中の、`color` というキーが余計」と
直す場所がそのまま出る。AI に直させるにはこの差が大きい。

### 3. IR は記法より明示的にする

記法では省ける・暗黙になるものも、IR では書き下す。

- 表紙は `meta` から作るのではなく、`{type: "cover"}` というスライドとして置く
  (中身を省くと `meta` の値を使う)。記法のフロントマターは、パーサーが表紙の
  スライドに変える。
- 範囲は `"2026-09 .. 2034-11"` のような文字列ではなく `{from, to}`。
- 複数行のラベルは `"\n"` を埋めず、行の配列(`["第1世代", "Board A", …]`)。

IR は人が書くものではなく、パーサーの出力・AI の構造化出力・他のプログラムの
入力になるものなので、読み取りの曖昧さを無くすほうを取った。

### 4. インラインは意味の4種類だけ

`em`(強調)・`code`・`link`・`br`(意味のある改行)。`bold`・`italic`・
`color`・大きさは無い。強調を太字にするか色にするかはテンプレートが決める。
`strong` と `em` の2段階は、いまは要る例が無いので入れていない(要れば
`em` に段階を持たせるか `strong` を足す)。

### 5. 変種(variant)は名前の形だけを検査する

スライドとブロックとサブシステムに `variant` を付けられる(例 `"dense"`)。
どの名前があるかはテンプレートが決めるので、Schema は形
(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)だけを見る。名前が実在するかは、テンプレートを
読んだ検査(将来の `boxloom check`)で見る。

### 6. サブシステムは `$dynamicRef` の差し込み口に入る

コアのブロックの1種類 `subsystem` は、`{"$dynamicRef": "#subsystem"}` で受ける。

- **コアだけ**で検査すると、差し込み口は既定の形(名前と `data` があるか)だけを
  見る。`data` の中身は見ない。
- **入口の `boxloom-ir.schema.json`** は、同じ `$dynamicAnchor: subsystem` を
  持ち、`subsystem` の名前で各サブシステムの Schema に振り分ける。登録の無い名前は
  断る。

サブシステムを足すときは、`schema/subsystems/<名前>.schema.json` を書き、入口の
`enum` と `allOf` に1つずつ足す。コアの Schema は触らない。テンプレートが自分の
サブシステムを持つときは、同じ形の入口をテンプレート側で作れば、そのテンプレート
用の検査になる。

Go からは、サブシステムの Schema を登録簿の `Declare`(`SubsystemDecl.Schema`)で
名乗らせる。`boxloom schema` はテンプレートの登録簿から、そのテンプレートで書ける
サブシステムだけを差し込んだ1つの Schema(`irSchema`)を組む
(`docs/schema-command.md`)。Go は `schema/` の写し `boxloom/schemas/` を埋め込む
(直したら `go generate`)。

ブロックの `type` を `"gantt"` にせず `"subsystem"` + 名前にしたのは、コアの
ブロック名とサブシステム名の名前空間を分けるため(サブシステムがいくら増えても、
コアのブロックの名前と衝突しない)。

### 7. Gantt は「期間・状態・注釈」だけを持つ

いまのデモの `GanttChart`(Go の構造体)から、意味でないものを外した。

| デモの構造体にあるもの | IR では | 理由 |
|---|---|---|
| `Scale`(軸の圧縮の制御点) | 持たない。`axisBreakBefore`(軸を切る日付)だけ | 圧縮の具合は見た目の調整。サブシステムが目盛りから決める(変種で選べるようにしてよい) |
| `Tick.MainLabel`(`"29/12"`) | 持たない。`at` から作る | 日付の書式はテンプレートが決める |
| `Tick.Role = "text_primary"` | 持たない。`status` は省けば「普通」 | `text_primary` はデザイン側の役割名 |
| `Annotation.AnchorYFrac`・`Direction` | 持たない。`target`(グループか行の id)と `emphasis` | 位置と上下は、サブシステムが重なりを避けて決める |
| 状態 `support`/`conditional`/`none`/`danger` | `status`(enum) | Gantt サブシステムが名乗る意味役割名 |
| `risk` | グループの `risk: {from, to}` と凡例のキー | リスクの期間という意味 |
| `accent`・`on_conditional` | 持たない | デザイン側の役割名(節目の色、帯の上の文字色) |
| `ExtNote*`(延長予定の破線) | 区間の `tentative: true` | 「確定していない区間」という意味。破線で描くかはテンプレート |

状態の名前は Gantt サブシステムの Schema の `enum` に固定した。状態を作者が
増やしたい場面が出たら、チャートの中で状態を宣言する形にするか、テンプレートが
状態の一覧を持つ形にするかを決める。

### 8. shapes は座標を持つが、枠の中の相対だけ

IR は座標・色を持たない(設計判断1)が、shapes サブシステムだけは例外として、
**サブシステムの枠の中の 0〜1 の相対座標** と **テーマの役割名の色** を持つ。既存の
パターンで表せない図のための最後の手段で、スライドの上の位置や16進の色は書けない
(`docs/subsystem.md` の8節)。範囲(和が 1 以下)・役割名・コネクタの指す先は、
Schema では書けないので `boxloom check` が見る。

### 9. 文字のパターン(section・statement・takeaway・stat・quote)

`docs/patterns.md` の R1〜R3・R11 の型。意味から形を決めた(記法は
`docs/content-syntax.md` の3.2・3.4、決めたことの一覧は `docs/patterns.md` の3.2.1)。

- **区切り・主張は本文(`body`)を持たない。** 持てないことを Schema の
  `additionalProperties: false` で表す(主張に理由の箇条を足す、区切りに目次を足す、という
  R1・R2 の悪い例を形で断る)。
- **区切りは `title`、主張は `content`。** 区切りの文は章の「題」。主張の1文は題ではなく
  中身そのもの(段落・結論と同じ `content`)。どちらも PPTX では題のプレースホルダーに
  書く(テンプレートの決めごと。`docs/build.md`)。
- **`stat` の `value` は文字列。** 書いたとおりに出し(`"3 → 12"`・`"−40"`・`"1,200"`)、
  計算しない。数字(0〜9 か全角の0〜9)を1つ以上含むことを Schema の `pattern` で見る。
  `unit` は飾りの無い文字列、`label` はインライン。並べ方(1〜4個を1行に)と数の字の大きさ
  (値の表の `statValue`、段は `scale.typeSizes`)はテンプレートが決める。個数の 1〜4 は
  Schema の `minItems`・`maxItems`。
- **`quote` の `source` は必須。** 出どころの無い他人の言葉は R11 の悪い例になるため。
  引用符は IR に書かない(付けるか・何を付けるかはテンプレートの印 `marks.quote`)。
- Schema で書けない「見える文字があるか」は `boxloom check` の `empty-text`(下の表)。

### 10. 並列のまとまり・表・画像(#20・#21・#22)

- **並列のまとまり**の幅の比は数で持たない。主のまとまりに `role: "main"` を付ける
  (意味)と、テンプレートが広く取る。2個以上の `main` は Schema(`maxContains`)と
  Go の読み込みが断る。中のブロックの種類は `columnBlock` に限る(`columns` の入れ子は断る)。
- **表**は見出しの行(`header`)を必須にした(GFM の表がいつも持つため)。列の配置・幅は
  持たない。行のセルの数が見出しと違うのは Schema では書けないので `boxloom check` の
  `table-shape`。
- **画像**の `src` は文書のファイルからの相対パス(区切りは `/`)。URL・絶対パス・
  文書のフォルダの外(`..`)・`\` は Schema の `pattern` と `boxloom check` の
  `image-path` が断る。IR に画像の中身(base64 など)は持たない(意味の木を軽く保ち、
  差分で読めるように)。代わりの文 `alt` は必須で、空白だけも不可。
  `caption` は画像の下に出す説明の文(インライン)。

## 意味の検査(boxloom check)

JSON Schema では書けない検査は Go の `boxloom check` がする。

```sh
cd boxloom
go run ./cmd/boxloom check ../examples/ir/demo-deck.json            # 誤りが無ければ exit 0
go run ./cmd/boxloom check ../examples/md/demo-deck.bx.md            # Markdown もそのまま検査する
go run ./cmd/boxloom check -format json deck.json                    # 機械が読む形(既定は1件1行の text)
go run ./cmd/boxloom check -strict deck.json                         # 警告も exit 1 にする
go run ./cmd/boxloom check -template dark deck.json                  # IR の template の代わりに
```

誤りと警告は `docs/content-syntax.md` の5章の形(`file`・`line`・`column`・`path`・
`code`・`message`・`expected`・`hint`)で出す。誤りは最初の1件で止めず全部集める。

- IR(JSON)を検査したときは `line`・`column` を出さない。
- Markdown を検査したときは、記法の誤りならパーサーの誤り(`line`・`column` つき)を
  そのまま返す。記法が通れば、できた IR を同じように検査し、誤りと警告には IR の
  場所(`path`)を作った Markdown の行を `line` に添える(`column` は無い)。

```json
{"ok": false,
 "errors":   [{"file": "deck.bx.md", "line": 23,
               "path": "/slides/0/body/0/data/milestones/0/at", "code": "within-span",
               "message": "2036-05 is outside the chart span 2026-09 .. 2034-11"}],
 "warnings": [{"file": "deck.json", "path": "/slides/1", "code": "fill-ratio",
               "message": "underfilled: 50.8% (< 65%)"}],
 "capacity": []}
```

`capacity` は本文の容量(下の「容量の計測」。誤りのあるファイルの分は空)。

| code | 見ること |
|---|---|
| `unique-id` | id は文書全体で一意(スライドやチャートをまたいでも重ならない) |
| `target-exists` | 注釈の `target` が、同じチャートのグループか行の id を指す。shapes のコネクタの `from`・`to` が、同じ図の図形の id を指す(別のチャート・図の id なら、その場所を添えて断る) |
| `date-order` | `from` が `to` より前(同じ日も断る。長さ0の区間は描けない) |
| `date-valid` | 日付が暦の上で実在する(Schema の正規表現は 2028-02-30 を通す) |
| `within-span` | Gantt の区間・リスク・注記・目盛り・節目・注釈・軸を切る日付が、`span` の中にある(外の日付は軸の端に押し込まれて、誤った絵になるため) |
| `template-exists` | `template` の名前のテンプレートがある |
| `variant-exists` | `variant` の名前が、そのテンプレートにある(デモのテンプレートは変種を1つも持たない) |
| `shape-bounds` | shapes の座標・大きさ・線の点が 0〜1 にあり、`x+w`・`y+h` が 1 以下(path は和が越えた `w`・`h`) |
| `role-exists` | shapes の色(`fill`・`line`・`textColor`・`color`)の役割名が、そのテンプレートのテーマにある |
| `connector-self` | shapes のコネクタの `from` と `to` が同じ図形 |
| `empty-text` | 区切りの `title`・主張の `content`・`takeaway` の `content`・`quote` の `content` と `source`・`stat` の各 `label` に、見える文字がある(Schema は `""` や空白だけを通す) |
| `matrix-cells` | matrix の各行の `cells` の数が `columns` の数と同じ(path はその行の `cells`) |
| `table-shape` | 表の行のセルの数が、見出しの行と同じ(path は違う行 `…/rows/R`) |
| `image-path` | 画像の `src` が文書からの相対パス(URL・絶対パス・`..`・`\` は断る) |
| `image-missing` | 画像のファイルが読める(`CheckOptions.Assets` から。コマンドは文書のフォルダ) |
| `image-format` | 画像が PNG か JPEG(中身で見る。SVG・GIF などは断る) |
| `image-alt` | 画像の代わりの文 `alt` が空白だけでない |
| `chart-values` | chart の比べ方とデータが合う:各系列の `values` の数が `categories` の数と同じ・構成比(component)は1系列で負の値が無く合計が正・`stacked` は item・timeSeries で系列が2つ以上・correlation は `xAxis`・`yAxis` と `points` を持ち `categories` などを持たない・系列は3つまで(path はその欄。Schema で書けるものは Schema も断る) |
| `chart-emphasis` | chart の `emphasis` の名前が `categories` にあり、系列が1つ。系列の `emphasis: true` は1つまでで、項目の強調と一緒に使わない |
| `schema` / `build` | Go の IR の読み込み・組み立てが断ったもの(知らないキー・知らない状態など。形の誤り。`schema` は Markdown のパーサー・`validate_ir.py` と同じ code) |

警告(誤りではない。`-strict` のときだけ exit 1):

| code | 見ること |
|---|---|
| `fill-ratio` | 本文の箱のうち 65% より少なくしか使っていない(テンプレートの方針)。本文を持たない表紙・区切り・主張には出さない |
| `layout-overflow` | 本文のブロックが本文の領域に収まらない。はみ出したブロックごとに1件で、`path` はそのブロック(`/slides/S/body/N`)、文にはみ出した高さ(pt)を書く(下の節)。区切りの題・主張の1文がページに収まらないときも出し、`path` はそのスライド(`/slides/S`)。並列のまとまりは、まとまりの中のはみ出したブロック(`/slides/S/body/N/columns/C/body/K`) |
| `subsystem-min` | サブシステムに割り当てた大きさが、自己申告の最小を下回った(サブシステムの警告の path は、そのブロック `/slides/S/body/N`) |
| `annotation-overlap` | Gantt の注釈に空いた置き場所が無く、ほかの注釈に重ねて置いた |
| `gantt-segment-label` | Gantt の区間の文が、帯が狭すぎて置けない(描かなかった)。path はその区間(`…/data/groups/G/rows/R/segments/S`)。折り返して収まる文は警告にならない(`docs/subsystem.md` の11節) |
| `shapes-overlap` | shapes の図形どうしが重なった。path は後から置いた図形(`…/data/items/N`) |
| `shapes-text-overflow` | shapes の図形の文が、割り当てられた大きさに収まらない。path はその図形 |
| `shapes-connector-angle` | shapes のコネクタが、どの辺の組でも辺と 30°未満の浅い角度でしか繋げない(角で終わったように見える)。path はそのコネクタ |
| `figure-text-overflow` | process・orgchart・matrix・funnel の箱・セル・注記の文が収まらない(折り返せない語が箱より広い・高さが足りない)。path はその要素(`…/data/steps/2` など) |
| `funnel-increase` | funnel の数の値が前の段より増えた。path はその段の `value` |
| `chart-labels` | chart の項目の名前が、アプリが置く場所(縦の形は1項目の幅、横棒は枠の幅の30%)に2行で収まらない(アプリが斜めにする・間引く)。path は最初の項目(`…/data/categories/N`)、文に残りの数 |
| `theme-role` | テーマに値の無い役割名をサブシステムが使った |

### はみ出し(layout-overflow)の測り方

レイアウトエンジンは、足りないとき子を比例で縮めて必ず箱の中に収める。そのため
並べた後の座標でははみ出しが見えず、文字が重なったり詰まったりするだけになる。
`layout-overflow` は「縮めなければ要る高さ」を測り直して比べる(`boxloom/overflow.go`)。

- **本文の領域** = ページの内側の高さ − 題とフッターが要る高さ − その間の gap。
- **ブロックが要る高さ** = 並べた幅で文字を折り返した高さ。サブシステムは自己申告の
  最小の高さ(`Min`)まで縮めてよいので、`Min` で測る(自然な高さより縮んでも、
  `Min` が収まればはみ出しにしない)。
- 表は縮めない(行の高さはセルの文を折り返した高さ)。画像は縦横比を保って
  `imageMinHeight`(値の表)まで縮めてよいので、その高さで測る。
- **並列のまとまり**は、まとまりごとに題と中のブロックを積んで測り、一番高いまとまりが
  ブロックの要る高さになる。はみ出しの警告は、まとまりの中で下端を越えたブロックごとに
  1件で、path は `/slides/S/body/N/columns/C/body/K`(例
  `examples/ir/check-warnings/columns-overflow.json`)。
- ブロックを上から gap を挟んで積み、領域の下端を越えたブロックごとに1件出す。
  越えた高さは、下端をまたぐブロックなら下端より下の分、丸ごと下に落ちたブロックなら
  要る高さ全部。0.5pt 未満は丸めの誤差として見ない。

```json
{"file": "layout-overflow.bx.md", "line": 38, "path": "/slides/1/body/2", "code": "layout-overflow",
 "message": "block (points) overflows the body area by 75.1pt (block needs 180.3pt; body area 414.1pt, all blocks need 518.7pt)"}
```

文字の高さはレイアウトエンジン自身の測り方(字体の幅の表と行の高さの倍率)で
測る。PowerPoint や LibreOffice が実際に描く行の高さとは少し違うので、数 pt の
はみ出しは、描いた絵では目立たないことがある。直し方は作者の側(ブロックを減らす・
スライドを分ける)で、テンプレートは勝手に縮めたり次のページへ送ったりしない。

区切り・主張(本文を持たないスライド)は、題・1文とその上下の余白の箱を同じ測り方で
積み、ページの内側(主張はフッターと gap を引いた残り)を越えたら
`statement overflows the slide by …pt` の形で1件出す(`examples/ir/check-warnings/statement-overflow.json`)。

例:`examples/ir/check-warnings/layout-overflow.json`(と同じ中身の
`examples/md/warnings/layout-overflow.bx.md`)は誤り0件で、2つのブロックに
`layout-overflow` が出る(`boxloom/overflow_test.go`)。

組み立ての警告は、意味の誤りが1件も無いときだけ出す(壊れた IR を組むと、
的外れな警告が混ざるため)。新しい種類の警告(`[XXX WARNING]` の形)は、
そのまま code `xxx` になる。

例:`examples/ir/check-invalid/` は Schema では通り check で断られるもの
(`validate_ir.py --self-test` は Schema で通ることを、`boxloom/check_test.go` は
決まった code で断られることを確かめる)。`examples/ir/invalid/` は Schema でも
check でも断られるもの。

### 容量の計測(capacity)

はみ出す前に「あとどれだけ入るか」を数字で知るために、`check -format json` の
結果には `capacity` が付く(`boxloom/capacity.go`)。本文を持つスライドごとに1件
(表紙・区切り・主張は本文を持たないので出ない)。誤りがあって組み立てまで通らなかったファイルの分は入らない。
`-format text` でも `-capacity` を付ければ同じ数字を1行ずつ出す。

測り方は上の `layout-overflow` と**同じ1回の測り値**から作るので、数字は警告と
食い違わない(警告の pt は、ここの値と同じく 0.1pt に丸めたもの)。高さはすべて pt。

```
"capacity": [{
  "file": "deck.bx.md", "line": 37,          // .md なら Markdown の行(IR なら省く)
  "path": "/slides/1",
  "bodyArea": 414.1,      // 本文の領域(ページの内側 − 題・フッター − gap)
  "need": 210.2,          // ブロックを縮めずに積んだ高さ(ブロックの間の gap を含む)
  "remaining": 203.9,     // bodyArea − need。負ならはみ出している
  "blockGap": 16,         // ブロックの間の gap。新しいブロックを足すなら先にこれが要る
  "bodyLineHeight": 18.9, // 本文の字の大きさの1行の高さ
  "remainingBodyLines": 10, // 残りに本文の行がいくつ入るか(既にあるブロックに足す場合)
  "blocks": [{
    "path": "/slides/1/body/0", "line": 40, "type": "points",
    "need": 68.7,         // そのブロックが縮めずに要る高さ
    "lines": 3,           // 中の文字の行の数(折り返し後。横に並ぶものは一番多い列)
    "top": 0, "bottom": 68.7, // 本文の領域の上端からの位置
    "overflow": 0         // 本文の領域の下端を越えた高さ(layout-overflow の pt と同じ)
  }]
}]
```

- **並列のまとまり**のブロックには `columns` が付く(まとまりごとに `path`
  (`…/columns/C`)・`need`・`lines`・`top`・`bottom`・`overflow` と、中のブロックの
  `blocks`(path は `…/columns/C/body/K`))。ブロックの `need`・`lines` は一番高い
  まとまりのもの。
- 表の `lines` は行ごとの一番多いセルの行の数の和(と説明の文の行)、画像は説明の文の行。
- **使い方の目安**:箇条を1項目足せるかは `remainingBodyLines`(1項目が1行なら
  その数まで)。新しいブロックを足すなら `remaining − blockGap` に入るか。
  `remaining` が負なら、`overflow` が 0 でない最初のブロックの前でスライドを
  分ける(エンジンは自動で分けない)。
- サブシステムは文字の行を数えない(`lines` は 0)。`need` は自己申告の最小の高さ。
- 字の行の数は、測った字体の幅で折り返したもの(`docs/fonts.md`)。

## まだ決めていないこと

- **`$id` のドメイン**:いまは `https://boxloom.invalid/…`(使えないことが
  分かる仮の名前)。OSS で公開するときに決める。
- **Schema の検査と意味の検査を1つのコマンドにするか**:いまは2つ
  (`tools/validate_ir.py` が形、`go run ./cmd/boxloom check` が意味)。Go は依存ゼロの方針で
  JSON Schema の検査器を持たないので、形の誤りは Go の IR の読み込みが代わりに
  断る(場所は分かるが、文は Schema より粗い。Gantt の中の知らないキーは
  `data` までしか指さない)。
- **コード片・図の引用**など、お題3枚に無いブロック(表・画像は #21・#22 で入った)。
- **スライドの骨組みの枠**(title / body / aside …)をコンテンツから指すか。いまの
  IR は `body` の1つだけ。
- **デモの Gantt の再現**:注釈の置き場所は Gantt サブシステムが決める
  ようになった(`docs/subsystem.md` の6節)。デモの Gantt 全体を IR で書いた例が
  `examples/ir/gantt-full.json`。置き方は自動なので、手で決めたデモと全く同じ
  見た目ではない(注釈の上下が一部違う)。
