# Schema では通るが、boxloom check で断られる IR の例

どれも `../demo-deck.json` か `../gantt-full.json` を1か所だけ変えたもの。
形(キー・型・enum)は正しいので `tools/validate_ir.py` は通す(`--self-test` は
「通ること」を確かめる)。意味が通らないので `go run . check` が断る
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

形が誤っていて Schema で断られる例は `../invalid/` にある。それらも
`boxloom check` は断る(Go の IR の読み込みが断る。rule は `ir-read` か `build`)。
