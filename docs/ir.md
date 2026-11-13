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
| `schema/subsystems/process.schema.json`・`orgchart`・`matrix`・`funnel`・`phases`・`cycle` | 箱と線の図・循環の図のサブシステム(`docs/subsystem.md` の11節)のブロック |
| `schema/subsystems/chart.schema.json` | 数値のグラフ(`docs/subsystem.md` の12節)のブロック。比べ方 `compare`・項目・系列・単位・強調だけを持つ |
| `schema/boxloom-ir.schema.json` | 検査の入口。コアに同梱のサブシステムを差し込んだもの |
| `examples/ir/demo-deck.json` | お題3枚を IR で書いた例 |
| `examples/ir/gantt-full.json` | デモの Gantt 全体(3グループ・9行・注釈4つ)を IR で書いた例 |
| `examples/ir/gantt-leader.json` | 注釈の引き出し線が目盛り・節目の文字を通る日付に注釈を置いた Gantt(`examples/md/gantt-leader.bx.md` の IR。`docs/subsystem.md` の6節) |
| `examples/ir/shapes.json` | shapes で描いた架空のシステム構成図の例 |
| `examples/ir/text-patterns.json` | 文字のパターン(区切り・主張・結論・統計強調・引用)の例(`examples/md/text-patterns.bx.md` の IR) |
| `examples/ir/figures.json` | process・orgchart・matrix・funnel の例(`examples/md/figures.bx.md` と同じ IR) |
| `examples/ir/phases.json` | phases の例。3段・群2つ・箱7つ(`examples/md/phases.bx.md` と同じ IR) |
| `examples/ir/cycle.json` | cycle の例(単独と columns の中。`examples/md/cycle.bx.md` と同じ IR) |
| `examples/ir/blocks.json` | 並列のまとまり・表・画像・画像の並びの例(画像は `examples/ir/images/`。`tools/gen_example_images.go` で作った架空の図) |
| `examples/ir/charts.json` | chart の例。比べ方の5分類ごと(`examples/md/charts.bx.md` と同じ IR) |
| `examples/yaml/charts.yaml` | `examples/ir/charts.json` を YAML の deck(IR の写し。`docs/content-syntax.md` の1章)で書いたもの。同じ IR・同じ PPTX になる |
| `examples/ir/invalid/` | 断られるべき例(色・位置・見た目の装飾などを1か所だけ足したもの) |
| `tools/validate_ir.py` | 検査のコマンド |

```sh
pip install jsonschema                         # 4.18 以降
python3 tools/validate_ir.py deck.json         # 誤りを JSON で出す(正しければ exit 0)
python3 tools/validate_ir.py --self-test       # 例が通り、断られるべきものが断られるか
```

誤りは `docs/content-syntax.md` の5章の形で返す(Go の `boxloom check`・Markdown の
パーサーと同じ形)。`path` は IR の中の場所(JSON Pointer)で、AI に渡せばその場所を
直せる。IR を直接検査するので `line`・`column` は無い。YAML の deck を Schema で検査するときは、
`boxloom parse deck.yaml > deck.json` で JSON の IR にしてから渡す(`validate_ir.py` は YAML を読まない)。`code` はいつも `schema` で、
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
meta   { title, subtitle?, author?, date?, confidential?, disclaimer?, toc?, coverImage?, closing? }   日付・機密区分・免責は文書に1回。toc: true で表紙の直後に目次、closing: true で最後に終わりのスライド(どちらも組み立てで作る)。coverImage は表紙(と終わりのスライド)に敷く写真
スライド  { type: "cover",     variant?, title?, subtitle?, author?, notes? }
         { type: "content",   variant?, title, lead?, marker?, body: [ブロック], source?, notes? }  題は短い見出し、言いたいことの文は lead、marker は隅の印(2字まで)
         { type: "section",   variant?, title, level?, notes? }  章の区切り(本文なし。level は 1 章・2 節・3 項目、既定 1)
         { type: "statement", variant?, content, source?, notes? }        言うことが1文だけ(本文なし)
ブロック  paragraph | points | compare | note | takeaway | stat | quote | columns | table | image | gallery | subsystem | pairs   (どれも type で選ぶ。pairs は標準でない型)
         takeaway { content }                                     本文の中の結論の1文
         stat     { items: [{ value: "3 → 12", unit?: "機種", label }] }   1〜4個
         quote    { content, source }                             出どころは必須
並列     { type: "columns", flow?: true, columns: [{ title?, role?: "main", body: [paragraph|points|note|table|image] }] }  (2〜3個)
対の並び  { type: "pairs", headings: { subject, response }, pairs: [{ subject: 側, response: 側 }] }  (2組以上。側は paragraph|points|quote か { title?, body: [paragraph|points|quote|image|table 1〜4個] })
表       { type: "table", header: [セル], rows: [[セル]], total?: [セル], caption? }   (セルはインライン。どの行も見出しと同じ数。total は合計の行)
画像     { type: "image", src, alt, caption? }   /  { type: "gallery", images: [{ src, alt, caption? }] }  (2〜6枚)
箇条     { type: "points", ordered?: true, items: [インライン | { content, items?, ordered? }] }  (ordered は番号つき。入れ子の項目の ordered はその items に効く)
インライン "文字列"  か  ["文字列", {type: em|code|link|br, …}, …]
サブシステム { type: "subsystem", subsystem: "gantt" | "shapes" | "process" | "orgchart" | "matrix" | "funnel" | "phases" | "cycle" | "chart", variant?, data: {…} }
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

番号つきの箇条(手順・読む順のように、順序に意味があるもの)は `ordered: true`
(既定は false)。番号は PowerPoint が振り(PPTX の `a:buAutoNum`)、番号の形は
テンプレートの値の表の `orderedStyle`(`arabicPeriod` 1. ・`romanLcPeriod` i. ・
`circleNumDbPlain` ①)が決める。入れ子の箇条は項目の object の `ordered` で、親とは
独立に番号つきにできる(入れ子の段の番号の形は 1. に決めてある。`ordered` には
`items` が要る)。番号の分の幅は箇条の字下げ(`pointIndentEm`)で、check の容量も
同じ幅で測る。

```json
{ "type": "points", "ordered": true,
  "items": ["票を書く", {"content": "係が確かめる", "ordered": true, "items": ["傷を見る", "台帳に写す"]}] }
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

標準でない型(型の語彙。`docs/layers.md` の8.2)を足すときは、
`schema/types/<名前>.schema.json` を型ごとのファイルで書き、コアの `$defs.block` の
`enum` と `allOf` に1つずつ足す(型の語彙はテンプレートに依らず1つなので、コアに足して
よい)。Go は語彙の `typeDecl.Schema` に、その写しを `typeSchema` で読ませる。

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
| `Tick.MainLabel`(`"2029/12"`) | 持たない。`at` から作る | 日付の書式はテンプレートが決める(値の表の `dateFormat`) |
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
  数と単位はそれぞれ途中で折らない(折れるのは数と単位の間だけ)。1行に入らないときは `stat-value` の警告(下の表)。
- **`quote` の `source` は必須。** 出どころの無い他人の言葉は R11 の悪い例になるため。
  引用符は IR に書かない(付けるか・何を付けるかはテンプレートの印 `marks.quote`)。
- Schema で書けない「見える文字があるか」は `boxloom check` の `empty-text`(下の表)。

### 9.1 区切りの階層と目次(#86)

- **区切りの `level`**(1〜3、既定 1)は意味だけ:1 章・2 節・3 節の中の項目。見た目は
  テンプレートの骨組みの `section` が決める(標準は階層によらず同じ。report は 1・2 を
  frame、3 を agenda)。親の無い階層は Schema で書けないので `boxloom check` の `section-level`。
- **目次は IR に書かない。** `meta.toc: true` だけを書き、組み立てで表紙の直後に1枚足す。
  頁番号は組み立てたあとに決まるもので、作者が書いて合わせるものではないため。目次に
  載せるのは level 1・2(項目まで並べると目次が本文の代わりになる)。
- 目次を足すと頁番号は IR の番号とずれる。フッター・目次の番号は頁の番号、`boxloom check`
  の場所(`/slides/N`)と容量は IR の番号(作者が直す場所)。目次のスライドの警告は場所
  `/meta/toc`。

### 9.2 スライドの隅の印とパンくず(#93)

- **`marker`** は本文のスライドだけの欄で、空白を含まない1〜2字の文(`1`・`2`・`A`・`B`)。
  詳細のページで、いまどのパターン・どの要因の話かを示す意味。字の数・空白は Schema
  (`minLength`・`maxLength`・`pattern`)と読み込みの両方で断る。区切り・主張・表紙に
  書けば知らない欄で誤り。
- **パンくずは IR に書かない。** そのスライドより前で最も近い区切りの題を、テンプレートが
  使う(区切りは浅い階層が来ると深い階層を閉じるので、最も近い区切りがいま開いている
  いちばん深い level の区切り)。区切りの題を書き直せばパンくずも付いてくるようにするため。
- 見た目はテンプレートの骨組みの `contentHeader` が決める。標準の `contentHeader` は描かない
  (dark・light は印を書いても出力が変わらない。check は情報も出さない:level と同じく、
  意味をどう見せるか・見せないかはテンプレートの方針で、作者の直すことではないため)。
  report は右上の隅に三角の地(幅・高さ 48pt、役割 `support`)と、その中の丸(地の色 `bg`)に
  印を、左にパンくず(`small`・`text_muted`)を、ページの上の余白の中に描く。パンくずは印の
  あるスライドにだけ描き、前に区切りが無ければ印だけ。

### 9.3 表紙の写真と終わりのスライド(#92)

- **`meta.coverImage`** は表紙に敷く写真のパス。決まりは画像の `src`(`imageSrc`)と同じで、
  ファイルは `Assets` から読み、`boxloom check` が `image-path`・`image-missing`・`image-format`
  を場所 `/meta/coverImage` で出す。飾りの写真なので代わりの文(`alt`)は持たない
  (中身を伝える画像は本文の `image` に書く)。
- 写真はスライドの地(`p:bg` の `a:blipFill`)として全面に敷く。縦横比がページと違えば、
  長い方の両端を同じだけ切る(`a:srcRect`。引き伸ばさない)。PowerPoint では背景の書式から
  差し替えられる。どう重ねるかはテンプレートの骨組みの `cover`・`closing`。
- **終わりのスライドは IR に書かない。** `meta.closing: true` だけを書き、組み立てで最後に
  1枚足す(目次と同じく `deckPages`)。中身を持たない頁なので、作者が書くものは無い。
  警告の場所は `/meta/closing`、`boxloom layout` の場所も `/meta/closing`(type `closing`)。

### 10. 並列のまとまり・表・画像(#20・#21・#22)

- **並列のまとまり**の幅の比は数で持たない。主のまとまりに `role: "main"` を付ける
  (意味)と、テンプレートが広く取る。2個以上の `main` は Schema(`maxContains`)と
  Go の読み込みが断る。中のブロックの種類は `columnBlock` に限る(`columns` の入れ子は断る)。
- **並列のまとまりの流れ**(#84)は `flow`(真偽、既定 `false`)。左のまとまりが右の
  まとまりにつながる(前提→結論、発見→対応)という意味だけを持ち、印の形(矢印・丸)・
  色・大きさは書けない。標準の組み方はまとまりの間の隙間の縦の真ん中に基調色の小さい
  矢印を置き、report は列の間の縦の罫線の真ん中に直径 24pt の丸と山形を置く
  (`flow` の無い文書の dark・light の出力は変わらない)。
- **表**は見出しの行(`header`)を必須にした(GFM の表がいつも持つため)。列の配置・幅は
  持たない。行のセルの数が見出しと違うのは Schema では書けないので `boxloom check` の
  `table-shape`。
- **画像**の `src` は文書のファイルからの相対パス(区切りは `/`)。URL・絶対パス・
  文書のフォルダの外(`..`)・`\` は Schema の `pattern` と `boxloom check` の
  `image-path` が断る。IR に画像の中身(base64 など)は持たない(意味の木を軽く保ち、
  差分で読めるように)。代わりの文 `alt` は必須で、空白だけも不可。
  `caption` は画像の下に出す説明の文(インライン)。

### 10.1 対の並び(pairs。最初の標準でない型、#65)

「分かったこと ↔ 打ち手」のように、2つの側の見出しを1回だけ書き、その下に対を並べる。
対の2つの側は行がそろう(`columns` では2つのまとまりの行がそろわない)。標準の型には
しない(`docs/layers.md` の8.1)。Schema は `schema/types/pairs.schema.json`(コアの
`block` の振り分けがそこを指す)。

```json
{ "type": "pairs",
  "headings": { "subject": "分かったこと", "response": "打ち手" },
  "pairs": [
    { "subject":  { "type": "paragraph", "content": "雨の日は箱の口から本が濡れる" },
      "response": { "type": "points", "items": ["口に庇を付ける", "濡れた本は乾かしてから戻す"] } },
    … ] }
```

- **欄の名前は意味で付ける**(位置の名前にしない、#1279989)。`subject` はその対が
  何についてか(分かったこと・課題・問い)、`response` はそれに応えるもの(打ち手・
  対策・答え・確かめ方)。どちらを左に置くか・幅・色は書けない(テンプレートが決める。
  dark / light は subject を左、2つの側を同じ幅に)。`left`・`right` のような位置の名前は
  知らないキーとして断る(`examples/ir/invalid/pairs-left-right.json`)。
- `headings` は2つの側の見出し(列の題。インライン)で、文書の中で1回だけ書く。両方必須。
- `pairs` は2組以上。対の2つの側はそれぞれ段落・箇条・引用(`paragraph`・`points`・
  `quote`)のどれか1つのブロックか、**見出しと複数のブロック**
  `{ "title": 見出し(任意。インライン), "body": [ブロック 1〜4個] }`(#87)。body に置けるのは
  段落・箇条・引用・画像・表(続きの表 `continues` は不可)。1つのブロックの形で表を置いたり、
  body に注記・図などを置いたりはできない(`examples/ir/invalid/pairs-side-table.json`)。
  1組なら段落か `columns` にする。例は `examples/ir/pairs-side-body.json`。

```json
{ "subject":  { "title": "家賃と広さが合わない",
                "body": [{ "type": "points", "items": ["空き店舗の多くは 80 平米を超える", "…"] }] },
  "response": { "body": [{ "type": "points", "items": ["区切って貸せば借りたいという声が多い"] },
                         { "type": "quote", "content": "半分の広さなら借りたい", "source": "店主 A" }] } }
```

- 側の見出し(`title`)と body のブロックも本文のブロックと同じ意味の検査を通る
  (`…/pairs/K/subject/title`・`…/pairs/K/subject/body/I/content` など)。
- Schema と同じ決まりを Go の読み込みも見る(型の語彙の `Validate`)。誤りの path は
  欄まで指す(`/slides/S/body/N/headings/left`・`…/pairs/K/response`)。
- **組み方を持たないテンプレート**(値の表だけの外のテンプレート、例
  `examples/templates/forest.json`)は、代わりの組み方で `columns` に書き換えて組み、
  警告 `component-fallback` を1件出す(path はそのブロック)。見出しはまとまりの題に、
  各対の側はそれぞれのまとまりに順に入れる。まとまりには引用を置けないので、引用は
  言葉の段落と出どころの注(`note`)に分ける。側の見出し(`title`)はまとまりの中の強調の
  段落にし、body のブロックはその後に順に入れる。書き換えた IR の例は
  `examples/ir/pairs-fallback.json`(`examples/ir/pairs.json` を書き換えたもの。Schema と
  check を通る)。
- **はみ出しは対ごとに測る**(下の「はみ出しの測り方」と容量の `pairs`)。
- **側の中のブロックは縮めない**(#101):本文が足りなくても、対の行と側の中のブロック(表・引用など)は
  要る高さのまま積み、はみ出しは `layout-overflow` で知らせる。先に縮むのは画像など縮められる
  サブシステムの分だけ(`nestedSlack`)。report の引用の人の印も縮めず、出どころの文と重ねない。

### 11. 根拠の置き場所:出典・日付・機密区分・免責・合計の行(#47)

資料の根拠(どこから来た数か・いつの時点か・誰に見せてよいか)を、注記(`note`)に
混ぜずに意味として持つ。

- **題とリード文(`lead`)。** 本文のスライドの `title` は何の話かを短く名指す見出しで、
  1行に収める(折り返すと警告 `title-wrap`)。このスライドで言いたいこと(結論)の文は
  `lead`(インライン、1〜2行)に書く。日本の資料の「タイトル+リード文」の型で、見出しと
  主張を分けると、題は1行で揃い、主張は文として読める。`lead` も本文の読む流れの外の
  欄で、テンプレートの骨組みが題の直下に置く(dark / light は題の下に `leadText` の字で、
  間は `headerGap`)。本文の領域(容量・はみ出し)はその高さと gap を引いて測る。
  本文のスライドだけが持つ(区切りの題・主張の1文は、それ自体が言いたいことなので持たない)。
- **出典はスライドの `source`(インライン)。** 本文のブロックではなくスライドの欄にした。
  出典は本文の読む流れに入らず、どのブロックの後ろに置くかに意味が無いため(置き場所は
  テンプレートの骨組みが決める。dark / light は本文の下・フッターの上に、頭に
  `marks.slideSource` を付けた小さな控えめな字)。本文のスライドと主張のスライドが持てる。
  表紙と区切りは持たない(Schema が断る)。1枚に1つで、複数の出どころは1つの文に書く
  (改行は `br`)。他人の言葉の出どころは従来どおり `quote` の `source`、但し書きは `note`。
- **日付・機密区分・免責は `meta`。** `date`(`YYYY-MM-DD` か `YYYY-MM`)・`confidential`
  (機密区分の短い文。例「社外秘」)・`disclaimer`(文書全体の免責・但し書き)。文書に1回だけ
  書き、どこに出すかはテンプレートが決める(dark / light は日付・機密区分・免責を表紙の下に、
  機密区分を本文と主張のスライドのフッターの作者の隣にも)。日付の見せ方(`2026年11月11日`
  など)はテンプレートの値の表の `marks.date`・`marks.yearMonth`(`docs/template-values.md`)。
  スライドごとに「(デモ)」「架空」と注記で繰り返さないための置き場所。
- **合計の行は表の `total`(セルの並び)。** `rows` の最後の行ではなく別の欄にした。
  「この行は合計」は意味で、見た目(太字・上の濃い線)と PPTX の `a:tbl` の `lastRow` は
  テンプレートが決める。セルの数は見出しと同じ(違えば `boxloom check` の `table-shape`、
  path は `…/total`)。合計の値が正しいかは計算しない(書いたとおりに出す)。Markdown では
  表の説明の行の属性 `{total=last}` で書く(`docs/content-syntax.md` の3.7。GFM の表に
  合計の行の書き方が無いのは記法の都合で、IR には持ち込まない)。
- 本文の領域(はみ出し・容量の測り)は、出典の枠の高さと gap を引いたものになる
  (下の「はみ出しの測り方」)。

### 12. 表の行の群(#79)

- **群は表の `groups`(群ごとの `{label, rows}`)。** 先頭の列を群の列とし、群の名前と、
  その群がまとめる本体の行の数だけを持つ。群は上から順に並び、`rows` の和は本体の行の数。
  行ごとに群の名前を書く形(同じ名前の繰り返し)は採らなかった:同じ名前の群が離れて2回
  出るのか、続きなのかを IR の上で区別できず、「この行はこの群に属する」という意味を
  文字の一致に頼ることになるため。
- `groups` があると、`header` の先頭は群の列の見出しで、本体の各行(`rows`)のセルは
  `header` より1つ少ない(群の列を除く。群の名前を空のセルで繰り返さない)。合計の行
  (`total`)は群に入らず、セルの数は `header` と同じ(先頭のセルは「合計」など)。
- 結合したセル・太字・群の境目の線は見た目なのでテンプレートが決める。dark / light は
  先頭の列を本物の `a:tbl` の縦の結合(始まりのセルが `rowSpan`、続きが `vMerge`)にし、
  群の名前を見出しと同じ太字にする。結合の中の行の境目には群の列だけ線を引かない。
- **強調は持たない。** 群そのものを推す意味は、いまの表の語彙(行・合計)に無い。群の名前
  を強めたいときは、文中の強調(`em`)を名前に書く(ほかのセルと同じ)。推す項目を1つ選ぶ
  意味(matrix・chart の `emphasis`)が表に要るようになったら、群ではなく行・列の名前で
  指す形にそろえる。
- 誤りは `boxloom check` の `table-groups`:群の名前が空(path は `…/groups/G/label`)、
  `rows` の和が本体の行の数と違う、群の列のほかに列が無い(どれも path は `…/groups`)。
  群があるときの行のセルの数の違いは従来どおり `table-shape`。`rows` が 1 未満は Schema。
- はみ出しと容量は行ごとに測る(群の境目は行の境目)。群の名前が結合した行の高さに
  収まらなければ、群の最後の行を伸ばす。

### 13. 表のセルの中の箇条(#80)

- **本体・合計の行のセルは、インラインか `{"points": [...]}`。** `points` は項目
  (インライン)の並びで2つ以上。項目が1つならただのインラインで書く(同じ意味を2つの形で
  書けないように)。見出しの行のセルはインラインだけ。

  ```json
  {"type": "table",
   "header": ["課題", "見直しの案"],
   "rows": [["承認が遅い", {"points": ["100 万円未満は2段", "承認の期限を3日に"]}]]}
  ```

- 列ごとに「この列は箇条」と宣言する欄は IR に持たない:箇条かどうかはセルの意味で、
  同じ列に箇条のセルと文のセルが混ざってよい(項目が1つのセルは文)。列の宣言
  (`{points=列名}`)は Markdown の記法の都合で、`;` を区切りと読む範囲を決めるだけ
  (`docs/content-syntax.md` の3.7)。
- 印・字下げは見た目なのでテンプレートが決める。dark / light は本文の箇条と同じ印
  (`marks.point`)を、本物の `a:tbl` のセルの段落の箇条の印(`a:buChar`)で書く。項目ごとに
  1つの段落で、折り返しはアプリに任せる。箇条のセルは数の列として右に揃えない。
- はみ出しと容量は、セルの中の箇条の行(項目ごとに折り返した行の和)で行の高さを測る。
- 誤りは `boxloom check` の `table-points`:項目が1つ以下(path は `…/rows/R/C/points`)、
  空の項目(path は `…/points/I`)。

### 14. 長い表を次のスライドへ続ける(continues。#81)

- **Boxloom は表を自動で次のスライドへ送らない。** IR の1スライド = PPTX の1枚を保つ
  (はみ出しを自動で直さない、と同じ方針)。長い表は**作者が分け**、続きのスライドの
  表に `"continues": true` を書く。続きの表は `header` を書かない(書くと誤り)。

  ```json
  {"type": "content", "title": "移す仕組みの一覧",
   "body": [{"type": "table", "header": ["仕組み", "利用者(人)"], "rows": [["受注の管理", "420"]]}]},
  {"type": "content", "title": "移す仕組みの一覧",
   "body": [{"type": "table", "continues": true, "rows": [["請求書の発行", "60"]], "total": ["合計", "480"]}]}
  ```

- **ひと続きの形**:続きの表は、すぐ前のスライドの本文の**最後の表**に続く。間に別の
  スライド(区切りを含む)を挟まない。続きの表はそのスライドの本文の最初の表で、本文の
  ブロック(並列のまとまり `columns` の中ではない)。`continues` は `true` だけを書く
  (続かない表は書かない)。
- **最初の表に従うもの**:見出しの行・列の数・`groups` の有無。続きの表の本体の行は
  最初の表の見出しの行で数える(`groups` があれば1つ少ない)。続きの表の `groups` は
  その枚の行の群を書き直す(群が枚をまたぐなら、続きの枚の最初の群にも名前を書く)。
  セルの中の箇条(`points`)はセルの意味なので、IR では表ごとの宣言を持たない
  (13章。Markdown の `{points=列名}` は最初の表から引き継ぐ)。合計の行(`total`)は
  ひと続きの最後の表だけ。
- **Boxloom がそろえるもの**:
  - 各枚に最初の表の見出しの行を繰り返す。
  - 各枚に「n/m」の印を付ける(n は何枚目か、m はひと続きの枚数)。文はテンプレートの
    `marks.tableContinued`(dark / light / report は `{n}/{m}`)、置き場所は組み方が
    決める(dark / light / report は説明の文の行の右端=表の右端の上)。
  - 列の幅をひと続きの全部の枚でそろえる(全部の枚の行と見出しの行を合わせて測る)。
    数の列の右揃えも全部の行で決める。
  - 説明の文(`caption`)を書かない続きの表は、最初の表の説明の文を繰り返す。
- 誤りは `boxloom check` の `table-continues`(下の表)と、列の数の違いは `table-shape`。
  Go の読み込み(`ParseIR`)も同じ形を断る。
- **分け方の案**:表がはみ出すと、`layout-overflow` の警告の `hint` と容量の
  `blocks[].split` に「何枚に分けられるか・どの行の前で分けるか」を出す(下の
  「容量の計測」)。分けるのは作者(AI)。

### 15. 表の列の幅(#98)

列の幅は IR に書かない(テンプレートが中身から測る)。Boxloom の規則:

- **折り返さない幅**(列の中で一番長い行)の合計が表の幅に入れば、その比で広げる。
- 入らなければ、各列の**守りたい最小の幅**を先に取り、残りを「折り返さない幅と
  守りたい最小の幅の差」の比で配る。守りたい最小の幅は次の大きい方(セルの左右の余白込み):
  - **見出しの文字列の幅**(見出しの行のセルは折らない。「規模感」が1字ずつ縦に折れない)。
  - セルの中の**折れない最長の塊**:欧文・数字の語(英数字と `._%+-` の続き)は丸ごと。
    和文(漢字・平仮名・片仮名・長音「ー」・「々」)の続いた字は **4字**ずつの塊とみなし、
    一番広い4字(続きが4字より短ければその続き全体)。句読点・括弧などの約物の前後では折れてよい。
    「地域中核」のような4字までの語は1行に置ける。
- 守りたい最小の幅の合計も表の幅に入らなければ、従来の割り方(語を割らない最小の幅を
  取り、残りを同じ比で配る。それも入らなければ最小の幅の比で縮める)に戻し、警告
  `table-col-width` を出す(path は表のブロック。文に最初に削られた列の見出しと、要る幅・
  有る幅)。直し方は見出しを短くする・列を減らす・表を縦に分けるなど(作者が決める)。
- 続きの表(14章)はひと続きの全部の枚の行で同じように測る(どの枚も同じ幅)。

## 意味の検査(boxloom check)

JSON Schema では書けない検査は Go の `boxloom check` がする。

```sh
cd boxloom
go run ./cmd/boxloom check ../examples/ir/demo-deck.json            # 誤りが無ければ exit 0
go run ./cmd/boxloom check ../examples/md/demo-deck.bx.md            # Markdown もそのまま検査する
go run ./cmd/boxloom check ../examples/yaml/charts.yaml              # YAML の deck(IR の写し)も
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
- YAML の deck(`.yaml`・`.yml`。IR の写し)を検査したときは、YAML の部分集合の外なら
  code `yaml` の誤り(`line`・`column` つき)を返す。通れば JSON の IR と同じように検査し、
  誤りと警告には IR の場所(`path`。無ければ一番近い親)の YAML の位置を `line`・`column` に
  添える(キーならキーの頭、列の項目なら `-` の後ろ)。容量には `line` だけを添える。

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
| `section-level` | 区切りの `level` に親が無い(level 2 の前に level 1 が無い・level 3 の前に、最後の level 1 の後の level 2 が無い) |
| `date-order` | `from` が `to` より前(同じ日も断る。長さ0の区間は描けない) |
| `date-valid` | 日付が暦の上で実在する(Schema の正規表現は 2028-02-30 を通す) |
| `within-span` | Gantt の区間・リスク・注記・目盛り・節目・注釈・軸を切る日付が、`span` の中にある(外の日付は軸の端に押し込まれて、誤った絵になるため) |
| `template-exists` | `template` の名前のテンプレートがある |
| `variant-exists` | `variant` の名前が、そのテンプレートにある(デモのテンプレートは変種を1つも持たない) |
| `shape-bounds` | shapes の座標・大きさ・線の点が 0〜1 にあり、`x+w`・`y+h` が 1 以下(path は和が越えた `w`・`h`) |
| `role-exists` | shapes の色(`fill`・`line`・`textColor`・`color`)の役割名が、そのテンプレートのテーマにある |
| `connector-self` | shapes のコネクタの `from` と `to` が同じ図形 |
| `empty-text` | 区切りの `title`・本文のスライドの `lead`・主張の `content`・`takeaway` の `content`・`quote` の `content` と `source`・`stat` の各 `label`・スライドの `source`・`meta` の `confidential` と `disclaimer`・`pairs` の2つの見出し(`headings`)と段落の側の `content` に、見える文字がある(Schema は `""` や空白だけを通す)。`pairs` の側の引用・`variant` も本文のブロックと同じに見る(path は IR の場所 `…/pairs/K/subject/…`。テンプレートや代わりの組み方に依らない) |
| `matrix-cells` | matrix の各行の `cells` の数が `columns` の数と同じ(path はその行の `cells`) |
| `matrix-emphasis` | matrix の `emphasis`(推す列)が `columns` の見出しのどれか(文中の記法を除いた文字)と同じ(path は `…/data/emphasis`) |
| `matrix-column-groups` | matrix の `columnGroups`(列の群)の `span` の合計が `columns` の数と同じ(path は `…/data/columnGroups`) |
| `table-shape` | 表の行・合計の行のセルの数が、見出しの行と同じ(path は違う行 `…/rows/R` か `…/total`)。`groups` があると本体の行は見出しより1つ少ない |
| `table-points` | 表のセルの中の箇条(`{"points": [...]}`)の項目が2つ以上で(path は `…/rows/R/C/points` か `…/total/C/points`)、空の項目が無い(path は `…/points/I`) |
| `table-continues` | 続きの表(`continues`)が、すぐ前のスライドの本文の最後の表に続く(前のスライドに表が無い・間にスライドを挟む・そのスライドの2つ目の表・並列のまとまりの中は path `…/continues`)。`header` を書かない(path `…/header`)。`groups` の有無が最初の表と同じ(path `…/groups`)。合計の行はひと続きの最後の表だけ(path は続きのある表の `…/total`)。`continues` は `true` だけ |
| `table-groups` | 表の `groups` の名前が空でなく(path は `…/groups/G/label`)、`rows` の和が本体の行の数で、群の列のほかに列がある(path は `…/groups`) |
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
| `title-wrap` | 本文のスライドの題が、題の幅で2行以上に折り返す。題は短い見出しにして1行に収め、言いたいことの文は `lead` に書く。path は題(`/slides/S/title`、.md なら「# 題」の行)、文に行の数と幅 |
| `fill-ratio` | 本文の箱のうち 65% より少なくしか使っていない(テンプレートの方針)。本文を持たない表紙・区切り・主張には出さない。`hint` に直し方(隣の薄いスライドとまとめる・根拠を足す・主張のスライドにする)を書き、隣り合うスライドも薄く、2枚の本文を1枚にまとめると本文の領域に収まるときは、その相手と見込みの割合も書く(下の「薄いスライドの hint」) |
| `spare-width` | 本文のブロックの中の図・画像が、縦横比(または横の上限)を保ったために、ブロックの幅の 20% 以上を横に空けた。path はそのブロック、文に空いた幅(pt と %)。容量の `remainingWidth` と同じ数字。並列のまとまり(columns)の中のグラフ・循環の図・画像は、まとまりの幅に対して同じく測り、path は `/slides/S/body/N/columns/C/body/K` |
| `stat-value` | `stat` の数と単位が、カードの幅に1行で入らない。数と単位はそれぞれ途中で折らない(「億ド / ル」「12,345, / 678」と折れると読み違えるため)ので、入らなければ単位が次の行に落ち、数そのものが入らなければ1行のままカードからはみ出す。path はその数(`/slides/S/body/N/items/K`、.md なら `:::value` の行)、文に要る幅と有る幅(pt)。字は縮めない |
| `layout-overflow` | 本文のブロックが本文の領域に収まらない。はみ出したブロックごとに1件で、`path` はそのブロック(`/slides/S/body/N`)、文にはみ出した高さ(pt)を書く(下の節)。区切りの題・主張の1文がページに収まらないときも出し、`path` はそのスライド(`/slides/S`)。並列のまとまりは、まとまりの中のはみ出したブロック(`/slides/S/body/N/columns/C/body/K`) |
| `component-fallback` | テンプレートが組み方を持たない標準でない型のブロックを、型の代わりの組み方(標準の型への書き換え)で組んだ。path はそのブロック(`/slides/S/body/N`)。テンプレートを指したとき(`-template`・値の表・IR の `template`)だけ出し、指さなければ型の語彙だけで検査する(`docs/layers.md` の8.2) |
| `subsystem-min` | サブシステムに割り当てた大きさが、自己申告の最小を下回った(サブシステムの警告の path は、そのブロック `/slides/S/body/N`) |
| `annotation-overlap` | Gantt の注釈に空いた置き場所が無く、ほかの注釈に重ねて置いた |
| `milestone-overlap` | Gantt の節目の日付が近すぎて、段を2つにずらしても文字が重なった。path はその節目(`…/data/milestones/N`)(`docs/subsystem.md` の6節「節目の段」) |
| `gantt-segment-label` | Gantt の区間の文が、帯が狭すぎて置けない(描かなかった)。path はその区間(`…/data/groups/G/rows/R/segments/S`)。折り返して収まる文は警告にならない(`docs/subsystem.md` の11節) |
| `shapes-overlap` | shapes の図形どうしが重なった。path は後から置いた図形(`…/data/items/N`) |
| `shapes-text-overflow` | shapes の図形の文が、割り当てられた大きさに収まらない。path はその図形 |
| `shapes-connector-angle` | shapes のコネクタが、どの辺の組でも辺と 30°未満の浅い角度でしか繋げない(角で終わったように見える)。path はそのコネクタ |
| `figure-text-overflow` | process・orgchart・matrix・funnel・phases・cycle の箱・セル・注記の文が収まらない(折り返せない語が箱より広い・高さが足りない)。path はその要素(`…/data/steps/2` など) |
| `funnel-increase` | funnel の数の値が前の段より増えた。path はその段の `value` |
| `figure-legend` | process・orgchart・funnel・phases・cycle で `status` を使ったのに、その状態の文が `legend` に無い(読み手は枠線の色の意味が分からない)。path は `…/data/legend` |
| `chart-labels` | chart の項目の名前が、アプリが置く場所(縦の形は1項目の幅、横棒は枠の幅の30%)に2行で収まらない(アプリが斜めにする・間引く)。path は最初の項目(`…/data/categories/N`)、文に残りの数 |
| `table-col-width` | 表の列の守りたい最小の幅(見出しの文字列・折れない語の幅。15章)の合計が表の幅に入らず、短い列が語の途中で折れうる幅で配った。path は表のブロック(並列のまとまりの中は `…/columns/C/body/K`)、文に最初に削られた列の見出し(`header[C]`)と要る幅・有る幅(pt) |
| `theme-role` | テーマに値の無い役割名をサブシステムが使った |

### はみ出し(layout-overflow)の測り方

レイアウトエンジンは、足りないとき子を比例で縮めて必ず箱の中に収める。そのため
並べた後の座標でははみ出しが見えず、文字が重なったり詰まったりするだけになる。
`layout-overflow` は「縮めなければ要る高さ」を測り直して比べる(`boxloom/overflow.go`)。

- **本文の領域** = ページの内側の高さ − 題・リード文(`lead` があるとき)・出典(`source` があるとき)・フッターが要る高さ
  − その間の gap。
- **ブロックが要る高さ** = 並べた幅で文字を折り返した高さ。サブシステムは自己申告の
  最小の高さ(`Min`)まで縮めてよいので、`Min` で測る(自然な高さより縮んでも、
  `Min` が収まればはみ出しにしない)。
- 表は縮めない(行の高さはセルの文を折り返した高さ)。画像は縦横比を保って
  `imageMinHeight`(値の表)まで縮めてよいので、その高さで測る。
- **並列のまとまり**は、まとまりごとに題と中のブロックを積んで測り、一番高いまとまりが
  ブロックの要る高さになる。はみ出しの警告は、まとまりの中で下端を越えたブロックごとに
  1件で、path は `/slides/S/body/N/columns/C/body/K`(例
  `examples/ir/check-warnings/columns-overflow.json`)。
- **対の並び(pairs)**は、見出しの行の下に対の行を gap を挟んで積み、対の高さは
  2つの側の高い方。はみ出しの警告は下端を越えた対ごとに1件で、path は
  `/slides/S/body/N/pairs/K`、文に対の高さと側ごとの高さを書く(例
  `examples/ir/check-warnings/pairs-overflow.json`)。見出しの行だけが越えたときは
  ブロックの警告になる。対の側の中の警告(折り返しなど)の path は `…/pairs/K/subject`
  (`response`)。
- ブロックを上から gap を挟んで積み、領域の下端を越えたブロックごとに1件出す。
  越えた高さは、下端をまたぐブロックなら下端より下の分、丸ごと下に落ちたブロックなら
  要る高さ全部。0.5pt 未満は丸めの誤差として見ない。

```json
{"file": "layout-overflow.bx.md", "line": 38, "path": "/slides/1/body/2", "code": "layout-overflow",
 "message": "block (points) overflows the body area by 131.5pt (block needs 199.2pt; body area 406.1pt, all blocks need 571.1pt)"}
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
  "file": "deck.bx.md", "line": 37,          // .md なら Markdown の行、.yaml なら YAML の行(JSON の IR なら省く)
  "path": "/slides/1",
  "bodyArea": 406.1,      // 本文の領域(ページの内側 − 題・リード文・出典・フッター − gap)
  "need": 210.2,          // ブロックを縮めずに積んだ高さ(ブロックの間の gap を含む)
  "remaining": 195.9,     // bodyArea − need。負ならはみ出している
  "blockGap": 20,         // ブロックの間の gap。新しいブロックを足すなら先にこれが要る
  "bodyLineHeight": 21.6, // 本文の字の大きさの1行の高さ
  "remainingBodyLines": 10, // 残りに本文の行がいくつ入るか(既にあるブロックに足す場合)
  "blocks": [{
    "path": "/slides/1/body/0", "line": 40, "type": "points",
    "need": 68.7,         // そのブロックが縮めずに要る高さ
    "lines": 3,           // 中の文字の行の数(折り返し後。横に並ぶものは一番多い列)
    "top": 0, "bottom": 68.7, // 本文の領域の上端からの位置
    "overflow": 0         // 本文の領域の下端を越えた高さ(layout-overflow の pt と同じ)
    // "remainingWidth": 190.5 // 図・画像が横に残した幅(図を持ち、余りがあるときだけ)
  }]
}]
```

- **並列のまとまり**のブロックには `columns` が付く(まとまりごとに `path`
  (`…/columns/C`)・`need`・`lines`・`top`・`bottom`・`overflow` と、中のブロックの
  `blocks`(path は `…/columns/C/body/K`))。ブロックの `need`・`lines` は一番高い
  まとまりのもの。
- **横に同じ幅のカードを並べるブロック**(`compare` の選択肢・`stat` の数)には
  `cards` が付く(カードごとに `path`(`…/options/K`、stat は `…/items/K`)・`need`・
  `lines` と、ブロックの高さを決める一番高いカードにだけ `"tallest": true`)。
  ブロックの `need` は一番高いカードの `need` なので、**ほかのカードに文を足しても
  `need` も `fill-ratio` も変わらない**。中身を足して埋まりを変えたいなら、
  `tallest` のカードに足すか、別のブロックとして足す。
- **対の並び**(`pairs`)のブロックには `pairs` が付く(対ごとに `path`(`…/pairs/K`)・
  `need`・`lines`・`top`・`bottom`・`overflow`・側ごとの高さ `subject`・`response` と、
  対の高さを決める側 `tallest`(`"subject"` か `"response"`))。低い方の側に文を足しても
  対の高さは増えない。ブロックの `need` は見出しの行と対の行を gap を挟んで積んだ高さ。
- 表の `lines` は行ごとの一番多いセルの行の数の和(と説明の文の行)、画像は説明の文の行。
- **はみ出した本文の表**には `split` が付く(はみ出さなければ省く)。表を何枚に分けると
  どの枚にも収まるかの案で、`tables`(枚数)・`splitBefore`(2枚目からの表が始まる本体の
  行。IR の `rows` の番号)・`rows`(表ごとの本体の行の数)。最初の枚は表の上端から本文の
  下端まで、続きの枚は本文が表だけで題・出典・フッターは同じと見込み、各枚に説明の文の
  行(「n/m」の印)と見出しの行を数える。`groups` の表は群の境目で分け、1つの群が1枚に
  入らないときだけ群の中で分ける。合計の行は最後の枚。同じ案を `layout-overflow` の警告の
  `hint` にも書く。分け方は作者が選び、続きの表に `continues: true` を書く(14章)。

  ```
  "split": {"tables": 3, "splitBefore": [7, 14], "rows": [7, 7, 4]}
  ```

- **使い方の目安**:箇条を1項目足せるかは `remainingBodyLines`(1項目が1行なら
  その数まで)。新しいブロックを足すなら `remaining − blockGap` に入るか。
  `remaining` が負なら、`overflow` が 0 でない最初のブロックの前でスライドを
  分ける(エンジンは自動で分けない)。
- サブシステムは文字の行を数えない(`lines` は 0)。`need` は自己申告の最小の高さ。
- 字の行の数は、測った字体の幅で折り返したもの(`docs/fonts.md`)。
- 図・画像を持つブロックには、図が左右に使わずに残した幅の合計 `remainingWidth`(pt)が
  付く(無ければ省く。`boxloom/spare.go`)。縦横比を保つ図(画像・shapes)は高さで幅が
  決まり、横の上限(Max)を持つ図は上限で止まるので、ブロックの幅より細く描かれる。
  余りは図の置き方(Place)の側に白く残り、縦の埋まり(`fill-ratio`)では見えない。
  ブロックの幅の 20% 以上なら `spare-width` の警告にもする。埋めるのは作者で、
  `columns` に入れて図の横に文を置くか(画像・グラフ・循環の図はまとまりの中に置ける)、同じ
  スライドの文を減らして図に高さを渡す。円グラフは高さで直径が決まるので、横の上限
  (その高さで要る幅)を申告する。並列のまとまりの中のブロックにも、まとまりの幅に対する
  `remainingWidth` を付ける(`columns[].blocks[]`)。図の地(カード)は図が使う矩形に
  合わせて描き、余りは地の外に残る。

### 薄いスライドの hint

`fill-ratio`(本文が 65% 未満)の警告の `hint` には、作者が選べる直し方を書く。
Boxloom は空きを自動で埋めない(字や図を大きくしない・ブロックを送らない)し、
内容の良し悪しも判定しない。数で測れることだけを添える(`boxloom/hints.go`):

- 前か後ろのスライドも `fill-ratio` で薄く、2枚の本文を1枚に積むと本文の領域に
  収まるとき、その相手のスライドと見込みの割合を書く。見込みは容量の数字から
  `(need + blockGap + 相手の need) ÷ 2枚の bodyArea の小さいほう`(題は1つになる)。
  両隣とも収まるなら、割合の大きい(ちょうど埋まる)ほう。区切り・主張・表紙は
  本文を持たないので相手にならない(章をまたいでまとめることは勧めない)。
- 本文に横に並ぶカード(`compare`・`stat`)・並列のまとまり(`columns`)があるとき、
  ブロックの高さを決めている一番高いカード・まとまりの path と高さを書き、ほかに
  文を足しても本文の高さは増えないことを添える(作者が足す場所を迷わないように)。
- どの場合も、隣とまとめる・根拠(数・出典・図・表)を足す・1つの主張だけなら
  主張のスライド(`type=statement`)にする、のどれかで直すこと、空きを埋めるために
  字や図を大きくしたり言い換えを足したりしないこと、を書く。

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
