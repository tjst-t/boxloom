# Go のライブラリとして使う

Boxloom は Go のパッケージ `boxloom`(モジュールの根 `boxloom/`)として使える。
コマンド `boxloom`(`boxloom/cmd/boxloom`)も、このページに挙げた入口だけで
できている。安定させるつもりなのは下の表の名前だけで、ほかに公開されている
名前(`Spec`・`PPTXBuilder`・`Template` など)は予告なく変わる。

```go
src, _ := os.ReadFile("deck.bx.md")
deck, err := boxloom.ReadDeck("deck.bx.md", src) // .md は Markdown、それ以外は IR(JSON)
if err != nil {
	return err // 記法の誤りなら boxloom.MarkdownErrors(行・列つきの Issue の一覧)
}
f, _ := os.Create("deck.pptx")
defer f.Close()
res, err := boxloom.Build(f, deck, boxloom.BuildOptions{Template: "dark"})
// res.Warnings は本文の埋まり具合・はみ出しなどの警告。書き出しは止めない
```

- ライブラリはファイルを読み書きしない。名前(誤りの場所に添える)と中身の
  バイト列を受け取り、PPTX は `io.Writer` に書く。
- 依存は標準ライブラリだけ。テンプレート・字体の寸法・Schema はパッケージに
  埋め込んである。
- 同じプロセスの中で `Build`・`Check` を並行して呼ばない。文字を測る字体を
  パッケージの中で切り替えているため(1つずつ呼べば、呼んだあとは元に戻る)。

## 安定させる入口

| 名前 | 形 | すること |
|---|---|---|
| `ParseMarkdown` | `(file string, src []byte) (*IRDeck, []Issue, error)` | Boxloom Markdown → IR。記法の誤りは `[]Issue`(行・列つき)で返し、そのとき deck は nil。記法は通ったのに IR として読めなかったときだけ error |
| `MarkdownToIR` | `(file string, src []byte) ([]byte, []Issue)` | Boxloom Markdown → IR の JSON(`boxloom parse` の出力) |
| `ReadIR` | `(file string, data []byte) (*IRDeck, error)` | IR(JSON)を読む。知らないキー・足りない欄は断る。誤りは「file: 場所: 文」 |
| `ReadDeck` | `(file string, data []byte) (*IRDeck, error)` | 拡張子で選ぶ。`.md` は `ParseMarkdown`(記法の誤りは `MarkdownErrors`)、それ以外は `ReadIR` |
| `Check` | `(file string, data []byte, CheckOptions) *CheckResult` | 意味の検査と組み立ての警告と本文の容量(`boxloom check` の1ファイル分) |
| `Build` | `(w io.Writer, *IRDeck, BuildOptions) (*BuildResult, error)` | テンプレートで組んで PPTX を w に書く |
| `Schema` | `(template string) ([]byte, error)` | AI に渡す一覧の JSON(`boxloom schema`。空なら全部のテンプレート) |
| `Templates` | `() []string` | 同梱したテンプレートの名前(並べたもの。既定は light) |

型と値:

| 名前 | 中身 |
|---|---|
| `IRDeck` | 読んだ IR(`docs/ir.md`)。中の欄は IR の形のまま |
| `Issue` | 誤りと警告の1件(`docs/content-syntax.md` の5章の形。`File`・`Line`・`Column`・`Path`・`Code`・`Message`・`Expected`・`Hint`)。`String()` で1行の文 |
| `MarkdownErrors` | `[]Issue` の error。`errors.As` で取り出す |
| `CheckOptions` | `Template`(IR の template より優先)・`Strict`(警告も誤り扱い) |
| `CheckResult` | `OK`・`Errors`・`Warnings`・`Capacity`。JSON にすると `boxloom check -format json` と同じ形(切れ端は nil にならない) |
| `SlideCapacity` | 本文を持つスライドごとの容量(`docs/ir.md` の「容量の計測」) |
| `BuildOptions` | `Template`(空なら IR の template、それも無ければ light) |
| `BuildResult` | `Template`(使った名前)・`Slides`(枚数)・`Warnings`(`"slide 2: [FILL-RATIO WARNING] …"` の文) |
| `ErrUnknownTemplate` | テンプレートの名前が無いとき。`Build`・`Schema` の誤りを `errors.Is` で見分ける |

- `Build` はテンプレートが無いときと組めなかったとき、w に何も書かずに誤りを返す。
  組めなかったときの誤りは IR の中の場所を添える(例 `slides[2]: body[0]: gantt: …`)。
- 警告を `Issue` の形で欲しいときは `Check` を使う。`Check` は誤りが無ければ組み立てまで
  通すので、`Build` の警告と同じものが code つきで返る。
- `Check` は file の拡張子で読み方を選ぶ。`.md` なら誤りと警告に Markdown の行が付く。

## 安定させない入口(コマンドのため)

| 名前 | すること |
|---|---|
| `WriteDemo(w, theme)` | 従来の Go で組んだデモ(引数なしの `boxloom`)。v11 の再現 |
| `NewFontCheck`・`FontCheck`・`WriteFontList` | `boxloom fonts`(`docs/fonts.md`)の測りと一覧 |
| `WriteDeck` | 組んだスライド(`PPTXBuilder`)を PPTX にする低い層 |

## 別のモジュールから使う

`boxloom/examples/libuse/` が例。自分の `go.mod` で `require boxloom v0.0.0` と
`replace boxloom => ../..` を書き、`.bx.md` か IR を読んで PPTX を書く。

```sh
cd boxloom/examples/libuse
go run . -template dark ../../../examples/md/demo-deck.bx.md out.pptx
```

ライブラリの試験 `TestExternalModule` が、これをネットワークなしで
(`GOFLAGS=-mod=mod`・`GOPROXY=off`・`GOTOOLCHAIN=local`・`GOWORK=off`)ビルドして、
`examples/md/demo-deck.bx.md` から zip の中に `ppt/presentation.xml` を持つ PPTX が
できることを確かめる。入れ子のモジュールなので、ライブラリの根での
`go test ./...`・`go vet ./...` には入らない。

## ファイルの並び

| 場所 | 中身 |
|---|---|
| `boxloom/`(`package boxloom`) | ライブラリ。入口は `api.go`・`check.go`、説明は `doc.go`。埋め込みのデータ(`schemas/`・`templates/`・`fonts/`・`subsystems/`)と試験もここ |
| `boxloom/cmd/boxloom/` | コマンド。フラグ・ファイルの読み書き・終了コードだけを持つ |
| `boxloom/examples/libuse/` | 別のモジュールから使う例(自分の `go.mod`) |
