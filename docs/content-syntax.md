# コンテンツの記法 — 仕様 第一版(v0.1)

Boxloom の作者が書くコンテンツの記法を定める。候補の比較と理由は
`docs/content-syntax-comparison.md` にある。この文書はその「決定」を受けた仕様である。

> 状態: 第一版。Go 版にパーサーがある(`boxloom/markdown.go`。8章)。要素の名前と
> 範囲は、意味の中間表現(IR)の Schema 第一版(`schema/`・`docs/ir.md`)に揃えてある。
> 食い違ったら IR を正とする。IR にまだ無いものは「予定」と書き、入るまでは誤りとして断る。

## 1. 決定

| 入口 | 位置づけ | 誰が使うか |
|---|---|---|
| **Boxloom Markdown**(`.bx.md`) | **主な入口**。作者が書くのはこれ | 人、AI |
| **IR の写し(YAML / JSON)** | 併存する入口。IR とほぼ1対1 | 他のプログラム、構造化出力で書く AI |
| Go DSL | 作者向けにはしない。テンプレート・プラグイン・試験用 | 開発者 |
| TeX 風 | **採らない** | — |

決め手は次の3つ。

1. **作者体験**: スライドの大半は文章と箇条で、Markdown が最も楽に書け、そのまま
   読める。Gantt のような構造の多いデータだけを YAML の囲みに逃がせる。
2. **AI との相性**: AI が最も自然に書ける記法で、トークンも最も少ない。TeX 風は
   AI が本物の LaTeX の見た目の命令(`\vspace` `\textcolor` など)を持ち込むので採らない。
3. **分離を機械で守れること**: Markdown の弱点(約束事を忘れる・誤りが黙って通る)は、
   次の「厳格な Markdown」の規則と `boxloom check` で塞ぐ。

「TeX 的」は記法の見た目ではなく、**意味だけ書けば組版が済む** という思想のほうで実現する。

## 2. 厳格な Markdown — 弱点の塞ぎ方

ふつうの Markdown は「何を書いても何かとして通る」。Boxloom Markdown はその逆で、
**意味が決まらない書き方は誤りとして断る**。

| 弱点 | 塞ぎ方 |
|---|---|
| 見た目で区別する約束事を忘れる(`+`/`-` で良い点・悪い点) | **約束事は名前で書く**。`:::pros` `:::cons` のように囲みの名前で意味を示す。記号の違いに意味を持たせない |
| 知らない囲み・属性が黙って無視される | 囲みの名前・属性・サブシステムのキーは **閉じた一覧**。知らないものは誤り |
| 生の HTML・style で見た目が漏れ込む | 生の HTML は誤り(`<br>` `<span style=…>` を含む) |
| 別の方言が混ざる(`> [!NOTE]`、脚注、表の配置指定 `:--:` など) | 対応しない構文は無視せず誤りにして、代わりの書き方を示す |
| 色・大きさ・位置の指定 | 属性に `color` `size` `align` `x` などは無い。見た目を変えたいときはテンプレートの **変種**(`variant`)を名前で選ぶだけ |
| 囲みの `:::` の入れ子がずれる | 閉じ忘れ・数の不一致は、開いた行の位置つきで誤り |
| 誤りの場所が分からない | すべての誤りを「ファイル・行・列・IR の中の場所・期待したもの・直し方の候補」で返す |

## 3. 文書の形

```
---                      ← 文書の頭(YAML)。省略可
title: …
---
# スライドの題            ← レベル1の見出しが1枚の始まり
本文…
```

### 3.1 文書の頭(front matter)

YAML で書く。使えるキー:

| キー | 意味 | 必須 |
|---|---|---|
| `title` | 文書の題(表紙に使う) | ○(文書の頭を書くなら) |
| `subtitle` | 副題 | |
| `author` | 作者 | |
| `date` | 日付(`YYYY-MM-DD` の文字列として扱う) | |
| `template` | テンプレートの名前 | |
| `cover` | 表紙を出すか(既定 `true`) | |

表紙は文書の頭から作る。表紙のためにスライドを書く必要はない。
IR では、文書の頭は `meta` になり、`cover` が `true` なら先頭に `{type: "cover"}` の
スライドが置かれる。`template` は IR の根の `template` になる。文書の頭を書かなければ
`meta` も表紙も無い。`title`・`subtitle`・`author` の値は本文と同じ文中の記法
(`**…**` など)で読む。

### 3.2 スライド

- `# 題` が1枚の始まり。次の `#` か文書の終わりまでがそのスライド。
- `#` の行の末尾に属性を `{…}` で付けられる: `# 論点 {variant=dense}`。
  使える属性は `type`(スライドの種類)と `variant`(テンプレートが名乗る名前)。
  スライドの `id` は予定(IR 第一版に無い)。
- **スライドの種類は `type` で選ぶ**(省けば `content`)。`#` の行の文の意味は種類で決まる:

  | `type` | `#` の行の文 | 後ろに書けるもの | IR |
  |---|---|---|---|
  | `content`(既定) | 題(結論の文) | 本文・`:::notes` | `{type: "content", title, body}` |
  | `section` | 章の題 | `:::notes` だけ | `{type: "section", title}` |
  | `statement` | 言いたいことの1文 | `:::notes` だけ | `{type: "statement", content}` |

  ```
  # 第2部 移行の進め方 {type=section}

  # 移行は 2 年早める必要がある {type=statement}

  :::notes
  根拠は第1部の数字。ここでは1文だけを言い切る。
  :::
  ```

  - `section`・`statement` の後ろに本文(段落・箇条・囲み・コードブロック)を書くと
    `misplaced-content`。区切りに目次を足す・主張に理由の箇条を足すのは、
    `docs/patterns.md` の R1・R2 の悪い例なので、記法で止める。
  - `type=cover` は誤り(表紙は文書の頭から作る。3.1)。知らない種類は `invalid-value`
    で、選べる名前を `expected` に入れる。
  - 理由:種類を `#` の行の属性にしたのは、どの種類も「1枚の始まり」と「1つの文」を
    持ち、違うのはその文の意味だけだから。`:::section` のような囲みにすると、スライドの
    境目(`#`)と中身の境目が2重になる。`# ---` のような記号の約束事は使わない
    (2章「約束事は名前で書く」)。
- 1枚目の `#` より前に本文を書くのは誤り(どのスライドのものか決まらない)。
- `---`(水平線)はスライドの区切りではない。誤りにして `#` を使うよう示す。

### 3.3 本文で使えるもの

| 書き方 | 意味 | IR |
|---|---|---|
| 段落 | 文 | `paragraph` |
| `- ` / `1. ` | 箇条(入れ子は2段まで) | `points`(入れ子は `{content, items}`) |
| `**…**` | 強調。**太字ではなく「強調」という意味**。どう見せるかはテンプレート | インラインの `em` |
| `` `…` `` | コード・識別子 | インラインの `code` |
| `[文](URL)` | リンク | インラインの `link` |
| 行末の `\` | 意味のある改行 | インラインの `br` |
| 表(GFM の表) | 行×列の文字・数(3.7) | `table` |
| `![代わりの文](path)` だけの段落 | 画像1枚。2〜6枚を並べると画像の並び(3.8) | `image` / `gallery` |

段落の中の改行(行末に `\` の無いもの)は、ふつうの Markdown と同じく改行にならず
次の行とつながる。つなぐとき、**つなぎ目の両側が英数字のときだけ空白を1つ入れ**、
どちらかが日本語なら空白を入れない(ふつうの Markdown は常に空白を入れるので、
日本語の文の途中に空白が入ってしまう)。

`\` の後ろの記号はただの文字になる(`\*` は `*`)。`_` と `&` には意味を持たせない
(ただの文字)。

`*…*`(斜体)・`~~…~~`(取り消し線)・脚注・生の HTML は使えない。強調は1種類に絞る。
意味の違う強調が要るなら、テンプレートが名乗る囲みを増やす(3.4)。

### 3.4 意味ブロック(囲み)

`:::名前` で開き `:::` で閉じる。中に別の囲みを入れるときは、**外側の囲みのコロンを
1つ増やす**(`::::compare` の中に `:::option`)。閉じも同じ数のコロンで書く。
名前の後ろに見出しの文、その後に属性を書ける:

```
:::名前 見出しの文 {属性=値}
中身(Markdown)
:::
```

第一版の囲み(既定のテンプレートが名乗るもの):

| 囲み | 中に書けるもの | 意味 | IR |
|---|---|---|---|
| `compare` | `option` だけ(2つ以上) | 選択肢の比較 | `compare` |
| `option 名前` | `pros`・`cons` だけ | 比較の1つの選択肢 | `compare.options[]`(`label` が名前) |
| `pros` / `cons` | 箇条 | 良い点 / 悪い点 | `pros` / `cons` |
| `note` | 本文 | 注記・但し書き | `note` |
| `takeaway` | 段落1つ | 本文の中で言いたいことを1文で目立たせる(結論) | `takeaway` |
| `stat` | `value` だけ(1〜4個) | 数を強く見せる(統計強調) | `stat` |
| `value 数` | 段落1つ(数の説明) | `stat` の1つの数。見出しの文が数、属性 `unit` が単位 | `stat.items[]`(`value`・`unit`・`label`) |
| `quote 出どころ` | 段落1つ(引用した言葉) | 他人の言葉。見出しの文が出どころ(必須) | `quote`(`content`・`source`) |
| `notes` | 段落・箇条 | 発表者ノート(スライドには出ない)。3.6 | スライドの `notes` |
| `columns` | `column` だけ(2つか3つ) | 並列のまとまり(段組みの指定ではなく「並列」という意味)。3.9 | `columns` |
| `column 題` | 段落・箇条・表・画像(1枚)・`note` | 並列のまとまりの1つ。題は省ける。属性 `role=main` | `columns.columns[]`(`title`・`role`・`body`) |

```
:::takeaway
減った問い合わせの大半は **初回の登録** に関するものだった
:::

::::stat
:::value 3 → 12 {unit=機種}
対応する端末の機種
:::
:::value −40 {unit=%}
月あたりの問い合わせ(前年同月比)
:::
::::

:::quote 窓口担当 A、2026 年 9 月の聞き取り
前は登録のたびに 3 つの画面を行き来していたが、いまは 1 つの画面で終わる。
:::
```

- `takeaway`・`quote`・`value` の中は段落1つ(改行したいなら行末に `\`)。空・2つ目の
  段落・箇条は `misplaced-content`。`takeaway`・`stat` は見出しの文を持てない。
- `value` の見出しの文は **書いたとおりの文字** で IR の `value` になる(文中の記法は読まない)。
  数字を1つも含まないものは `invalid-value`、無いものは `missing-key`。属性は `unit` だけで、
  値は空白を含まない1語(`{unit=%}`・`{unit=機種}`)。`value` が5つ以上は `misplaced-content`
  (系列は表・グラフ。`docs/patterns.md` の R3・R4)。`value` を `stat` の外に書くのも誤り。
- `quote` の見出しの文が無ければ `missing-key`(他人の言葉は出どころと対。R11)。
- 理由:
  - 引用を `>` で書かないのは、`>` が GFM の注意書き(`> [!NOTE]`)・メールの引用など
    複数の意味で使われ、出どころを書く場所も無いため(`>` は引き続き `unsupported-syntax`
    で、`:::quote` を示す)。出どころを「— 名前」の行のような記号の約束事にもしない。
  - 出どころを囲みの見出しの文にしたのは、`:::option 名前` と同じく「囲みの名前の後ろ =
    その囲みが名乗る意味の短い文」の形に揃えるため(何の文かは `boxloom schema` の
    `label` が名乗る)。
  - 数を箇条(`- 12 台 対応機種`)で書かないのは、どこまでが数・単位・説明かを空白の位置で
    決めることになり、意味が記号と位置に頼るため。`:::value` の見出し・属性・中身の3か所に
    名前つきで分けた。

いまは「予定」の囲みは無い。

囲みの一覧はコアが固定で持つのではなく、**テンプレートとサブシステムが名乗る**。
いまのテンプレートが名乗っていない囲みを使うと誤りになる。

入れ子は3段まで。閉じの省略は許さない(AI も人も、省略規則は忘れやすいため)。

### 3.5 サブシステム

言語名つきのコードブロックで、中身は YAML。言語名がサブシステムの名前:

````
```gantt
span: 2026-09 .. 2034-11
groups: …
```
````

- 中身はサブシステムが名乗る Schema で検査する。知らないキーは誤り。
- 中身は IR の Gantt の `data`(`schema/subsystems/gantt.schema.json`)と同じキー。
- 状態は意味役割名(`support` `conditional` `none` `danger`)。色は書けない。
  ハードウェアのリスクのような「期間の性質」は状態ではなく、グループの `risk: A .. B`。
- 注釈は位置や上下を持たない。どのグループ・行に付くかを `target`(`id`)で示し、
  置き場所はサブシステムが決める。
- 日付は `YYYY-MM` か `YYYY-MM-DD`。YAML の日付型への自動変換はせず、文字列として
  受け取ってサブシステムが解釈する。範囲は `A .. B` と書け、パーサーが IR の
  `{from, to}` に変える。複数行のラベルは YAML の配列で書く(IR の行の配列になる)。
- YAML は落とし穴の少ない部分集合で読む: 参照(`&` `*`)・タグ(`!`)・複数文書は誤り。
  真偽値は `true` / `false` だけ(`yes` `no` `on` `off` は文字列)。
- 文の値(IR でインラインのもの:`label`・`text`・凡例の説明など)は、本文と同じ
  文中の記法で読む(`**…**`・`` `…` ``・`[文](URL)`)。**YAML の列で書くと、各行を
  意味のある改行(`br`)でつなぐ**:`text: [ここまでに全利用者を, Board B/C へ移行予定]`。
  グループの `label`(IR で行の配列のもの)は、列がそのまま行の配列になる。
- 範囲は `A .. B` のほか、IR と同じ `{from: A, to: B}` でも書ける。
- 流れの書き方(`{…}` `[…]`)は1行の中で閉じる。複数行にまたがる値・ブロックの
  文字列(`|` `>`)は誤り。値の中に「: 」を書くときは `"…"` で囲む。
- コードブロックの言語名の後ろに `{variant=名前}` を書ける(IR のサブシステムの
  `variant`)。
- **```shapes**(生の図形への逃げ道。既存のパターンで表せない図のときだけ使う最後の手段。
  `docs/subsystem.md` の8節)も同じ作り。中身は IR の shapes の `data` と同じキー。
  座標と大きさは引用しない数(0〜1)、線の点は流れの書き方の列(`from: [0.1, 0.5]`)、
  色は役割名、文字の大きさは `textStyle` の段の名前で書く。

  ````
  ```shapes
  aspect: 2.6
  items:
    - {id: web, kind: roundRect, x: 0.22, y: 0.38, w: 0.15, h: 0.24, text: Web 層, line: accent}
    - {id: api, kind: rect, x: 0.44, y: 0.38, w: 0.15, h: 0.24, text: API 層, fill: accent, textColor: on_conditional}
    - {kind: connector, from: web, to: api, arrow: true}
  ```
  ````

  パーサーが断るのは形の誤り(知らないキー・0〜1 の外・引用した数・16進の色・知らない
  段)だけ。`x+w` が 1 を超える・テーマに無い役割名・無い id へのコネクタは
  `boxloom check` が、図形やキーの行と code(`shape-bounds`・`role-exists`・
  `target-exists`)で返す。例は `examples/md/shapes.bx.md`(↔ `examples/ir/shapes.json`)。
- **```process**・**```orgchart**・**```matrix**・**```funnel**(箱と線の図。
  `docs/subsystem.md` の11節)も同じ作り。中身は IR の `data` と同じキーで、書けるのは
  文と状態(`status`)と強調(`emphasis`)と、状態の意味の凡例(`legend`)だけ(座標・色・
  字の大きさは書けない)。matrix の推す列は `emphasis: <列の見出しの文>`。funnel の
  `value` は引用しない数なら数、それ以外は文字列。

  ````
  ```process
  steps:
    - {title: 申請, detail: 申請者が書く}
    - {title: 審査, detail: [平均3営業日, "**ここが詰まる**"], emphasis: true}
    - {title: 承認, status: support}
  ```
  ````

  パーサーが断るのは形の誤り(知らないキー・知らない状態の名前・型)だけ。matrix の
  行のセルの数が列の数と違うこと・`emphasis` が見出しに無いことは `boxloom check` が
  `matrix-cells`・`matrix-emphasis`(その行の行つき)で返す。`status` を使ったのに
  `legend` に文が無いと警告 `figure-legend`。例は `examples/md/figures.bx.md`(↔ `examples/ir/figures.json`)。
- **```chart**(数値のグラフ。`docs/subsystem.md` の12節)も同じ作り。書くのは比べ方
  `compare`(item・timeSeries・component・frequency・correlation)と、項目・系列・値・
  単位・強調だけで、グラフの形・色・大きさは書けない(形は比べ方とデータの数から
  サブシステムが選ぶ)。値は引用しない数(`"2"` は誤り)、散布図の点は `[x, y]`。

  ````
  ```chart
  compare: timeSeries
  unit: 件
  categories: [4月, 5月, 6月, 7月, 8月, 9月]
  emphasis: [9月]
  series:
    - name: 申込数
      values: [120, 150, 170, 210, 260, 340]
  ```
  ````

  パーサーが断るのは形の誤り(知らないキー・知らない比べ方・引用した数・点の形)だけ。
  値の数が項目の数と違う・強調する項目が無いなどは、`boxloom check` が `chart-values`・
  `chart-emphasis`(その欄の行つき)で返す。例は `examples/md/charts.bx.md`
  (↔ `examples/ir/charts.json`)。
- サブシステムの名前でないコードブロック(` ```go ` など)と言語名の無い
  コードブロックは、**いまは誤り(予定)**。IR 第一版にコードのブロックが無いため
  (冒頭の「食い違ったら IR を正とする」)。IR にコードのブロックが入ったら、
  ふつうのコードとして扱う。

### 3.6 発表者ノート

スライドの本文の一番外側に `:::notes` を置くと、その中身がスライドの発表者ノート
(IR のスライドの `notes`)になる。スライドの見た目には何も足さない。

```
# 今期の進め方

- 第1段階で試作を作る

:::notes
第1段階の「5 人」は目安。\
増やせるかは次の会議で決める。

話す順:
- 試作の狙い
  - 見た目より流れを確かめる
:::
```

- 置ける場所はスライドの一番外側だけ(`:::compare` などの中は誤り)。本文の前でも
  後ろでもよい。**1枚に1つ**(2つ目は誤り)。
- 最初のスライド(`# 題`)より前に置いた `:::notes` は**表紙のノート**になる。
  表紙が無い(文書の頭が無い・`cover: false`)ときは誤り。
- 中に書けるのは段落と箇条(入れ子は2段まで)だけ。囲み・コードブロック・
  生の HTML は誤り。見出しの文と属性は付けられない。
- ノートは見た目を持たないので、IR では**文字だけの文字列**になる。文中の記法は
  本文と同じく検査したうえで文字にする: `**…**` と `` `…` `` は中の文字、
  `[文](URL)` は `文 (URL)`、行末の `\` は改行。
- 段落と箇条の項目は1つずつ改行で区切る(空行は残らない)。箇条は `- ` を頭に付け、
  入れ子の項目はさらに空白2つで字下げする。上の例の IR の `notes` は
  `"第1段階の「5 人」は目安。\n増やせるかは次の会議で決める。\n話す順:\n- 試作の狙い\n  - 見た目より流れを確かめる"`。
- 例は `examples/md/speaker-notes.bx.md`(IR は `examples/ir/speaker-notes.json`)。

### 3.7 表

GFM の表で書く。1行目が見出しの行、2行目が区切りの行、3行目からが本体。

```
| 世代 | 台数 | 保守の期限 |
| --- | --- | --- |
| 第1世代 | 120台 | 2027-03 |
| 第2世代 | 85台 | 2029-12 |
: 台数は2026年10月の棚卸しによる
```

- 行は `|` で始めて `|` で終える(終わりの `|` が無いのは誤り)。
- **列の配置の指定(`:--` `:--:` `--:`)は誤り**(`unsupported-syntax`)。配置は
  テンプレートが決める:dark / light は、本体のセルがすべて数(`120台`・`12.5%`・
  `1,200` のような、数と短い単位)の列を右に揃え、ほかは左に揃える。
- どの行も見出しと同じ数のセル(違えば `table-shape`)。空のセルは `| |`。
- セルの中はインラインだけ(強調・コード・リンク)。`\|` はセルの中の文字の `|`
  (コードの中でも)。箇条・囲み・改行は書けない。
- 表の**すぐ次の行**の `: 説明` は表の説明の文(IR の `caption`。空行を挟むとふつうの段落)。
- 列の幅は書かない。テンプレートが中身から測る(折り返さない幅と、単語を割らない
  最小の幅。`docs/fonts.md` の字体の幅)。行が多くて収まらなければ `layout-overflow`
  の警告で、字を小さくしたり次のスライドへ送ったりはしない。
- PPTX では本物の表(`a:tbl`)になり、PowerPoint・LibreOffice で表として編集できる。

### 3.8 画像

**画像だけの段落**に書く。1枚なら `image`、2〜6枚を空白か改行で並べると `gallery`。

```
![受付の画面の試作。左に窓口の一覧、右に週ごとの件数](images/screen.png "試作の画面")

![第1段階](images/step-1.png "1. 受け付ける") ![第2段階](images/step-2.png "2. 振り分ける")
```

- **代わりの文(`[…]`)は必須**。空なら `image-alt`。画像が何を示すかを1文で書く
  (読み上げと、画像が無いときに使う。PPTX では画像の代わりの文 `descr` になる)。
- `"…"`(Markdown の title)は画像の説明の文(IR の `caption`)。画像の下に出る。
- path は**文書のファイルからの相対パス**(区切りは `/`)。URL・絶対パス・文書の
  フォルダの外(`..`)・`\` は `image-path` の誤り。ファイルが無い・読めないものは
  `image-missing`、PNG・JPEG でないもの(SVG・GIF なども)は `image-format`
  (この2つは `boxloom check`・`build` が見る。パーサーはファイルを読まない)。
- 文の中に画像を書くのは誤り(`misplaced-content`)。7枚以上も誤り。
- 縦横比を保って枠に収める(切り抜かない・引き伸ばさない)。余りは右と下に残る
  (左上に寄せる)。2〜3枚の並びは1段、4枚は 2×2、5〜6枚は3列の2段。
- 画像と文を横に並べたいときは、画像の属性ではなく **`::::columns`** で書く(3.9)。

### 3.9 並列のまとまり

```
:::::columns
::::column 札の並べ方 {role=main}
- 1枚の札に1件の受付
- 丸の色は状態

:::note
色だけで状態を示さない
:::
::::
::::column
![手持ちの端末の画面の試作](images/device.jpg)
::::
:::::
```

- `columns` の中は `column` だけで、2つか3つ。4つ以上の並列で、それぞれ1行なら箇条にする。
- `:::column` の後ろの文はまとまりの題(省ける)。題は意味の一部なので IR の `title` になる
  (`##` の小見出しは使わない)。
- **幅の比は数では書かない。** 主のまとまり(ほかはそれを補う)に `{role=main}` を付ける
  (1つの `columns` に1つまで)。どれだけ広くするかはテンプレートが決める
  (dark / light は値の表の `columnMainWeight` = 2、ほかは 1)。付けなければ同じ幅。
  `width` などの属性は無い(`unknown-attribute`)。
- 中に書けるのは段落・箇条・表・画像(1枚)・`:::note`。`columns` の入れ子・`compare`・
  画像の並び・サブシステムは書けない(`misplaced-content`)。
- はみ出しはまとまりごとに測る(一番高いまとまりがブロックの高さを決める)。
  `layout-overflow` は、まとまりの中のはみ出したブロックを
  `/slides/S/body/N/columns/C/body/K` で指す。

## 4. 例(比較文書の3枚)

````markdown
---
title: ランタイム移行計画
subtitle: ボード世代ごとの選択肢と期限
author: Boxloom デモ
template: light
---

# 論点

- 第1世代(Board A)は **2029年12月** にハードウェア保守が切れる
- Loom 4 は Store X 非対応のため、第1世代は移行先になり得ない
- 第2世代以降は Loom 継続と Weft OS 移行の両方が選べる

:::::compare
::::option Loom 継続
:::pros
- 既存の運用手順をそのまま使える
:::
:::cons
- 定額料金が 28/5・31/5 に更新
:::
::::
::::option Weft OS 移行
:::pros
- OS 25 で 2034年11月まで延ばせる
:::
:::cons
- 移行期間の確保が要る
:::
::::
:::::

:::note
期日はすべてイメージ
:::

# ランタイム選択肢と利用可能期間

```gantt
span: 2026-09 .. 2034-11
ticks:
  - {at: 2029-12, label: Board A・Store X, status: danger}
milestones:
  - {at: 2028-05, label: ライセンス更新}
legend:
  support: ランタイム サポート有
  none: ランタイム サポート無
  risk: ハードウェア サポート無
groups:
  - id: gen1
    label: [第1世代, Board A, 120台, Store X]
    risk: 2029-12 .. 2031-02
    rows:
      - label: Loom 2
        sub: 固定版
        segments:
          - {from: 2026-09, to: 2029-12, status: none, label: 内部利用のみ、外部保守}
      - label: Loom 3
        sub: 固定版
        segments:
          - {from: 2026-09, to: 2028-08, status: support, label: 内部+外部利用}
          - {from: 2028-08, to: 2029-12, status: conditional, label: 延長要確認}
annotations:
  - at: 2029-12
    target: gen1
    text: [ここまでに全利用者を, Board B/C へ移行予定]
    status: danger
    emphasis: true
```
````

この例は `examples/md/demo-deck.bx.md` にあり、パーサーで IR にすると
`examples/ir/demo-deck.json` と JSON として等しくなる(試験で確かめている)。
デモの Gantt 全体は `examples/md/gantt-full.bx.md`(↔ `examples/ir/gantt-full.json`)。

比較文書の案B と違うのは、良い点・悪い点を `+` / `-` の記号ではなく `:::pros` / `:::cons`
の名前で書くところ(2章の「約束事は名前で書く」)。

## 5. 誤りの返し方(`boxloom check`)

`boxloom check -format json deck.bx.md` は、誤りを1件ずつ次の形で返す。Markdown の
パーサー(`parse`)・`boxloom check`・`tools/validate_ir.py` は、どれもこの形で出す。

```json
{
  "file": "deck.bx.md",
  "line": 14, "column": 1,
  "path": "/slides/0/body/1",
  "code": "unknown-attribute",
  "message": "属性 color はありません。見た目はテンプレートが決めます",
  "expected": ["variant", "id"],
  "hint": "強調したいなら **…** か、テンプレートの variant を使ってください"
}
```

- `line`・`column` は Markdown の中の場所。**IR(JSON)を直接検査したときは出さない**。
  Markdown の記法が通ったあとの意味の誤り(日付が範囲の外など)は、IR の場所から
  引いた行だけを付ける(`column` は無い)。
- `path` は IR の中の場所(JSON Pointer)。いつも出す(`""` は文書全体)。
- `expected`(選べるもの)と `hint`(直し方)は、あるときだけ出す。
- `code` は決まった一覧(`unknown-block` `unknown-attribute` `raw-html` `unsupported-syntax`
  `unclosed-block` `schema` `layout-overflow` …)。AI はこれで分岐できる。
- テンプレートの規則の検査(はみ出し・fill-ratio など)も同じ形で返す。
  `layout-overflow` のときは、どの意味ブロックがどれだけはみ出したかを付ける。
- 警告(fill-ratio など)は `warnings` に同じ形で入る。
- 終了コードは、誤りがあれば 1、警告だけなら 0(`-strict` なら警告も 1)。
- `code` の全部の一覧は、記法の誤りが8章、意味の誤りと警告が `docs/ir.md` の
  「意味の検査」の節。

## 6. AI に書かせるときの決まり

- `boxloom schema` で、いまのテンプレートとサブシステムが名乗る囲み・属性・
  サブシステムの Schema を、まとめて JSON で出す。AI にはこれを渡す。
  `-template 名前` でそのテンプレートの分だけに絞る。出力の形は
  `docs/schema-command.md`(IR を直接書くための Schema も `irSchema` に同梱する)。
- 推奨の流れ: 書く → `boxloom check` → 誤りを渡して直させる、を誤りが無くなるまで回す。
- 構造化出力で確実に書かせたいときは、Markdown ではなく IR(JSON)を直接書かせてよい。
  `boxloom fmt --to md` で IR から Boxloom Markdown に戻せるようにする(往復できること)。

## 7. まだ決めていないこと

- 拡張子(`.bx.md` 案。パーサーは `.md` で終わるものをすべて読む)。
- スライドの `id` を IR に足すか。`takeaway`(3.4)と `columns`(3.9)は入った。
- (決めた)**小見出し(`##`)は持たない。** 小見出しは Markdown の書き方の都合で、
  意味ではない。IR は意味から設計し、記法の都合を持ち込まない。`##` 以下の見出しは
  誤りのまま。まとまりに名前が要るなら、それは意味の型(例:`columns` の各まとまりの
  題)として IR に持ち、Markdown では囲みの見出しの文(`:::名前 見出しの文`)で書く。
- (決めた)**表と画像は第1層のブロックにする**(サブシステムにしない)。表は `table`、
  画像は `image`(1枚)と `gallery`(2〜6枚)。理由と決まりは `docs/patterns.md` の
  2.1・2.2、書き方は 3.7・3.8。
- 1つの文書を複数ファイルに分ける方法(取り込み)。
- 囲みの属性の値の型(文字列だけか、数や真偽値も許すか)。いまは空白を含まない1語の
  文字列だけ(`unit` もこれ。空白を含む単位はまだ書けない)。

## 8. パーサー(Go 版)

```sh
cd boxloom
go run ./cmd/boxloom parse ../examples/md/demo-deck.bx.md            # IR(JSON)を標準出力に書く
go run ./cmd/boxloom parse -format json deck.bx.md                   # 誤りを {"ok": false, "errors": [...]} で出す
go run ./cmd/boxloom build ../examples/md/demo-deck.bx.md            # .md もそのまま PPTX にする
```

- 拡張子が `.md`(`.bx.md` を含む)なら Markdown として、それ以外は IR の JSON として読む。
  出力の名前は `deck.bx.md` → `out/deck.pptx`。
- 誤りは最初の1件で止めず、読めるところまで全部集めて、行の順に返す。形は5章の形
  (`file`・`line`・`column`・`path`・`code`・`message`・`expected`・`hint`)。
  `-format text`(既定)は `file:line:column path: code: message` の1行ずつ。
- いまの `code` の一覧:`unknown-block`・`unknown-attribute`・`raw-html`・
  `unsupported-syntax`・`planned-syntax`(仕様で「予定」のもの)・`unclosed-block`
  (囲み・コードブロック・強調・コロンの数の不一致)・`misplaced-content`(その場所に
  書けないもの。例 `:::option` を `:::compare` の外に書いた)・`unknown-key`・
  `missing-key`・`invalid-value`(YAML と文書の頭の値・`role` の値)・`yaml`(YAML の部分集合の外)・
  `table-shape`(表の行のセルの数が見出しと違う)・`image-path`(画像の path が URL・
  絶対パス・文書のフォルダの外)・`image-alt`(画像の代わりの文が無い)・
  `schema`(できた IR が通らない。ふつうは起きない)。
- shapes と図(process・orgchart・matrix・funnel)と chart のコードブロックは、YAML の形の検査
  だけをする(サブシステム自身には読ませない。意味の誤りは `boxloom check` が行つきで返す)。
- Gantt のコードブロックは、YAML の検査が通ったあと Gantt サブシステム自身にも
  読ませる。Schema では書けない誤り(`target` の行き先が無い・id の重複・範囲の
  前後)もここで見つかる(このときの `line` はコードブロックの開きの行)。
- `boxloom check` は `.md` も受け取る(記法の誤り → 無ければ IR の意味の検査と
  組み立ての警告。`docs/ir.md` の「意味の検査」)。`layout-overflow`(はみ出し)の
  検査はまだ無い。
