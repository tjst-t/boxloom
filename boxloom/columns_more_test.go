package boxloom

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 列の中に置ける型を広げた(#117):pairs・phases・matrix・orgchart・gallery を
// role=main の箇条の横に置く。文はすべて架空。

// columnsMoreBodies は型ごとの、列の中に書く Markdown(:::column の中身)。
var columnsMoreBodies = map[string]string{
	"pairs": `:::::pairs 分かったこと | 打ち手
:::subject
返却は閉館後に集まる
:::
:::response
返却箱を2つに増やす
:::
::::subject
:::quote 利用者 B
返したのに延滞の知らせが来た
:::
::::
:::response
開館と同時に読み取る
:::
:::::
`,
	"phases": "```phases\nsteps:\n  - title: 現状を知る\n    items:\n      - title: 乗降の調べ\n      - title: 聞き取り\n" +
		"  - title: 案を作る\n    items:\n      - title: 路線の案\n  - title: まとめる\n    items:\n      - title: 報告書\n```\n",
	"matrix": "```matrix\ncolumns: [案 A, 案 B]\nrows:\n  - label: 費用\n    cells:\n      - {status: support, text: 追加なし}\n" +
		"      - {status: conditional, text: 年に200万円}\n  - label: 期限\n    cells:\n      - {status: danger, text: 2028年}\n" +
		"      - {status: support, text: 2034年}\n```\n",
	"orgchart": "```orgchart\nroot: {title: 責任者}\nunits:\n  - title: 基盤\n    members:\n      - {title: 認証}\n  - title: 移行\n```\n",
	"gallery":  "![一つ目の画面](wide.png) ![二つ目の画面](img/tall.png) ![三つ目の画面](wide.png)\n",
}

// columnsMoreMD は figure の列と role=main の箇条の列を並べた1枚の deck。pairs は
// 中に3段の囲みを持つので、外の囲みのコロンを増やす。
func columnsMoreMD(tmpl, typ string) string {
	cols, col := "::::", ":::"
	if typ == "pairs" {
		cols, col = ":::::::", "::::::"
	}
	return fmt.Sprintf("---\ntitle: T\ntemplate: %s\n---\n\n# 図と要点\n\n%scolumns\n%scolumn 要点 {role=main}\n"+
		"- 返却の半分は閉館後に集まる\n- 雨の日は本が濡れる\n- 案内板が見えにくい\n%s\n%scolumn\n%s%s\n%s\n",
		tmpl, cols, col, col, col, columnsMoreBodies[typ], col, cols)
}

// 5つの型それぞれを role=main の箇条の横に置いた deck が、3つのテンプレートで
// check の誤り0・はみ出しや図の警告なしで組め、PPTX に書け、列の中の箱と図の
// 矩形がすべてまとまりの矩形の中に収まる。
func TestColumnsHoldMoreTypes(t *testing.T) {
	assets := testAssets(t)
	for _, typ := range []string{"pairs", "phases", "matrix", "orgchart", "gallery"} {
		for _, tmpl := range []string{"report", "dark", "light"} {
			name := typ + "/" + tmpl
			src := []byte(columnsMoreMD(tmpl, typ))
			dir := t.TempDir()
			for p, f := range assets {
				if strings.HasSuffix(p, ".png") {
					_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
					if err := os.WriteFile(filepath.Join(dir, p), f.Data, 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			path := filepath.Join(dir, "c.bx.md")
			if err := os.WriteFile(path, src, 0o644); err != nil {
				t.Fatal(err)
			}
			errs, warns, _ := CheckFileCapacity(path, "")
			if len(errs) > 0 {
				t.Fatalf("%s: errors %+v", name, errs)
			}
			for _, w := range warns {
				switch w.Code {
				case "fill-ratio", "spare-width":
				default:
					t.Errorf("%s: unexpected warning %s: %s", name, w.Code, w.Message)
				}
			}
			d, err := ReadDeck(path, src)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := Build(&buf, d, BuildOptions{Assets: assets}); err != nil {
				t.Fatalf("%s: build: %v", name, err)
			}
			dump, err := Layout(path, src, BuildOptions{Assets: assets})
			if err != nil {
				t.Fatal(err)
			}
			const colPath = "/slides/1/body/0/columns/1"
			col := findPath(dump.Slides[1].Root, colPath)
			if col == nil {
				t.Fatalf("%s: no %s in the layout", name, colPath)
			}
			n := 0
			walkLayout(dump.Slides[1].Root, func(k *LayoutNode) {
				if !strings.HasPrefix(k.Path, colPath+"/body/") {
					return
				}
				n++
				if !boxWithin(k.Box, col.Box) {
					t.Errorf("%s: %s %s box %+v is outside the column %+v", name, k.Kind, k.Path, k.Box, col.Box)
				}
				if k.Subsystem != nil && !boxWithin(k.Subsystem.Draw, col.Box) {
					t.Errorf("%s: %s draws %+v outside the column %+v", name, k.Subsystem.Name, k.Subsystem.Draw, col.Box)
				}
			})
			if n == 0 {
				t.Errorf("%s: nothing laid out in the column", name)
			}
		}
	}
}

func walkLayout(n *LayoutNode, f func(*LayoutNode)) {
	f(n)
	for _, c := range n.Children {
		walkLayout(c, f)
	}
}

func boxWithin(in, out LayoutBox) bool {
	const eps = 0.01
	return in.X >= out.X-eps && in.Y >= out.Y-eps && in.X+in.Width <= out.X+out.Width+eps && in.Y+in.Height <= out.Y+out.Height+eps
}
