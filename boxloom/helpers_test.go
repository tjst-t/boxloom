package boxloom

import (
	"os"
	"path/filepath"
)

// 試験だけが使う、ファイルを読み書きする手伝い(ライブラリ自身はファイルを
// 扱わない)。

// CheckFile は path を読んで Check の誤りと警告を返す。
func CheckFile(path, templateOverride string) (errs, warns []Issue) {
	errs, warns, _ = CheckFileCapacity(path, templateOverride)
	return errs, warns
}

// CheckFileCapacity は CheckFile に容量を足したもの。
func CheckFileCapacity(path, templateOverride string) (errs, warns []Issue, caps []SlideCapacity) {
	b, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{File: path, Message: err.Error(), Code: "read"}}, nil, nil
	}
	return checkCapacity(path, b, templateOverride, os.DirFS(filepath.Dir(path)))
}

// readDeck は path を拡張子で選んで読む。
func readDeck(path string) (*IRDeck, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadDeck(path, b)
}

// readIRFile は path を IR(JSON)として読む。
func readIRFile(path string) (*IRDeck, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadIR(path, b)
}

// renderDeckToFile は t で d を組んで path に書く(RenderDeck のファイル版)。
func renderDeckToFile(t *Template, d *IRDeck, path string) ([]string, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	ws, err := t.RenderDeck(d, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return ws, err
}
