# 誤りは無いが、boxloom check が警告を出す IR の例

形も意味も正しいので `tools/validate_ir.py` も `go run ./cmd/boxloom check` も通す(誤り0件・
終了コード 0)。組み立てると警告が出る(`-strict` なら終了コード 1)。
`boxloom/overflow_test.go`・`boxloom/shapes_test.go` が、決まった code の警告が決まった場所に出ることを確かめる。

| ファイル | 中身 | 警告の code と場所 |
|---|---|---|
| `layout-overflow.json` | 1枚に箇条・比較・箇条・注記を詰め込みすぎた | `layout-overflow` を `/slides/1/body/2`(箇条)と `/slides/1/body/3`(注記)に |
| `shapes-overlap.json` | `../shapes.json` のデータベースの楕円を上にずらし、認証サービスの矩形に重ねた | `shapes-overlap` を `/slides/1/body/1/data/items/7`(後から置いた楕円)に |

`layout-overflow.json` は `../../md/warnings/layout-overflow.bx.md` を
`go run ./cmd/boxloom parse` したもの(.md を check すると、同じ警告に Markdown の行が付く)。
