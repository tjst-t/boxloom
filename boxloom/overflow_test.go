package main

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// 高さを決めた箱だけで組んだページ: 内側 100pt − 題 20pt − gap 10pt×2 −
// フッター 10pt = 本文の領域 50pt。ブロック 30pt + gap 5pt + 30pt = 65pt なので、
// 2つ目のブロックが 15pt はみ出す。
func TestMeasureBodyOverflow(t *testing.T) {
	blk := func() *Spec { return Col(nil, WithHeight(30)) }
	body := Col([]*Spec{blk(), blk()}, WithGap(5), WithGrow(1))
	root := Col([]*Spec{Col(nil, WithHeight(20)), body, Col(nil, WithHeight(10))}, WithGap(10), WithPadding(10))
	computeLayout(root, 0, 0, 200, 120)
	over, capacity, need := measureBodyOverflow(root, body)
	if math.Abs(capacity-50) > 1e-9 || math.Abs(need-65) > 1e-9 {
		t.Fatalf("capacity %.2f need %.2f, want 50 and 65", capacity, need)
	}
	if len(over) != 1 || over[0].Index != 1 || math.Abs(over[0].Overflow-15) > 1e-9 {
		t.Fatalf("want block 1 overflowing by 15pt, got %+v", over)
	}

	// 収まるなら何も出さない。
	root.Box.Height = 140
	if over, _, _ := measureBodyOverflow(root, body); len(over) != 0 {
		t.Fatalf("want no overflow, got %+v", over)
	}

	// 領域の下端より下に丸ごと落ちたブロックは、要る高さ全部がはみ出し。
	body.Children = append(body.Children, blk())
	root.Box.Height = 120
	over, _, _ = measureBodyOverflow(root, body)
	if len(over) != 2 || over[1].Index != 2 || math.Abs(over[1].Overflow-30) > 1e-9 {
		t.Fatalf("want block 2 overflowing by 30pt, got %+v", over)
	}
}

// サブシステムは最小の高さまで縮めてよいので、自然な高さを越えても
// 最小の高さが収まれば はみ出しにしない。
func TestOverflowUsesSubsystemMin(t *testing.T) {
	sub := minSub{natural: Size{Height: 200}, min: Size{Height: 40}}
	body := Col([]*Spec{SubsystemNode(sub, WithGrow(1))}, WithGrow(1))
	root := Col([]*Spec{body}, WithPadding(0))
	computeLayout(root, 0, 0, 200, 100)
	if over, _, _ := measureBodyOverflow(root, body); len(over) != 0 {
		t.Fatalf("subsystem above its min should not overflow: %+v", over)
	}
	sub.min = Size{Height: 120}
	body.Children[0].Subsystem = sub
	over, _, _ := measureBodyOverflow(root, body)
	if len(over) != 1 || math.Abs(over[0].Overflow-20) > 1e-9 {
		t.Fatalf("want 20pt overflow, got %+v", over)
	}
}

type minSub struct{ natural, min Size }

func (minSub) Name() string { return "fixed" }
func (f minSub) Measure(Constraint) Measurement {
	return Measurement{Natural: f.natural, Min: f.min}
}
func (minSub) Render(RenderContext, LBox) {}

// はみ出す例は、はみ出したブロックごとに layout-overflow の警告が出て、
// path はそのブロック、.md なら行も付く。誤りにはしない。
func TestCheckLayoutOverflowExample(t *testing.T) {
	for _, f := range []string{"../examples/ir/check-warnings/layout-overflow.json", "../examples/md/warnings/layout-overflow.bx.md"} {
		errs, warns := CheckFile(f, "")
		if len(errs) != 0 {
			t.Fatalf("%s: unexpected errors %+v", f, errs)
		}
		var paths []string
		for _, w := range warns {
			if w.Code != "layout-overflow" {
				t.Errorf("%s: unexpected warning %+v", f, w)
				continue
			}
			if !strings.Contains(w.Message, "overflows the body area by") {
				t.Errorf("%s: message lacks the amount: %q", f, w.Message)
			}
			if isMarkdownPath(f) != (w.Line > 0) {
				t.Errorf("%s: line %d", f, w.Line)
			}
			paths = append(paths, w.Path)
		}
		if strings.Join(paths, ",") != "/slides/1/body/2,/slides/1/body/3" {
			t.Errorf("%s: paths %v", f, paths)
		}
	}
}

// 正しい例(examples/ir と examples/md)は はみ出さない。
func TestExamplesDoNotOverflow(t *testing.T) {
	files, _ := filepath.Glob("../examples/ir/*.json")
	md, _ := filepath.Glob("../examples/md/*.bx.md")
	for _, f := range append(files, md...) {
		_, warns := CheckFile(f, "")
		for _, w := range warns {
			if w.Code == "layout-overflow" {
				t.Errorf("%s: %+v", filepath.Base(f), w)
			}
		}
	}
}

// 警告の文の頭の body[N] は、そのブロックの場所になる。
func TestWarningIssueBlockPath(t *testing.T) {
	is := warningIssue("d.json", "slide 3: [LAYOUT-OVERFLOW WARNING] body[4]: block (note) overflows the body area by 2.0pt")
	if is.Code != "layout-overflow" || is.Path != "/slides/2/body/4" || strings.HasPrefix(is.Message, "body[") {
		t.Fatalf("got %+v", is)
	}
}
