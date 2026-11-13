// ライブラリの入口(docs/library.md)。コマンド(cmd/boxloom)はこれだけを使う。
//
// 流れは Markdown か IR(JSON・YAML)を読む(ParseMarkdown・ReadIR・
// ParseYAMLDeck・ReadDeck)→
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

// MarkdownErrors は Markdown と YAML の deck の記法の誤りの一覧(行と列つき。
// error として返すため)。ReadDeck が返す。
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
// (記法の誤りは MarkdownErrors)、.yaml・.yml なら YAML の deck(IR の写し。
// 誤りは行と列つきの MarkdownErrors)、それ以外は IR(JSON。ReadIR)。
func ReadDeck(file string, data []byte) (*IRDeck, error) {
	if isYAMLPath(file) {
		d, issues := ParseYAMLDeck(file, data)
		if len(issues) > 0 {
			return nil, MarkdownErrors(issues)
		}
		return d, nil
	}
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
	// Values は外から読んだテンプレートの値の表(ParseTemplate)。nil でなければ
	// Template と IR の template より優先する。
	Values *Template
	// AllowOverflow は、はみ出し(layout-overflow)があっても PPTX を書く
	// (警告は Warnings に残る)。false なら、はみ出しが1件でもあれば書かずに
	// *OverflowError を返す。
	AllowOverflow bool
	// Fit は、はみ出す本文のスライドを値の表の梯子で詰めて組む(既定は縮めない)。
	// 今の密度から1段ずつ(normal は dense の組 → fit.shrink[0] → …、dense は
	// fit.shrink[0] → …)組み直して測り、最初に収まった段で組む。最後の段でも
	// 収まらなければ今までどおり layout-overflow。fill-ratio が下回る normal の本文の
	// スライドは fit.grow を上り、はみ出さない最も上の段で組む(目標に届いたらそこで
	// 止める。fit_grow.go)。詰めた・広げたスライドは Info に書く。
	// 「収まった」は layout-overflow も図の崩れ(figure-text-overflow・subsystem-min・
	// column-floor)も無いこと(#145)。最後の段でも崩れが残れば、AllowOverflow でない限り
	// 書かずに *OverflowError を返す(Fit でなければ図の崩れは警告のまま)。
	// IR は変えない。false なら出力は Fit の無いときと同じ。
	Fit bool

	// noOverflowMark は試験用:AllowOverflow でもはみ出しの印(overflow_mark.go)を
	// 描かない。印は配置の段の木に無いので、木を読み直した PPTX と比べるときに使う。
	noOverflowMark bool
}

// ErrLayoutOverflow は、はみ出しがあって Build が PPTX を書かなかったことを表す
// (errors.Is で見る。中身は *OverflowError)。
var ErrLayoutOverflow = errors.New("layout-overflow: the deck does not fit")

// OverflowError は Build がはみ出しで止まったときの誤り。Issues は
// layout-overflow の全件と、Fit のときは図の崩れ(figure-text-overflow・subsystem-min・
// column-floor。全部の段でも直らなかったもの、#145)の全件(check の警告と同じ path・message・hint。File と行は
// 空なので、呼ぶ側が文書の名前を添える)、Warnings はそれ以外も含めた組み立ての
// 警告の全部(BuildResult.Warnings と同じ形)。
type OverflowError struct {
	Issues   []Issue
	Warnings []string
}

func (e *OverflowError) Error() string {
	var b strings.Builder
	figures := 0
	for _, is := range e.Issues {
		if is.Code != codeLayoutOverflow {
			figures++
		}
	}
	if figures == 0 {
		fmt.Fprintf(&b, "layout-overflow: %d block(s) do not fit the slide; PPTX not written (follow each hint, or build with AllowOverflow / -allow-overflow)", len(e.Issues))
	} else {
		fmt.Fprintf(&b, "layout-overflow: %d block(s) do not fit the slide and %d figure(s) break even at the last -fit step; PPTX not written (follow each hint, or build with AllowOverflow / -allow-overflow)", len(e.Issues)-figures, figures)
	}
	for _, is := range e.Issues {
		b.WriteString("\n  " + is.String())
	}
	return b.String()
}

func (e *OverflowError) Unwrap() error { return ErrLayoutOverflow }

// BuildResult は Build の結果。
type BuildResult struct {
	// Template は使ったテンプレートの名前。
	Template string
	// Slides は書いたスライドの枚数。
	Slides int
	// Warnings は組み立ての警告("slide 2: [FILL-RATIO WARNING] ..." の形)。
	// はみ出し(layout-overflow)のほかは書き出しを止めない(はみ出しは
	// AllowOverflow のときだけここに残る)。Issue の形で欲しいときは Check を使う。
	Warnings []string
	// Info は知らせ(-fit で詰めた・広げたスライド。"slide 3: [FIT-APPLIED INFO] ..." の形。
	// check の Info の fit-applied と同じ文)。無ければ nil。
	Info []string
}

// Build は deck をテンプレートで組み、PPTX を w に書く。テンプレートが無ければ
// ErrUnknownTemplate を包んだ誤りを、組めなければ IR の中の場所を添えた誤りを
// 返す(どちらも w には何も書かない)。はみ出し(layout-overflow)が1件でもあれば、
// AllowOverflow でない限り書かずに *OverflowError(ErrLayoutOverflow)を返す。
func Build(w io.Writer, deck *IRDeck, opt BuildOptions) (*BuildResult, error) {
	defer useFonts(activeFonts)() // 組み立てが替えた字体を、終わったら戻す
	t := opt.Values
	if t == nil {
		name := deck.Template
		if opt.Template != "" {
			name = opt.Template
		}
		var err error
		if t, err = LookupTemplate(name); err != nil {
			return nil, err
		}
	}
	t.SetAssets(opt.Assets)
	pages, warnings, caps, err := t.renderPages(deck, opt.Fit)
	if err != nil {
		return nil, err
	}
	// -fit のときは図の崩れ(figure-text-overflow・subsystem-min・column-floor)も、全部の
	// 段で直らなかった収まらなさとして誤りにする(#145。-fit なしでは警告のまま)。
	fitBroken := false
	if opt.Fit {
		for _, w := range warnings {
			fitBroken = fitBroken || fitBrokenCode(w) != ""
		}
	}
	if !opt.AllowOverflow && (hasOverflowWarning(warnings) || fitBroken) {
		designated := opt.Values != nil || opt.Template != "" || deck.Template != ""
		issues, err := t.layoutIssues("", deck, warnings, caps, designated)
		if err != nil {
			return nil, err
		}
		oe := &OverflowError{Warnings: warnings}
		for _, is := range issues {
			if is.Code == codeLayoutOverflow || opt.Fit && fitBrokenCodes[is.Code] {
				oe.Issues = append(oe.Issues, is)
			}
		}
		return nil, oe
	}
	if opt.AllowOverflow && !opt.noOverflowMark {
		// はみ出したスライドにだけ、本文の領域の下端の印を描く(overflow_mark.go)。
		for _, p := range pages {
			if p.overflow != nil {
				p.addOverflowMark(p.overflow)
			}
		}
	}
	title := ""
	if deck.Meta != nil {
		title = deck.Meta.Title.Plain()
	}
	if err := WriteDeck(w, pages, title); err != nil {
		return nil, err
	}
	res := &BuildResult{Template: t.Name, Slides: len(pages)}
	for _, w := range warnings {
		if strings.Contains(w, "[FIT-APPLIED INFO]") || strings.Contains(w, "[FIT-SPLIT INFO]") {
			res.Info = append(res.Info, w)
			continue
		}
		res.Warnings = append(res.Warnings, w)
	}
	return res, nil
}

// hasOverflowWarning は組み立ての警告にはみ出しがあるか。
func hasOverflowWarning(warnings []string) bool {
	for _, w := range warnings {
		if strings.Contains(w, "[LAYOUT-OVERFLOW WARNING]") {
			return true
		}
	}
	return false
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

// SchemaTemplate は Schema の、外から読んだテンプレート(ParseTemplate)版。
// そのテンプレートが名乗るものだけを出す。
func SchemaTemplate(t *Template) ([]byte, error) {
	out, err := buildSchemaOutput([]*Template{t})
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(out, "", "  ")
}
