# Schema では通るが、boxloom check で断られる IR の例

どれも `../demo-deck.json`・`../gantt-full.json`・`../shapes.json`・`../text-patterns.json`・`../figures.json`・`../charts.json` のどれかを1か所だけ変えたもの。
表と画像の例(`table-shape.json`・`table-total-shape.json`・`image-*.json`)は、`../blocks.json` の表紙とスライド1枚
だけを取り出して1か所変えたもの(画像の src はこのフォルダからの相対パス)。
形(キー・型・enum)は正しいので `tools/validate_ir.py` は通す(`--self-test` は
「通ること」を確かめる)。意味が通らないので `go run ./cmd/boxloom check` が断る
(`boxloom/check_test.go` が、ファイルごとに決まった rule の誤りが1件だけ出ることを
確かめる)。

| ファイル | 変えたところ | rule |
|---|---|---|
| `date-order.json` | 区間の from と to を入れ替えた | `date-order` |
| `duplicate-id.json` | 別のグループの行に、すでにある id を付けた | `unique-id` |
| `not-a-real-date.json` | 節目の日付を 2028-02-30 にした | `date-valid` |
| `outside-span.json` | 節目の日付を範囲(span)の外にした | `within-span` |
| `unknown-target.json` | 注釈の target を、どこにも無い id にした | `target-exists` |
| `unknown-template.json` | テンプレートの名前を、無いものにした | `template-exists` |
| `unknown-variant.json` | テンプレートに無い変種(dense)を付けた | `variant-exists` |
| `shapes-out-of-frame.json` | shapes の図形の幅を広げ、x+w が 1 を超えた | `shape-bounds` |
| `shapes-unknown-connector.json` | shapes のコネクタの to を、どこにも無い id にした | `target-exists` |
| `shapes-unknown-role.json` | shapes の図形の塗りを、テーマに無い役割名にした | `role-exists` |
| `quote-empty-source.json` | 引用の出どころを空白だけにした | `empty-text` |
| `matrix-cells.json` | matrix の2行目のセルを1つ減らした(列は3つ) | `matrix-cells` |
| `matrix-emphasis.json` | matrix の推す列(emphasis)に、見出しに無い名前を書いた | `matrix-emphasis` |
| `matrix-column-groups.json` | `matrix-emphasis.json` の emphasis を除き、列の群(columnGroups)を span 1 の2つにした(列は3つ) | `matrix-column-groups` |
| `table-shape.json` | 表の2行目のセルを1つ減らした(見出しは4つ) | `table-shape` |
| `table-total-shape.json` | `table-shape.json` の2行目を直し、セルが3つの合計の行(`total`)を足した(見出しは4つ) | `table-shape` |
| `table-groups.json` | `../table-groups.json` の表紙と1枚目の表だけを取り出し、最初の群の行の数(`groups/0/rows`)を3から2にした(群の行の和が本体の行より1つ少ない) | `table-groups` |
| `table-group-columns.json` | `../table-group-columns.json` の表紙と1枚目の表だけを取り出し、2列目の群(`groupColumns/0/groups`)の「設備の支援」を国の最後の行から県の2行までの1つの群(3行)にした(先頭の列の国と県の境を越える) | `table-groups` |
| `table-points.json` | `../table-points.json` の表紙と1枚目の表だけを取り出し、2行目の「すぐ打てる手」のセルの箇条の2つ目の項目を空白だけにした | `table-points` |
| `table-continues.json` | `../table-continues.json` の1枚目と2枚目の表の間に区切りのスライド(`section`)を挟んだ(続きの表 `continues` の前のスライドに表が無い) | `table-continues` |
| `image-missing.json` | 画像の src を、無いファイルにした | `image-missing` |
| `image-format.json` | 画像の src を、画像でないファイル(`README.md`)にした | `image-format` |
| `chart-values.json` | chart(時系列の縦棒)の系列の値を1つ減らした(項目は6つ) | `chart-values` |
| `chart-emphasis.json` | chart(構成比の円)の強調に、項目に無い名前を書いた | `chart-emphasis` |
| `pairs-label-mixed.json` | `../pairs-label.json` の表紙と1枚目だけを取り出し、3つ目の対の名前(`label`)を消した(名前のある対と無い対が混ざる) | `pairs-label` |

形が誤っていて Schema で断られる例は `../invalid/` にある。それらも
`boxloom check` は断る(Go の IR の読み込みが断る。rule は `ir-read` か `build`)。
