// Boxloom Markdown を受け取るコマンド。
//
//	go run . parse [-format json] deck.bx.md   # IR(JSON)を標準出力に書く
//	go run . build deck.bx.md                  # .md もそのまま PPTX にする(readDeck)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// isMarkdownPath は Boxloom Markdown として読むファイルか(.md、.bx.md を含む)。
func isMarkdownPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
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

// MarkdownErrors は Markdown の誤りの一覧(error として返すため)。
type MarkdownErrors []Issue

func (es MarkdownErrors) Error() string {
	var lines []string
	for _, e := range es {
		lines = append(lines, e.String())
	}
	return fmt.Sprintf("%d error(s)\n%s", len(es), strings.Join(lines, "\n"))
}

// ReadMarkdown は Boxloom Markdown のファイルを IR として読む。
func ReadMarkdown(path string) (*IRDeck, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ir, errs := ParseMarkdown(src, path)
	if len(errs) > 0 {
		return nil, MarkdownErrors(errs)
	}
	return ParseIR(ir)
}

// readDeck は拡張子で IR(JSON)か Boxloom Markdown かを選んで読む。
func readDeck(path string) (*IRDeck, error) {
	if isMarkdownPath(path) {
		return ReadMarkdown(path)
	}
	return ReadIR(path)
}

// runParse は「boxloom parse」。
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
	ir, errs := ParseMarkdown(src, in)
	if len(errs) == 0 {
		stdout.Write(ir)
		return 0
	}
	if *format == "json" {
		b, _ := json.MarshalIndent(struct {
			OK     bool    `json:"ok"`
			Errors []Issue `json:"errors"`
		}{false, errs}, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 1
	}
	for _, e := range errs {
		fmt.Fprintln(stderr, e.String())
	}
	return 1
}
