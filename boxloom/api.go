// ライブラリの入口(docs/library.md)。コマンド(cmd/boxloom)はこれだけを使う。
//
// 流れは Markdown か IR(JSON)を読む(ParseMarkdown・ReadIR・ReadDeck)→
// 検査する(Check)→ PPTX を書く(Build)。ファイルの読み書きはしない。
// 名前と中身(バイト列)を受け取り、書き出しは io.Writer に書く。

package boxloom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// isMarkdownPath は Boxloom Markdown として読むファイルか(.md、.bx.md を含む)。
func isMarkdownPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

// MarkdownErrors は Markdown の記法の誤りの一覧(error として返すため)。
// ReadDeck が返す。
type MarkdownErrors []Issue

func (es MarkdownErrors) Error() string {
	var lines []string
	for _, e := range es {
		lines = append(lines, e.String())
	}
	return fmt.Sprintf("%d error(s)\n%s", len(es), strings.Join(lines, "\n"))
}

// ParseMarkdown は Boxloom Markdown(src)を読んで IR にする。file は誤りの
// 場所に添える名前。記法の誤りは行と列つきの Issue の一覧で返し、そのとき
// deck は nil。記法は通ったのに IR として読めなかったときは err を返す。
func ParseMarkdown(file string, src []byte) (deck *IRDeck, issues []Issue, err error) {
	ir, issues := MarkdownToIR(file, src)
	if len(issues) > 0 {
		return nil, issues, nil
	}
	d, err := ParseIR(ir)
	if err != nil {
		return nil, nil, err
	}
	return d, nil, nil
}

// ReadDeck は file の拡張子で読み方を選ぶ。.md なら Boxloom Markdown
// (記法の誤りは MarkdownErrors)、それ以外は IR(JSON。ReadIR)。
func ReadDeck(file string, data []byte) (*IRDeck, error) {
	if !isMarkdownPath(file) {
		return ReadIR(file, data)
	}
	d, issues, err := ParseMarkdown(file, data)
	if len(issues) > 0 {
		return nil, MarkdownErrors(issues)
	}
	return d, err
}

// ErrUnknownTemplate はテンプレートの名前が同梱のものに無いとき(errors.Is で
// 見分ける)。
var ErrUnknownTemplate = errors.New("unknown template")

type unknownTemplateError struct{ name string }

func (e unknownTemplateError) Error() string {
	return fmt.Sprintf("unknown template %q (known: %s)", e.name, strings.Join(TemplateNames, ", "))
}

func (e unknownTemplateError) Is(target error) bool { return target == ErrUnknownTemplate }

// Templates は同梱したテンプレートの名前(並べたもの。既定は light)。
func Templates() []string {
	return append([]string(nil), TemplateNames...)
}

// BuildOptions は Build の選択。
type BuildOptions struct {
	// Template はテンプレートの名前。空なら IR の template、それも無ければ light。
	Template string
	// Assets は画像(image・gallery の src)を読む fs.FS。根は文書のフォルダで、
	// src は文書からの相対パス(ライブラリはファイルを直接は読まない)。コマンドは
	// os.DirFS(文書のフォルダ) を渡す。nil なら画像を使う文書は組めない。
	Assets fs.FS
}

// BuildResult は Build の結果。
type BuildResult struct {
	// Template は使ったテンプレートの名前。
	Template string
	// Slides は書いたスライドの枚数。
	Slides int
	// Warnings は組み立ての警告("slide 2: [FILL-RATIO WARNING] ..." の形)。
	// 書き出しは止めない。Issue の形で欲しいときは Check を使う。
	Warnings []string
}

// Build は deck をテンプレートで組み、PPTX を w に書く。テンプレートが無ければ
// ErrUnknownTemplate を包んだ誤りを、組めなければ IR の中の場所を添えた誤りを
// 返す(どちらも w には何も書かない)。
func Build(w io.Writer, deck *IRDeck, opt BuildOptions) (*BuildResult, error) {
	defer useFonts(activeFonts)() // 組み立てが替えた字体を、終わったら戻す
	name := deck.Template
	if opt.Template != "" {
		name = opt.Template
	}
	t, err := LookupTemplate(name)
	if err != nil {
		return nil, err
	}
	t.SetAssets(opt.Assets)
	pages, warnings, err := t.RenderPages(deck)
	if err != nil {
		return nil, err
	}
	title := ""
	if deck.Meta != nil {
		title = deck.Meta.Title.Plain()
	}
	if err := WriteDeck(w, pages, title); err != nil {
		return nil, err
	}
	return &BuildResult{Template: t.Name, Slides: len(deck.Slides), Warnings: warnings}, nil
}

// Schema は `boxloom schema` の JSON(AI に渡す一覧。docs/schema-command.md)を
// 返す。template が空なら全部のテンプレート、名前ならそのテンプレートが
// 名乗るものだけ。
func Schema(template string) ([]byte, error) {
	names := TemplateNames
	if template != "" {
		names = []string{template}
	}
	out, err := BuildSchemaOutput(names)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(out, "", "  ")
}
