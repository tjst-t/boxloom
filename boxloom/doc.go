// Package boxloom は、意味だけを書いた文書(Boxloom Markdown か JSON の IR)を、
// テンプレートで組んで、PowerPoint で編集できる PPTX にする。
//
// 作者は意味の木(スライド・見出し・箇条・比較・Gantt のタスクと状態…)だけを
// 書き、座標・色・字の大きさはテンプレートが決める。依存は標準ライブラリだけで、
// テンプレート・字体の寸法・Schema はパッケージに埋め込んである。
// ファイルの読み書きはしない(名前と中身を受け取り、io.Writer に書く)。
//
// 流れ:
//
//	src, _ := os.ReadFile("deck.bx.md")
//	deck, err := boxloom.ReadDeck("deck.bx.md", src) // .md は Markdown、それ以外は IR(JSON)
//	if err != nil { ... }                            // 記法の誤りは boxloom.MarkdownErrors
//	f, _ := os.Create("deck.pptx")
//	res, err := boxloom.Build(f, deck, boxloom.BuildOptions{Template: "dark"})
//	// res.Warnings: 本文の埋まり具合・はみ出しなどの警告(書き出しは止めない)
//
// 書き出さずに検査だけするなら Check(誤りと警告を Issue の一覧で、本文の
// 残りの高さを Capacity で返す)。AI に渡す記法と Schema の一覧は Schema。
//
// 安定させるつもりの入口は次のものだけ(docs/library.md)。ほかに公開されている
// 名前(Spec・PPTXBuilder・Template など)はコマンドと試験のための中身で、
// 予告なく変わる。
//
//   - 読む: ParseMarkdown・MarkdownToIR・ReadIR・ReadDeck・IRDeck・MarkdownErrors
//   - 検査する: Check・CheckOptions・CheckResult・Issue・SlideCapacity
//   - 書く: Build・BuildOptions・BuildResult・ErrUnknownTemplate
//   - 一覧: Schema・Templates
//
// 同じプロセスの中で Build・Check を並行して呼ばないこと(文字を測る字体を
// パッケージの中で切り替えているため)。
//
// コマンドは cmd/boxloom(build・check・parse・schema・fonts)。
package boxloom
