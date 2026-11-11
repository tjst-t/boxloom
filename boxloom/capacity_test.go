package main

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var overflowMsgRe = regexp.MustCompile(`overflows the body area by ([0-9.]+)pt \(block needs ([0-9.]+)pt; body area ([0-9.]+)pt, all blocks need ([0-9.]+)pt\)`)

// 容量の数字は layout-overflow の警告と食い違わない:警告の出たブロックは
// overflow が警告の pt と同じで、警告の無いブロックは overflow が 0。本文の
// 領域と要る高さの合計も同じ。.md なら容量にも Markdown の行が付く。
func TestCapacityAgreesWithOverflowWarnings(t *testing.T) {
	files := []string{
		"../examples/ir/check-warnings/layout-overflow.json",
		"../examples/md/warnings/layout-overflow.bx.md",
		"../examples/ir/demo-deck.json",
		"../examples/md/demo-deck.bx.md",
		"../examples/ir/gantt-full.json",
		"../examples/ir/speaker-notes.json",
		"../examples/ir/shapes.json",
	}
	sawOverflow := false
	for _, f := range files {
		errs, warns, caps := CheckFileCapacity(f, "")
		if len(errs) != 0 {
			t.Fatalf("%s: errors %+v", f, errs)
		}
		if len(caps) == 0 {
			t.Fatalf("%s: no capacity", f)
		}
		blocks := map[string]BlockCapacity{}
		slides := map[string]SlideCapacity{}
		for _, c := range caps {
			slides[c.Path] = c
			if abs(c.Remaining-(c.BodyArea-c.Need)) > 0.11 {
				t.Errorf("%s %s: remaining %.1f != %.1f − %.1f", f, c.Path, c.Remaining, c.BodyArea, c.Need)
			}
			if c.Remaining <= 0 && c.RemainingBodyLines != 0 {
				t.Errorf("%s %s: no room but %d lines", f, c.Path, c.RemainingBodyLines)
			}
			if c.Remaining > 0 && float64(c.RemainingBodyLines)*c.BodyLineHeight > c.Remaining+0.1 {
				t.Errorf("%s %s: %d lines of %.1fpt do not fit in %.1fpt", f, c.Path, c.RemainingBodyLines, c.BodyLineHeight, c.Remaining)
			}
			if isMarkdownPath(f) && c.Line == 0 {
				t.Errorf("%s %s: no Markdown line", f, c.Path)
			}
			for _, b := range c.Blocks {
				blocks[b.Path] = b
				if isMarkdownPath(f) && b.Line == 0 {
					t.Errorf("%s %s: no Markdown line", f, b.Path)
				}
			}
		}
		warned := map[string]bool{}
		for _, w := range warns {
			if w.Code != "layout-overflow" {
				continue
			}
			sawOverflow = true
			warned[w.Path] = true
			m := overflowMsgRe.FindStringSubmatch(w.Message)
			if m == nil {
				t.Fatalf("%s: unexpected message %q", f, w.Message)
			}
			b, ok := blocks[w.Path]
			if !ok {
				t.Fatalf("%s: no capacity block for %s", f, w.Path)
			}
			num := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
			sp := w.Path[:strings.LastIndex(w.Path, "/body/")]
			s := slides[sp]
			if b.Overflow != num(m[1]) || b.Need != num(m[2]) || s.BodyArea != num(m[3]) || s.Need != num(m[4]) {
				t.Errorf("%s %s: capacity %+v / %+v disagrees with %q", f, w.Path, b, s, w.Message)
			}
		}
		for p, b := range blocks {
			if !warned[p] && b.Overflow != 0 {
				t.Errorf("%s %s: overflow %.1f without a warning", f, p, b.Overflow)
			}
		}
	}
	if !sawOverflow {
		t.Fatal("expected at least one layout-overflow warning in the examples")
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// 決まった数字:はみ出しの例のスライド(本文の領域 414.1pt、要る高さ
// 518.7pt。値の段階 #29 のあとの値)と、収まる例の残りの行。JSON のキーの形も見る。
func TestCapacityNumbersAndJSON(t *testing.T) {
	_, _, caps := CheckFileCapacity("../examples/ir/check-warnings/layout-overflow.json", "")
	if len(caps) != 1 {
		t.Fatalf("want 1 slide with a body, got %d", len(caps))
	}
	c := caps[0]
	if c.Path != "/slides/1" || c.BodyArea != 414.1 || c.Need != 518.7 || c.Remaining != -104.6 || c.RemainingBodyLines != 0 || len(c.Blocks) != 4 {
		t.Fatalf("unexpected capacity %+v", c)
	}
	if b := c.Blocks[2]; b.Type != "points" || b.Lines == 0 || b.Overflow != 75.1 {
		t.Fatalf("unexpected block %+v", b)
	}

	_, _, caps = CheckFileCapacity("../examples/ir/demo-deck.json", "")
	for _, c := range caps {
		if c.Remaining <= 0 || c.RemainingBodyLines == 0 || c.BodyLineHeight != 18.9 || c.BlockGap <= 0 {
			t.Errorf("demo-deck %s: unexpected %+v", c.Path, c)
		}
		if c.File != "../examples/ir/demo-deck.json" || c.Line != 0 {
			t.Errorf("IR capacity must carry file and no line: %+v", c)
		}
	}
	js, _ := json.Marshal(caps[0])
	var m map[string]any
	_ = json.Unmarshal(js, &m)
	for _, k := range []string{"file", "path", "bodyArea", "need", "remaining", "blockGap", "bodyLineHeight", "remainingBodyLines", "blocks"} {
		if _, ok := m[k]; !ok {
			t.Errorf("capacity JSON lacks %q: %s", k, js)
		}
	}
	if _, ok := m["line"]; ok {
		t.Errorf("IR capacity JSON must omit line: %s", js)
	}

	// 誤りのあるファイルは容量を出さない。
	if _, _, caps := CheckFileCapacity("../examples/ir/check-invalid/date-order.json", ""); len(caps) != 0 {
		t.Errorf("invalid deck must not report capacity: %+v", caps)
	}
}
