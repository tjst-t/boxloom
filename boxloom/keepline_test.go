package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// Keep の塊は途中で折らない:和文の単位(「億ドル」)も数(「12,345,678」)も
// 途中では折れず、折れるのは塊と塊の境目(数と単位の間)だけ。入りきらない塊は
// 1行のまま残る。Keep でない塊はこれまでどおり折れる。
func TestWrapRunsKeep(t *testing.T) {
	keep := []TextRun{{Text: "2,859", Keep: true}, {Text: " 億ドル", Keep: true}}
	w := naturalRunsWidth(keep, 36)
	ok := map[string]bool{"2,859": true, " 億ドル": true, "2,859 億ドル": true}
	for _, width := range []float64{w * 1.01, w * 0.9, w * 0.5, 10} {
		lines := wrapRuns(keep, 36, width)
		for _, l := range lines {
			if !ok[runsText(l)] {
				t.Errorf("width %.1f: a keep run was broken: %v", width, lines)
			}
		}
		if width > w && len(lines) != 1 {
			t.Errorf("width %.1f: fits but wrapped: %v", width, lines)
		}
		if width < w && len(lines) != 2 {
			t.Errorf("width %.1f: want value and unit on 2 lines, got %v", width, lines)
		}
	}
	long := []TextRun{{Text: "12,345,678", Keep: true}}
	if lines := wrapRuns(long, 36, 50); len(lines) != 1 {
		t.Errorf("long value was broken: %v", lines)
	}
	plain := []TextRun{{Text: "2,859"}, {Text: " 億ドル"}}
	if lines := wrapRuns(plain, 36, w*0.9); len(lines) < 2 {
		t.Errorf("plain runs should still wrap, got %d line", len(lines))
	}
	// Keep の前の文はふつうに折れ、Keep の塊は丸ごと次の行に送られる。
	mixed := []TextRun{{Text: "前の文 前の文 "}, {Text: "12", Keep: true}, {Text: "億ドル", Keep: true}}
	lines := wrapRuns(mixed, 12, naturalRunsWidth(mixed[:1], 12)+2)
	last := lines[len(lines)-1]
	if got := runsText(last); got != "12億ドル" {
		t.Errorf("keep group not kept on its own line: lines = %v", lines)
	}
}

const statValueDoc = `# 数と単位

::::stat
:::value 12,345,678 {unit=千円}
長い数
:::
:::value 3 {unit=社}
短い数
:::
:::value 71 {unit=%}
割合
:::
::::
`

// 入りきらない数と単位は stat-value の警告(その数を指す。.md なら :::value の行)。
// 入る数には出ない。
func TestStatValueWarning(t *testing.T) {
	for _, f := range []string{"../examples/md/warnings/stat-value.bx.md", "../examples/ir/check-warnings/stat-value.json"} {
		for _, tpl := range []string{"light", "dark"} {
			_, warns := CheckFile(f, tpl)
			var got []Issue
			for _, w := range warns {
				if w.Code == "stat-value" {
					got = append(got, w)
				}
			}
			if len(got) != 1 || got[0].Path != "/slides/1/body/0/items/0" || got[0].Hint == "" {
				t.Errorf("%s %s: stat-value = %+v", f, tpl, got)
				continue
			}
			if strings.HasSuffix(f, ".md") && got[0].Line != 10 {
				t.Errorf("%s %s: line = %d, want 10", f, tpl, got[0].Line)
			}
		}
	}
}

// PPTX では数と単位は途中で切れない:入るものは1つの段落、入らないものも
// 数「12,345,678」と単位「千円」はそれぞれ1つの文字の塊のまま(折れるのは間だけ)。
func TestStatValueOneLineInPPTX(t *testing.T) {
	ir, errs := MarkdownToIR("t.bx.md", []byte(statValueDoc))
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	d, err := ParseIR(ir)
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := LookupTemplate("light")
	var buf bytes.Buffer
	if _, err := tm.RenderDeck(d, &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var slide string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			slide = string(b)
		}
	}
	shapeOf := func(text string) string {
		i := strings.Index(slide, text)
		if i < 0 {
			t.Fatalf("%q not found", text)
		}
		start := strings.LastIndex(slide[:i], "<p:sp>")
		end := strings.Index(slide[i:], "</p:sp>") + i
		return slide[start:end]
	}
	long := shapeOf("12,345,678")
	if !strings.Contains(long, "<a:t>12,345,678</a:t>") || !strings.Contains(long, "<a:t>千円</a:t>") {
		t.Errorf("long value or unit was broken: %s", long)
	}
	// カードは1つの図形で、1段落目が数と単位(改行なし)、2段落目が説明。
	short := shapeOf(">3<")
	paras := strings.Split(short, "<a:p>")[1:]
	if len(paras) != 2 || !strings.Contains(paras[0], "社") || strings.Contains(paras[0], "<a:br>") || !strings.Contains(paras[1], "短い数") {
		t.Errorf("short value: %d paragraphs: %s", len(paras), short)
	}
}
