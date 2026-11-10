# 意味の中間表現(IR)

Boxloom のコンテンツは、どの記法で書いても最後は **IR(意味の中間表現)** になり、
テンプレートは IR だけを見る。IR は JSON で、形は JSON Schema(draft 2020-12)で
決める。

> 状態: 第一版の下書き(版 `0.1`)。`docs/content-syntax-comparison.md` の共通の
> お題3枚(表紙・論点と比較・Gantt)をすべて表せる範囲で作った。Go 版は
> IR を読んで PPTX を書ける(`go run . build`、`docs/build.md`)。Markdown から
> IR へのパーサーはまだ無い。

## ファイル

| 場所 | 中身 |
|---|---|
| `schema/core.schema.json` | コア:文書・スライド・ブロック・インライン・サブシステムの差し込み口 |
| `schema/subsystems/gantt.schema.json` | Gantt サブシステムのブロック |
| `schema/boxloom-ir.schema.json` | 検査の入口。コアに同梱のサブシステムを差し込んだもの |
| `examples/ir/demo-deck.json` | お題3枚を IR で書いた例 |
| `examples/ir/gantt-full.json` | デモの Gantt 全体(3グループ・9行・注釈4つ)を IR で書いた例 |
| `examples/ir/invalid/` | 断られるべき例(色・位置・見た目の装飾などを1か所だけ足したもの) |
| `tools/validate_ir.py` | 検査のコマンド |

```sh
pip install jsonschema                         # 4.18 以降
python3 tools/validate_ir.py deck.json         # 誤りを JSON で出す(正しければ exit 0)
python3 tools/validate_ir.py --self-test       # 例が通り、断られるべきものが断られるか
```

誤りは次の形で返す。`at` は IR の中の場所(JSON Pointer)で、AI に渡せばその場所を
直せる。

```json
{"file": "deck.json",
 "at": "/slides/2/body/0/data/groups/0/rows/1/segments/0/status",
 "message": "'#34C08F' is not one of ['support', 'conditional', 'none', 'danger']",
 "rule": "enum"}
```

## 形の要点

```
文書   { boxloom: "0.1", template?, meta?, slides: [スライド] }
スライド  { type: "cover",   variant?, title?, subtitle?, author?, notes? }
         { type: "content", variant?, title, body: [ブロック], notes? }
ブロック  paragraph | points | compare | note | subsystem   (どれも type で選ぶ)
インライン "文字列"  か  ["文字列", {type: em|code|link|br, …}, …]
サブシステム { type: "subsystem", subsystem: "gantt", variant?, data: {…} }
```

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

## まだ決めていないこと

- **`$id` のドメイン**:いまは `https://boxloom.invalid/…`(使えないことが
  分かる仮の名前)。OSS で公開するときに決める。
- **id の一意性・参照先の実在**(注釈の `target` が実在する id を指すか)、
  **日付の前後**(`from` が `to` より前か)は、JSON Schema では書けない。
  将来の `boxloom check` で見る。
- **画像・表・コード片・図の引用**など、お題3枚に無いブロック。
- **スライドの骨組みの枠**(title / body / aside …)をコンテンツから指すか。いまの
  IR は `body` の1つだけ。
- **デモの Gantt の再現**:注釈の置き場所は Gantt サブシステムが決める
  ようになった(`docs/subsystem.md` の6節)。デモの Gantt 全体を IR で書いた例が
  `examples/ir/gantt-full.json`。置き方は自動なので、手で決めたデモと全く同じ
  見た目ではない(注釈の上下が一部違う)。
