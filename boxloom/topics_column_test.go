package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// 並列のまとまりの中の topics(#129):左に概要、右に A/B/C の塊。引用を持つ塊があるので
// コロンは外側ほど多い。
const topicsInColumnMD = `---
title: 割り振りの見直し
---

# 見直しの3つの観点

:::::::columns
::::::column 概要
区画の割り振りを、来年から3つの観点で見直す。

- 申し込みは例年どおり
::::::
::::::column 観点
:::::topics
:::topic 通いやすさ {label=A}
- 徒歩 15 分より遠い人には近くの区画を案内する
:::
:::topic 続けやすさ {label=B}
初めての人には相談役の区画の隣を割り振る
:::
::::topic 公平さ {label=C}
抽選の結果と理由を知らせる

:::quote 利用者 D
外れた理由が分かれば来年も申し込める
:::
::::
:::::
::::::
:::::::
`

const topicsInColumnIR = `{"boxloom": "0.1", "meta": {"title": "割り振りの見直し"}, "slides": [
  {"type": "cover", "title": "割り振りの見直し"},
  {"type": "content", "title": "見直しの3つの観点", "body": [
    {"type": "columns", "columns": [
      {"title": "概要", "body": [
        {"type": "paragraph", "content": "区画の割り振りを、来年から3つの観点で見直す。"},
        {"type": "points", "items": ["申し込みは例年どおり"]}]},
      {"title": "観点", "body": [
        {"type": "topics", "topics": [
          {"title": "通いやすさ", "label": "A", "body": [{"type": "points", "items": ["徒歩 15 分より遠い人には近くの区画を案内する"]}]},
          {"title": "続けやすさ", "label": "B", "body": [{"type": "paragraph", "content": "初めての人には相談役の区画の隣を割り振る"}]},
          {"title": "公平さ", "label": "C", "body": [
            {"type": "paragraph", "content": "抽選の結果と理由を知らせる"},
            {"type": "quote", "content": "外れた理由が分かれば来年も申し込める", "source": "利用者 D"}]}]}]}]}]}]}`

const topicsInColumnPath = "/slides/1/body/0/columns/1/body/0"

// Markdown の :::column の中の ::::topics は、IR の column の body の topics になる。
func TestTopicsInColumnMarkdownIR(t *testing.T) {
	ir, issues := MarkdownToIR("t.bx.md", []byte(topicsInColumnMD))
	if len(issues) > 0 {
		t.Fatalf("issues %+v", issues)
	}
	var got, want any
	if err := json.Unmarshal(ir, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(topicsInColumnIR), &want); err != nil {
		t.Fatal(err)
	}
	// 表紙のスライドの形は Markdown の読み方に任せ、本文のスライドだけを比べる。
	g := got.(map[string]any)["slides"].([]any)[1]
	w := want.(map[string]any)["slides"].([]any)[1]
	gj, _ := json.Marshal(g)
	wj, _ := json.Marshal(w)
	if !sameJSON(gj, wj) {
		t.Errorf("markdown slide\n%s\nwant\n%s", gj, wj)
	}
	if _, err := ParseIR(ir); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseIR([]byte(topicsInColumnIR)); err != nil {
		t.Fatal(err)
	}

	// まとまりの中の topics の形の誤りは、本文と同じく場所つき。
	bad := strings.Replace(topicsInColumnMD, ":::topic 続けやすさ {label=B}", ":::topic 続けやすさ", 1)
	_, issues = MarkdownToIR("t.bx.md", []byte(bad))
	at := false
	for _, is := range issues {
		at = at || (is.Path == topicsInColumnPath+"/topics" && is.Code == codeInvalidValue)
	}
	if !at {
		t.Errorf("mixed labels in a column: issues %+v", issues)
	}
}

// check は列の中の topics の中も本文と同じく見る。組み方を持たないテンプレートは、
// その場所に component-fallback を1件出す(代わりの箇条は列の中に置ける)。
func TestTopicsInColumnCheck(t *testing.T) {
	for _, src := range []struct{ name, data string }{{"t.bx.md", topicsInColumnMD}, {"t.json", topicsInColumnIR}} {
		for _, tmpl := range []string{"report", "dark", "light"} {
			res := Check(src.name, []byte(src.data), CheckOptions{Template: tmpl})
			if len(res.Errors) > 0 {
				t.Fatalf("%s@%s: errors %+v", src.name, tmpl, res.Errors)
			}
			for _, w := range res.Warnings {
				if w.Code == "component-fallback" || w.Code == "layout-overflow" {
					t.Errorf("%s@%s: %+v", src.name, tmpl, w)
				}
			}
		}
		res := Check(src.name, []byte(src.data), CheckOptions{Values: noTopicsTemplate(t)})
		if len(res.Errors) > 0 {
			t.Fatalf("%s fallback: errors %+v", src.name, res.Errors)
		}
		var fb []string
		for _, w := range res.Warnings {
			if w.Code == "component-fallback" {
				fb = append(fb, w.Path)
			}
		}
		if !slices.Equal(fb, []string{topicsInColumnPath}) {
			t.Errorf("%s: component-fallback at %v", src.name, fb)
		}
	}

	// 中の題・段落の空は、列の中の場所で指す。
	empty := strings.Replace(topicsInColumnIR, `"content": "初めての人には相談役の区画の隣を割り振る"`, `"content": " "`, 1)
	res := Check("t.json", []byte(empty), CheckOptions{Template: "light"})
	found := false
	for _, e := range append(res.Errors, res.Warnings...) {
		found = found || e.Path == topicsInColumnPath+"/topics/1/body/0/content"
	}
	if !found {
		t.Errorf("empty paragraph in a topic in a column: errors %+v warnings %+v", res.Errors, res.Warnings)
	}
}

// report・dark・light で組むと、塊は右のまとまりの中に上から順に積まれる。
func TestTopicsInColumnBuild(t *testing.T) {
	for _, src := range []struct{ name, data string }{{"t.bx.md", topicsInColumnMD}, {"t.json", topicsInColumnIR}} {
		for _, tmpl := range []string{"report", "dark", "light"} {
			deck, err := ReadDeck(src.name, []byte(src.data))
			if err != nil {
				t.Fatal(err)
			}
			var pptx bytes.Buffer
			res, err := Build(&pptx, deck, BuildOptions{Template: tmpl})
			if err != nil {
				t.Fatalf("%s@%s: %v", src.name, tmpl, err)
			}
			for _, w := range res.Warnings {
				if strings.Contains(w, "COMPONENT-FALLBACK") || strings.Contains(w, "LAYOUT-OVERFLOW") {
					t.Errorf("%s@%s: %s", src.name, tmpl, w)
				}
			}
			dump, err := Layout(src.name, []byte(src.data), BuildOptions{Template: tmpl})
			if err != nil {
				t.Fatal(err)
			}
			seen := pairsLayoutNodes(dump)
			block := seen[topicsInColumnPath]
			left := seen["/slides/1/body/0/columns/0/body/0"]
			if block == nil || left == nil || block.Marks == nil || block.Marks.IRType != "topics" || block.Marks.TopicRows != 3 {
				t.Fatalf("%s@%s: block %+v", src.name, tmpl, block)
			}
			if block.Box.X < left.Box.X+left.Box.Width {
				t.Errorf("%s@%s: topics %+v not right of the left column %+v", src.name, tmpl, block.Box, left.Box)
			}
			bottom := block.Box.Y
			for k := 0; k < 3; k++ {
				row := seen[fmt.Sprintf("%s/topics/%d", topicsInColumnPath, k)]
				if row == nil {
					t.Fatalf("%s@%s: topic %d missing", src.name, tmpl, k)
				}
				if row.Box.Y < bottom-0.01 {
					t.Errorf("%s@%s: topic %d at %.2f above the previous bottom %.2f", src.name, tmpl, k, row.Box.Y, bottom)
				}
				if row.Box.X < block.Box.X-0.01 || row.Box.X+row.Box.Width > block.Box.X+block.Box.Width+0.01 {
					t.Errorf("%s@%s: topic %d %+v outside the block %+v", src.name, tmpl, k, row.Box, block.Box)
				}
				bottom = row.Box.Y + row.Box.Height
			}
		}
	}
}

// IR の Schema のまとまりの中のブロック(columnBlock)は topics を振り分ける。
func TestTopicsInColumnSchema(t *testing.T) {
	var core struct {
		Defs map[string]struct {
			Properties struct {
				Type struct {
					Enum []string `json:"enum"`
				} `json:"type"`
			} `json:"properties"`
			AllOf []struct {
				If struct {
					Properties struct {
						Type struct {
							Const string `json:"const"`
						} `json:"type"`
					} `json:"properties"`
				} `json:"if"`
				Then struct {
					Ref string `json:"$ref"`
				} `json:"then"`
			} `json:"allOf"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(embeddedSchema("core.schema.json"), &core); err != nil {
		t.Fatal(err)
	}
	d := core.Defs["columnBlock"]
	if !slices.Contains(d.Properties.Type.Enum, topicsType) {
		t.Errorf("columnBlock does not name topics: %v", d.Properties.Type.Enum)
	}
	ok := false
	for _, a := range d.AllOf {
		ok = ok || (a.If.Properties.Type.Const == topicsType && a.Then.Ref == "types/topics.schema.json")
	}
	if !ok {
		t.Error("columnBlock does not dispatch topics to types/topics.schema.json")
	}
	if !slices.Contains(columnContains, "topics") {
		t.Errorf("columnContains %v", columnContains)
	}
}
