// libuse は Boxloom をライブラリとして使う例(別のモジュールから import する)。
// Boxloom Markdown か IR(JSON)を読んで、PPTX を書く。
//
//	cd boxloom/examples/libuse
//	go run . -template dark ../../../examples/md/demo-deck.bx.md out.pptx
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"boxloom"
)

func main() {
	tmpl := flag.String("template", "", "テンプレートの名前(省くと文書の template)")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: libuse [-template NAME] deck.bx.md|deck.json out.pptx")
		os.Exit(2)
	}
	in, out := flag.Arg(0), flag.Arg(1)
	if err := run(in, out, *tmpl); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(in, out, tmpl string) error {
	src, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	deck, err := boxloom.ReadDeck(in, src)
	var issues boxloom.MarkdownErrors
	if errors.As(err, &issues) {
		for _, is := range issues {
			fmt.Fprintln(os.Stderr, is.String())
		}
		return fmt.Errorf("%d error(s) in %s", len(issues), in)
	}
	if err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	res, err := boxloom.Build(f, deck, boxloom.BuildOptions{Template: tmpl})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Println("warning:", w)
	}
	fmt.Printf("wrote %s (%d slides, template %s)\n", out, res.Slides, res.Template)
	return nil
}
