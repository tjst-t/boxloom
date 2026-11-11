// Command boxloom は Boxloom のコマンド。中身はライブラリ(boxloom)の公開した
// 入口(docs/library.md)だけを使い、ここではフラグ・ファイルの読み書き・
// 終了コードを受け持つ。
//
//	boxloom build [-o out.pptx] [-template dark|light] deck.json|deck.bx.md
//	boxloom check [-format text|json] [-capacity] [-template dark|light] [-strict] deck.json|deck.bx.md ...
//	boxloom parse [-format text|json] deck.bx.md
//	boxloom schema [-template NAME]
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

// readDeck は path を読み、拡張子で IR(JSON)か Boxloom Markdown かを選ぶ。
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
	for _, ext := range []string{".bx.md", ".md", ".json"} {
		if strings.HasSuffix(strings.ToLower(b), ext) {
			return b[:len(b)-len(ext)]
		}
	}
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// runBuild は「boxloom build」: IR(JSON)か Boxloom Markdown を読んで pptx を書く。
// テンプレートは -template > IR の template > light の順で決まる。
func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "書き出す pptx(省くと out/<IR の名前>.pptx)")
	tmpl := fs.String("template", "", "テンプレートの名前(省くと IR の template)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boxloom build [-o out.pptx] [-template dark|light] deck.json|deck.bx.md")
		return 2
	}
	in := fs.Arg(0)
	deck, err := readDeck(in)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	var buf bytes.Buffer
	res, err := boxloom.Build(&buf, deck, boxloom.BuildOptions{Template: *tmpl, Assets: deckAssets(in)})
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
	tmpl := fs.String("template", "", "テンプレートの名前(省くと IR の template)")
	strict := fs.Bool("strict", false, "警告も誤りとして扱う(終了コード 1)")
	format := fs.String("format", "text", "出し方: text か json")
	showCap := fs.Bool("capacity", false, "text の出し方でも容量(本文の残り)を出す(json では常に出る)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "usage: boxloom check [-format text|json] [-capacity] [-template dark|light] [-strict] deck.json|deck.bx.md ...")
		return 2
	}
	res := boxloom.CheckResult{OK: true, Errors: []boxloom.Issue{}, Warnings: []boxloom.Issue{}, Capacity: []boxloom.SlideCapacity{}}
	for _, f := range fs.Args() {
		b, err := os.ReadFile(f)
		if err != nil {
			res.OK = false
			res.Errors = append(res.Errors, boxloom.Issue{File: f, Message: err.Error(), Code: "read"})
			continue
		}
		r := boxloom.Check(f, b, boxloom.CheckOptions{Template: *tmpl, Strict: *strict, Assets: deckAssets(f)})
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

// runParse は「boxloom parse」: Boxloom Markdown を IR(JSON)にして標準出力に書く。
func runParse(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "誤りの出し方: text か json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || (*format != "text" && *format != "json") {
		fmt.Fprintln(stderr, "usage: boxloom parse [-format text|json] deck.bx.md")
		return 2
	}
	in := fs.Arg(0)
	src, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	ir, errs := boxloom.MarkdownToIR(in, src)
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

// runSchema は「boxloom schema」: AI に渡す一覧(docs/schema-command.md)。
func runSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmpl := fs.String("template", "", "このテンプレートが名乗るものだけを出す(省くと全部: "+strings.Join(boxloom.Templates(), ", ")+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: boxloom schema [-template NAME]")
		return 2
	}
	b, err := boxloom.Schema(*tmpl)
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
		fmt.Fprintln(stderr, "usage: boxloom fonts [-template 名前] [-against \"欧文/和文;…\"] deck.json|deck.bx.md …")
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
	fc.Template, fc.Verbose = *tmpl, *verbose
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
