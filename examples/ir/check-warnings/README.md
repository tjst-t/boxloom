# 誤りは無いが、boxloom check が警告を出す IR の例

形も意味も正しいので `tools/validate_ir.py` も `go run ./cmd/boxloom check` も通す(誤り0件・
終了コード 0)。組み立てると警告が出る(`-strict` なら終了コード 1)。
`boxloom/overflow_test.go`・`boxloom/figures_test.go`・`boxloom/shapes_test.go`・`boxloom/gantt_label_test.go`・`boxloom/patterns_text_test.go` が、決まった code の警告が決まった場所に出ることを確かめる。

| ファイル | 中身 | 警告の code と場所 |
|---|---|---|
| `layout-overflow.json` | 1枚に箇条・比較・箇条・注記を詰め込みすぎた | `layout-overflow` を `/slides/1/body/2`(箇条)と `/slides/1/body/3`(注記)に |
| `gantt-segment-label.json` | Gantt の3つの行:帯に1行で収まる文・帯の中で折り返す文・帯が狭すぎて置けない文 | `gantt-segment-label` を `/slides/0/body/0/data/groups/0/rows/2/segments/0`(置けない区間)に。ほかに `fill-ratio` |
| `columns-overflow.json` | 段落の下に並列のまとまりを2つ置き、右のまとまりの箇条を長くした | `layout-overflow` を `/slides/1/body/1/columns/1/body/0`(右のまとまりの箇条)に。左のまとまりは収まるので出ない |
| `shapes-overlap.json` | `../shapes.json` のデータベースの楕円を上にずらし、認証サービスの矩形に重ねた | `shapes-overlap` を `/slides/1/body/1/data/items/7`(後から置いた楕円)に |
| `shapes-connector-angle.json` | 斜めに並んだ3組の図形をコネクタで繋いだ。2組は辺を選び直して繋げるが、受付→振り分けは浅くしか繋げない | `shapes-connector-angle` を `/slides/0/body/0/data/items/6`(そのコネクタ)に |
| `figure-legend.json` | process の2つの段階に状態を付け、凡例の文を1つだけ書いた | `figure-legend` を `/slides/1/body/0/data/legend` に(文の無い conditional)。ほかに `fill-ratio` |
| `statement-overflow.json` | `../text-patterns.json` の主張の1文を、ページに収まらないほど長くした | `layout-overflow` を `/slides/4`(主張のスライド。本文を持たないのでスライドを指す)に |

`layout-overflow.json`・`columns-overflow.json` は `../../md/warnings/` の同じ名前の `.bx.md` を
`go run ./cmd/boxloom parse` したもの(.md を check すると、同じ警告に Markdown の行が付く)。
`gantt-segment-label.json`・`shapes-connector-angle.json`・`figure-legend.json` も同じく `../../md/warnings/` の同じ名前の `.bx.md` から作った。
