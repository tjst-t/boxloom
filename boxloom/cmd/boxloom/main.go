// Command boxloom は Boxloom のコマンド。中身はライブラリ(boxloom)の公開した
// 入口(docs/library.md)だけを使い、ここではフラグ・ファイルの読み書き・
// 終了コードを受け持つ。
//
//	boxloom build [-o out.pptx] [-template dark|light|report|./my.json] deck.json|deck.yaml|deck.bx.md
//	boxloom check [-format text|json] [-capacity] [-template dark|light|report|./my.json] [-strict] deck.json|deck.yaml|deck.bx.md ...
//	boxloom parse [-format text|json] deck.bx.md|deck.yaml
//	boxloom schema [-template NAME]
//	boxloom layout [-template dark|light|report|./my.json] deck.json|deck.bx.md|deck.yaml   # 配置の段の木(JSON)
//	boxloom layout-build [-o out.pptx] [-deck deck] [-template …] tree.json  # 開発者・試験用:木を読み直して組む
//	boxloom templates            # 同梱したテンプレートの名前(1行に1つ)
//	boxloom mcp                  # stdio の MCP サーバ(道具 schema・check・build。docs/mcp.md)
//	boxloom fonts [-template 名前] [-against "欧文/和文;…"] [-v] deck… | -list
//	boxloom                      # 従来の Go で組んだデモ(out/boxloom_dark.pptx・out/boxloom_light.pptx)
//
// リポジトリの boxloom/ からは `go run ./cmd/boxloom <cmd>` で動かす。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"boxloom"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "build":
			return runBuild(args[1:], stdout, stderr)
		case "check":
			return runCheck(args[1:], stdout, stderr)
		case "fonts":
			return runFonts(args[1:], stdout, stderr)
		case "schema":
			return runSchema(args[1:], stdout, stderr)
		case "parse":
			return runParse(args[1:], stdout, stderr)
		case "layout":
			return runLayout(args[1:], stdout, stderr)
		case "layout-build":
			return runLayoutBuild(args[1:], stdout, stderr)
		case "templates":
			return runTemplates(args[1:], stdout, stderr)
		case "mcp":
			return runMCP(args[1:], os.Stdin, stdout, stderr)
		}
	}
	return runDemo(stdout, stderr)
}

// runDemo は引数なし: 従来の Go で組んだデモを out/ に dark と light で書く。
func runDemo(stdout, stderr io.Writer) int {
	if err := os.MkdirAll("out", 0o755); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	for _, theme := range []string{"dark", "light"} {
		outPath := "out/boxloom_" + theme + ".pptx"
		var buf bytes.Buffer
		res, err := boxloom.WriteDemo(&buf, theme)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		if res.FillWarning != "" {
			fmt.Fprintf(stdout, "[%s] content fill ratio: %.1f%%  %s\n", outPath, res.FillRatio*100, res.FillWarning)
		} else {
			fmt.Fprintf(stdout, "[%s] content fill ratio: %.1f%%  OK\n", outPath, res.FillRatio*100)
		}
		for _, w := range res.Warnings {
			fmt.Fprintf(stdout, "[%s] %s\n", outPath, w)
		}
		if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		fmt.Fprintln(stdout, "saved:", outPath)
	}
	return 0
}

// templateFlag は -template の値を読む。ファイルのパス(.json で終わるか、
// パスの区切りを含む)なら値の表を読んで検査し、名前ならそのまま返す。
// 誤りは5章の形の1件で返す。
func templateFlag(v string) (name string, values *boxloom.Template, issue *boxloom.Issue) {
	if !boxloom.IsTemplateFile(v) {
		return v, nil, nil
	}
	b, err := os.ReadFile(v)
	if err != nil {
		return "", nil, &boxloom.Issue{File: v, Message: err.Error(), Code: "read"}
	}
	t, err := boxloom.ParseTemplate(v, b)
	var te *boxloom.TemplateError
	if errors.As(err, &te) {
		return "", nil, &te.Issue
	}
	if err != nil {
		return "", nil, &boxloom.Issue{File: v, Message: err.Error(), Code: "template-values"}
	}
	return "", t, nil
}

// readDeck は path を読み、拡張子で IR(JSON か YAML)か Boxloom Markdown かを選ぶ。
func readDeck(path string) (*boxloom.IRDeck, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return boxloom.ReadDeck(path, b)
}

// deckAssets は文書の画像を読む口:文書のフォルダ(src はそこからの相対パス)。
func deckAssets(deckPath string) fs.FS {
	return os.DirFS(filepath.Dir(deckPath))
}

// deckBaseName は出力の名前の元(deck.bx.md → deck)。
func deckBaseName(path string) string {
	b := filepath.Base(path)
	for _, ext := range []string{".bx.md", ".md", ".json", ".yaml", ".yml"} {
		if strings.HasSuffix(strings.ToLower(b), ext) {
			return b[:len(b)-len(ext)]
		}
	}
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// runBuild は「boxloom build」: IR(JSON か YAML)か Boxloom Markdown を読んで pptx を書く。
// テンプレートは -template > IR の template > light の順で決まる。
func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "書き出す pptx(省くと out/<IR の名前>.pptx)")
	tmpl := fs.String("template", "", "テンプレートの名前か値の表のファイル(./my.json。省くと IR の template)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boxloom build [-o out.pptx] [-template dark|light|report|./my.json] deck.json|deck.yaml|deck.bx.md")
		return 2
	}
	in := fs.Arg(0)
	tname, tvalues, tissue := templateFlag(*tmpl)
	if tissue != nil {
		fmt.Fprintln(stderr, "error:", tissue.String())
		return 1
	}
	deck, err := readDeck(in)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	var buf bytes.Buffer
	res, err := boxloom.Build(&buf, deck, boxloom.BuildOptions{Template: tname, Values: tvalues, Assets: deckAssets(in)})
	if errors.Is(err, boxloom.ErrUnknownTemplate) {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if *out == "" {
		*out = filepath.Join("out", deckBaseName(in)+".pptx")
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if err == nil {
		err = os.WriteFile(*out, buf.Bytes(), 0o644)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %s: %v\n", in, err)
		return 1
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stdout, w)
	}
	fmt.Fprintf(stdout, "saved: %s (%d slides, template %s)\n", *out, res.Slides, res.Template)
	return 0
}

// runCheck は「boxloom check」。終了コードは 0=誤りなし、1=誤りあり(-strict
// なら警告もこちら)、2=使い方の誤り。
func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmpl := fs.String("template", "", "テンプレートの名前か値の表のファイル(./my.json。省くと IR の template)")
	strict := fs.Bool("strict", false, "警告も誤りとして扱う(終了コード 1)")
	format := fs.String("format", "text", "出し方: text か json")
	showCap := fs.Bool("capacity", false, "text の出し方でも容量(本文の残り)を出す(json では常に出る)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "usage: boxloom check [-format text|json] [-capacity] [-template dark|light|report|./my.json] [-strict] deck.json|deck.yaml|deck.bx.md ...")
		return 2
	}
	res := boxloom.CheckResult{OK: true, Errors: []boxloom.Issue{}, Warnings: []boxloom.Issue{}, Capacity: []boxloom.SlideCapacity{}}
	tname, tvalues, tissue := templateFlag(*tmpl)
	files := fs.Args()
	if tissue != nil {
		// 値の表が読めなければ、文書は検査しない(どのテンプレートで見るか決まらない)。
		res.OK, files = false, nil
		res.Errors = append(res.Errors, *tissue)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			res.OK = false
			res.Errors = append(res.Errors, boxloom.Issue{File: f, Message: err.Error(), Code: "read"})
			continue
		}
		r := boxloom.Check(f, b, boxloom.CheckOptions{Template: tname, Values: tvalues, Strict: *strict, Assets: deckAssets(f)})
		res.OK = res.OK && r.OK
		res.Errors = append(res.Errors, r.Errors...)
		res.Warnings = append(res.Warnings, r.Warnings...)
		res.Capacity = append(res.Capacity, r.Capacity...)
	}
	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		for _, e := range res.Errors {
			fmt.Fprintln(stdout, "error:", e.String())
		}
		for _, w := range res.Warnings {
			fmt.Fprintln(stdout, "warning:", w.String())
		}
		if *showCap {
			for _, c := range res.Capacity {
				fmt.Fprintln(stdout, c.String())
			}
		}
		fmt.Fprintf(stdout, "%d error(s), %d warning(s)\n", len(res.Errors), len(res.Warnings))
	}
	if !res.OK {
		return 1
	}
	return 0
}

// runParse は「boxloom parse」: Boxloom Markdown か YAML の deck(.yaml・.yml)を
// IR(JSON)にして標準出力に書く。
func runParse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "誤りの出し方: text か json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "usage: boxloom parse [-format text|json] deck.bx.md|deck.yaml")
		return 2
	}
	in := fs.Arg(0)
	src, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	var ir []byte
	var errs []boxloom.Issue
	if ext := strings.ToLower(filepath.Ext(in)); ext == ".yaml" || ext == ".yml" {
		ir, errs = boxloom.YAMLToIR(in, src)
	} else {
		ir, errs = boxloom.MarkdownToIR(in, src)
	}
	if len(errs) == 0 {
		stdout.Write(ir)
		return 0
	}
	if *format == "json" {
		b, _ := json.MarshalIndent(struct {
			OK     bool            `json:"ok"`
			Errors []boxloom.Issue `json:"errors"`
		}{false, errs}, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 1
	}
	for _, e := range errs {
		fmt.Fprintln(stderr, e.String())
	}
	return 1
}

// runLayout は「boxloom layout」: 配置の段の木(箱・印・値・決まった座標)を JSON で
// 標準出力に書く(docs/build.md の「配置の段の木を見る」)。作者の入口は出すだけで、
// この形を読み直す口は開発者・試験用の「boxloom layout-build」(runLayoutBuild、
// docs/build.md の「配置の段の木を読み直す」)だけ。
func runLayout(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("layout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmpl := fs.String("template", "", "テンプレートの名前か値の表のファイル(./my.json。省くと IR の template)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boxloom layout [-template dark|light|report|./my.json] deck.json|deck.bx.md|deck.yaml")
		return 2
	}
	in := fs.Arg(0)
	tname, tvalues, tissue := templateFlag(*tmpl)
	if tissue != nil {
		fmt.Fprintln(stderr, "error:", tissue.String())
		return 1
	}
	src, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	dump, err := boxloom.Layout(in, src, boxloom.BuildOptions{Template: tname, Values: tvalues, Assets: deckAssets(in)})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	dump.File = in
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(dump); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

// runLayoutBuild は「boxloom layout-build」:layout が出した木を読み直して PPTX に
// 組む(開発者・試験用。作者の入口ではないので schema・mcp には出さない)。
// -deck を渡すとサブシステムの中身・ノート・題をその deck から取る。
func runLayoutBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("layout-build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "書き出す pptx(省くと out/<木の名前>.pptx)")
	deckPath := fs.String("deck", "", "サブシステムの中身・ノート・題を取る deck(省くとサブシステムは何も描かない)")
	tmpl := fs.String("template", "", "テンプレートの名前か値の表のファイル(省くと木の template)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boxloom layout-build [-o out.pptx] [-deck deck.bx.md] [-template dark|light|report|./my.json] tree.json")
		return 2
	}
	in := fs.Arg(0)
	tname, tvalues, tissue := templateFlag(*tmpl)
	if tissue != nil {
		fmt.Fprintln(stderr, "error:", tissue.String())
		return 1
	}
	src, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	opt := boxloom.LayoutReadOptions{Template: tname, Values: tvalues}
	if *deckPath != "" {
		deck, err := readDeck(*deckPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		opt.Deck, opt.Assets = deck, deckAssets(*deckPath)
	}
	var buf bytes.Buffer
	res, err := boxloom.BuildLayout(&buf, in, src, opt)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if *out == "" {
		*out = filepath.Join("out", deckBaseName(in)+".pptx")
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stdout, w)
	}
	fmt.Fprintf(stdout, "saved: %s (%d slides, template %s)\n", *out, res.Slides, res.Template)
	return 0
}

// runSchema は「boxloom schema」: AI に渡す一覧(docs/schema-command.md)。
// runTemplates は同梱したテンプレートの名前を1行に1つ書く(既定は light)。
func runTemplates(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: boxloom templates")
		return 2
	}
	for _, n := range boxloom.Templates() {
		fmt.Fprintln(stdout, n)
	}
	return 0
}

func runSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmpl := fs.String("template", "", "このテンプレート(名前か値の表のファイル)が名乗るものだけを出す(省くと全部: "+strings.Join(boxloom.Templates(), ", ")+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: boxloom schema [-template NAME|./my.json]")
		return 2
	}
	tname, tvalues, tissue := templateFlag(*tmpl)
	if tissue != nil {
		fmt.Fprintln(stderr, "boxloom schema:", tissue.String())
		return 1
	}
	var b []byte
	var err error
	if tvalues != nil {
		b, err = boxloom.SchemaTemplate(tvalues)
	} else {
		b, err = boxloom.Schema(tname)
	}
	if err != nil {
		fmt.Fprintln(stderr, "boxloom schema:", err)
		return 1
	}
	stdout.Write(b)
	fmt.Fprintln(stdout)
	return 0
}

// runFonts は「boxloom fonts」: 字体が置き換わったときのはみ出しを測る(docs/fonts.md)。
func runFonts(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fonts", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmpl := fs.String("template", "", "テンプレート(省けば文書の template か light)")
	against := fs.String("against", "", `試す置き換わり方。"欧文/和文" か "字体" を ; で区切る(省けば既定の5つ)`)
	list := fs.Bool("list", false, "寸法を持つ字体の一覧を出して終わる")
	verbose := fs.Bool("v", false, "はみ出す行を1行ずつ出す")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: boxloom fonts [-template 名前] [-against \"欧文/和文;…\"] deck.json|deck.yaml|deck.bx.md …")
		fmt.Fprintln(stderr, "       boxloom fonts -list")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *list {
		boxloom.WriteFontList(stdout)
		return 0
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	fc, err := boxloom.NewFontCheck(*against)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	tname, tvalues, tissue := templateFlag(*tmpl)
	if tissue != nil {
		fmt.Fprintln(stderr, "error:", tissue.String())
		return 1
	}
	fc.Template, fc.Values, fc.Verbose = tname, tvalues, *verbose
	for _, in := range fs.Args() {
		d, err := readDeck(in)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		fc.Assets = deckAssets(in)
		if err := fc.Deck(stdout, in, d); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
	}
	fc.Summary(stdout)
	return 0
}
