# Go のライブラリとして使う

Boxloom は Go のパッケージ `boxloom`(モジュールの根 `boxloom/`)として使える。
コマンド `boxloom`(`boxloom/cmd/boxloom`)も、このページに挙げた入口だけで
できている。安定させるつもりなのは「安定させる入口」の表の名前だけ。ほかに公開されている
名前は、次のどれかのために公開しているもので、予告なく変わる。

- 安定させる入口の返り値の中の型(`CheckResult.Capacity` の `BlockCapacity`・`CardCapacity`・
  `ColumnCapacity`・`PairCapacity`、`Schema` の JSON の形の `SchemaOutput`・`TypeSchema` など)。
  JSON にしたときの形は入口と同じく保つが、Go の型としては安定させない
- 「安定させない入口」の表の入口とその引数・返り値(`LayoutReadOptions`・`LayoutDump` ほかの
  `Layout*`、`FontCheck`・`DemoResult` など)
- コマンドと試験が使う低い層(`Spec`・`PPTXBuilder`・`Template`・サブシステムの型など)

```go
src, _ := os.ReadFile("deck.bx.md")
deck, err := boxloom.ReadDeck("deck.bx.md", src) // .md は Markdown、.yaml・.yml は YAML の deck、それ以外は IR(JSON)
if err != nil {
	return err // Markdown・YAML の誤りなら boxloom.MarkdownErrors(行・列つきの Issue の一覧)
}
f, _ := os.Create("deck.pptx")
defer f.Close()
res, err := boxloom.Build(f, deck, boxloom.BuildOptions{
	Template: "dark",
	Assets:   os.DirFS("."), // 画像(image・gallery の src)を読む口。根は文書のフォルダ
})
// はみ出し(layout-overflow)があれば f に何も書かず *boxloom.OverflowError を返す
// (errors.Is(err, boxloom.ErrLayoutOverflow))。oe.Issues が check と同じ path・文・hint の全件。
// BuildOptions.AllowOverflow なら今までどおり書き、はみ出しは res.Warnings に残る。
// BuildOptions.Fit なら、はみ出すスライドを値の表の梯子で詰めて書き、詰めたスライドは res.Info に残る。
// res.Warnings は本文の埋まり具合などの警告。はみ出しのほかは書き出しを止めない
```

- ライブラリはファイルを読み書きしない。名前(誤りの場所に添える)と中身の
  バイト列を受け取り、PPTX は `io.Writer` に書く。画像だけは文書の外にあるので、
  呼ぶ側が `fs.FS` を渡す(下の「画像」)。
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
| `ParseYAMLDeck` | `(file string, src []byte) (*IRDeck, []Issue)` | YAML の deck(IR の写し。`docs/content-syntax.md` の1章)→ IR。YAML の部分集合の外(code `yaml`)と、Go の読み込みが断るもの(知らないキー・型の違い。code `schema`)を、YAML の行・列つきの `[]Issue` で返す。そのとき deck は nil |
| `YAMLToIR` | `(file string, src []byte) ([]byte, []Issue)` | YAML の deck → IR の JSON(`boxloom parse deck.yaml` の出力)。誤りは `ParseYAMLDeck` と同じ |
| `ReadDeck` | `(file string, data []byte) (*IRDeck, error)` | 拡張子で選ぶ。`.md` は `ParseMarkdown`(記法の誤りは `MarkdownErrors`)、`.yaml`・`.yml` は `ParseYAMLDeck`(誤りは `MarkdownErrors`)、それ以外は `ReadIR` |
| `Check` | `(file string, data []byte, CheckOptions) *CheckResult` | 意味の検査と組み立ての警告と本文の容量(`boxloom check` の1ファイル分) |
| `Build` | `(w io.Writer, *IRDeck, BuildOptions) (*BuildResult, error)` | テンプレートで組んで PPTX を w に書く。はみ出し(`layout-overflow`)があれば書かずに `*OverflowError` |
| `Schema` | `(template string) ([]byte, error)` | AI に渡す一覧の JSON(`boxloom schema`。空なら全部のテンプレート) |
| `ParseTemplate` | `(file string, data []byte) (*Template, error)` | 外のテンプレートの値の表(JSON)を読む。検査は同梱のものと同じ。誤りは `*TemplateError`(`docs/template-values.md`) |
| `SchemaTemplate` | `(*Template) ([]byte, error)` | `Schema` の、`ParseTemplate` で読んだテンプレート版 |
| `Templates` | `() []string` | 同梱したテンプレートの名前(並べたもの。既定は light) |

型と値:

| 名前 | 中身 |
|---|---|
| `IRDeck` | 読んだ IR(`docs/ir.md`)。中の欄は IR の形のまま |
| `Issue` | 誤りと警告の1件(`docs/content-syntax.md` の5章の形。`File`・`Line`・`Column`・`Path`・`Code`・`Message`・`Expected`・`Hint`)。`String()` で1行の文 |
| `MarkdownErrors` | `[]Issue` の error(Markdown と YAML の deck の誤り。行・列つき)。`errors.As` で取り出す |
| `CheckOptions` | `Template`(IR の template より優先)・`Values`(`ParseTemplate` で読んだ値の表。あれば名前より優先)・`Strict`(警告も誤り扱い)・`Assets`(画像を読む `fs.FS`)・`Fit`(`BuildOptions.Fit` と同じに詰めて測る。詰めたスライドは `Info` の `fit-applied` と容量の `fit`) |
| `CheckResult` | `OK`・`Errors`・`Warnings`・`Capacity`・`Info`(知らせ。`-fit` の `fit-applied`。警告ではないので `Strict` でも `OK` に効かない。無ければ nil で JSON に出ない)。JSON にすると `boxloom check -format json` と同じ形(`Info` のほかの切れ端は nil にならない) |
| `SlideCapacity` | 本文を持つスライドごとの容量(`docs/ir.md` の「容量の計測」) |
| `BuildOptions` | `Template`(空なら IR の template、それも無ければ light)・`Values`(`ParseTemplate` で読んだ値の表。あれば名前より優先)・`Assets`(画像を読む `fs.FS`)・`AllowOverflow`(はみ出し `layout-overflow` があっても PPTX を書く。警告は `Warnings` に残る。false なら書かずに `*OverflowError`)・`Fit`(はみ出す本文のスライドを値の表の梯子 dense → `fit.shrink` で詰め、最初に収まった段で組む。最後の段でも収まらなければ `layout-overflow` のまま。IR は変えず、false なら出力は変わらない) |
| `Template` | 読んだテンプレート。`ParseTemplate` の返り値を `Values` に渡すためのもので、中の欄は安定させない |
| `TemplateError` | `ParseTemplate` の誤り。`Issue`(code は `template-values`、path は値の表の中の場所)を持つ。`errors.As` で取り出す |
| `BuildResult` | `Template`(使った名前)・`Slides`(枚数)・`Warnings`(`"slide 2: [FILL-RATIO WARNING] …"` の文)・`Info`(`Fit` で詰めたスライドの `"slide 2: [FIT-APPLIED INFO] …"` の文。無ければ nil) |
| `ErrLayoutOverflow` / `OverflowError` | はみ出しで `Build` が書かなかったとき(`errors.Is` / `errors.As`)。`Issues` は check の `layout-overflow` の警告と同じ path・文・hint の全件(`File` と行は空。呼ぶ側が添える)、`Warnings` は組み立ての警告の全部 |
| `BuildBlocking` | `([]Issue) int`。警告のうち build を止めるもの(`layout-overflow`)の数(`boxloom check` の要約の行) |
| `ErrUnknownTemplate` | テンプレートの名前が無いとき。`Build`・`Schema` の誤りを `errors.Is` で見分ける |

- `Build` はテンプレートが無いときと組めなかったとき、w に何も書かずに誤りを返す。
  組めなかったときの誤りは IR の中の場所を添える(例 `slides[2]: body[0]: gantt: …`)。
- 警告を `Issue` の形で欲しいときは `Check` を使う。`Check` は誤りが無ければ組み立てまで
  通すので、`Build` の警告と同じものが code つきで返る。
- `Check` は file の拡張子で読み方を選ぶ。`.md` なら誤りと警告に Markdown の行が付く。
  `.yaml`・`.yml` なら誤りと警告に YAML の行と列が、容量に行が付く。

## 画像(Assets)

`image`・`gallery` の `src` は文書のファイルからの相対パス(`docs/content-syntax.md` の
3.8)。ライブラリはファイルを開かず、`BuildOptions.Assets`・`CheckOptions.Assets` に
渡された `fs.FS` から `src` を読む。

- **根は文書のフォルダ**。コマンドは `os.DirFS(filepath.Dir(文書のパス))` を渡す。
  メモリの中の画像なら `fstest.MapFS` や `embed.FS` でもよい。
- `src` は `path.Clean` してから `fs.ReadFile` で読む。`fs.FS` の決まり
  (`fs.ValidPath`)により、`..` や絶対パスは読まれない。そのうえで、URL・絶対パス・
  `..`・`\` は読む前に `image-path` で断る。
- `Assets` が nil なら、画像を使う文書は `Check` が `image-missing` を返し、`Build` は
  誤りを返す(画像を使わない文書には要らない)。
- 形式は中身で見る(`image.DecodeConfig`。PNG と JPEG だけ)。画素の大きさから縦横比を
  取り、PPTX の `ppt/media/` に入れる(同じ中身の画像は1つにまとめる)。
- `FontCheck.Assets` も同じ(`boxloom fonts` が文書ごとに入れる)。

## 安定させない入口(コマンドのため)

| 名前 | すること |
|---|---|
| `WriteDemo(w, theme)` | 従来の Go で組んだデモ(引数なしの `boxloom`)。v11 の再現 |
| `NewFontCheck`・`FontCheck`・`WriteFontList` | `boxloom fonts`(`docs/fonts.md`)の測りと一覧 |
| `WriteDeck` | 組んだスライド(`PPTXBuilder`)を PPTX にする低い層 |
| `Layout(file, src, BuildOptions)` | `boxloom layout` の配置の段の木(`*LayoutDump`。JSON にすると `schema/layout.schema.json` の形。`docs/build.md` の「配置の段の木を見る」)。木の形は `docs/layers.md` の7章の4段目で変わるので安定させない |
| `BuildLayout(w, file, src, LayoutReadOptions)` | 開発者・試験用:`Layout` の木を読み直して PPTX を書く(`boxloom layout-build`。トークンを値の表の段で検査し、誤りは `MarkdownErrors`。`docs/build.md` の「配置の段の木を読み直す」)。作者の入口ではなく、安定させない |
| `IsTemplateFile` | `-template` の値がファイルのパスか(`.json` で終わるか、パスの区切りを含む) |

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
