# 断られるべき IR の例

どれも `../demo-deck.json`(`shapes-*.json` は `../shapes.json`、`stat-*.json`・`statement-*.json`・`quote-*.json` は `../text-patterns.json`、`figure-*.json`・`funnel-*.json` は `../figures.json`)を1か所だけ変えたもの。
表・画像・並列のまとまりの例(`table-*`・`image-*`・`column*`)は、`../blocks.json` の
表紙とスライド1枚だけを取り出して1か所変えたもの。`tools/validate_ir.py --self-test` は、これらが全部断られることを確かめる。

| ファイル | 変えたところ |
|---|---|
| `align-on-note.json` | 注記に揃えの指定を足した |
| `bad-variant-name.json` | 変種の名前の形が不正 |
| `color-on-segment.json` | Gantt の区間に16進の色を足した |
| `color-on-slide.json` | スライドに色を足した |
| `design-role-as-status.json` | 状態にデザイン側の役割名(text_primary)を書いた |
| `hex-as-status.json` | 状態に役割名ではなく色を書いた |
| `inline-bold.json` | 文中に見た目の装飾(bold)を書いた |
| `position-on-annotation.json` | 注釈に位置を足した |
| `unknown-subsystem.json` | 登録されていないサブシステム |
| `shapes-coordinate-out-of-range.json` | shapes の図形の x を 1.2(0〜1 の外)にした |
| `shapes-font-size.json` | shapes の字の段(textStyle)に数を書いた |
| `shapes-hex-fill.json` | shapes の図形の塗りに役割名ではなく16進の色を書いた |
| `statement-with-body.json` | 主張のスライドに本文(body)を足した |
| `stat-five-items.json` | 統計強調の数を5つにした(1〜4個) |
| `stat-value-no-digit.json` | 統計強調の数を、数字の無い言葉(多数)にした |
| `quote-without-source.json` | 引用の出どころ(source)を消した |
| `figure-design-role-as-status.json` | process の段階の状態にデザイン側の役割名(accent)を書いた |
| `funnel-width.json` | funnel の段に幅(width)を足した(幅はサブシステムが決める) |
| `table-align.json` | 表に列の揃えの指定(`align`)を足した |
| `column-width.json` | 並列のまとまりに数の幅(`width: 2`)を足した |
| `columns-two-mains.json` | 並列のまとまりの2つに `role: main` を付けた |
| `columns-nested.json` | まとまりの中に `columns` を入れ子にした |
| `image-url.json` | 画像の `src` を URL にした(`boxloom check` は `image-path`) |
| `image-empty-alt.json` | 画像の代わりの文を空にした(`boxloom check` は `image-alt`。このフォルダに画像は無いので `image-missing` も出る) |
