package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"boxloom"
)

// mcpSession は要求の行を流し、答えを id ごとに返す。
func mcpSession(t *testing.T, lines ...string) map[string]map[string]any {
	t.Helper()
	var out bytes.Buffer
	if c := runMCP(nil, strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, io.Discard); c != 0 {
		t.Fatalf("exit %d", c)
	}
	got := map[string]map[string]any{}
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		id, _ := json.Marshal(m["id"])
		got[string(id)] = m
	}
	return got
}

// toolText は tools/call の答えの文と isError を取り出す。
func toolText(t *testing.T, m map[string]any) (string, bool) {
	t.Helper()
	r, ok := m["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", m)
	}
	c := r["content"].([]any)[0].(map[string]any)
	isErr, _ := r["isError"].(bool)
	return c["text"].(string), isErr
}

func call(id int, name string, args map[string]any) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	return string(b)
}

// 初期化・通知・道具の一覧・3つの道具を、Claude Code が送るのと同じ順に流す。
func TestMCPSession(t *testing.T) {
	out := filepath.Join(t.TempDir(), "deck.pptx")
	got := mcpSession(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		call(3, "schema", map[string]any{"template": "dark"}),
		call(4, "check", map[string]any{"path": "../../../examples/md/demo-deck.bx.md"}),
		call(5, "build", map[string]any{"path": "../../../examples/md/demo-deck.bx.md", "template": "dark", "output": out}),
		call(6, "check", map[string]any{"source": "---\ntitle: x\n---\n\n# A\n\n:::nope\n:::\n"}),
		call(7, "build", map[string]any{}),
		`{"jsonrpc":"2.0","id":8,"method":"nope"}`,
		`{"jsonrpc":"2.0","id":9,"method":"ping"}`,
	)
	if len(got) != 9 {
		t.Fatalf("answers = %d, want 9 (the notification gets none)", len(got))
	}
	ini := got["1"]["result"].(map[string]any)
	if ini["protocolVersion"] != "2025-03-26" || ini["capabilities"].(map[string]any)["tools"] == nil {
		t.Errorf("initialize = %v", ini)
	}
	var names []string
	for _, tl := range got["2"]["result"].(map[string]any)["tools"].([]any) {
		names = append(names, tl.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "schema,check,build" {
		t.Errorf("tools = %v", names)
	}
	if s, isErr := toolText(t, got["3"]); isErr || !json.Valid([]byte(s)) || !strings.Contains(s, `"irSchema"`) {
		t.Errorf("schema: isError=%v %.200s", isErr, s)
	}
	s, isErr := toolText(t, got["4"])
	var cr struct {
		OK       bool              `json:"ok"`
		Capacity []json.RawMessage `json:"capacity"`
	}
	if err := json.Unmarshal([]byte(s), &cr); isErr || err != nil || !cr.OK || len(cr.Capacity) == 0 {
		t.Errorf("check: isError=%v err=%v %.300s", isErr, err, s)
	}
	s, isErr = toolText(t, got["5"])
	var br mcpBuildResult
	if err := json.Unmarshal([]byte(s), &br); isErr || err != nil || br.Slides != 3 || br.Template != "dark" || br.Output != out {
		t.Errorf("build: isError=%v err=%v %s", isErr, err, s)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("pptx: %v", err)
	}
	zr.Close()
	s, isErr = toolText(t, got["6"])
	if isErr || !strings.Contains(s, `"ok": false`) || !strings.Contains(s, `"line": 7`) {
		t.Errorf("check source with error: isError=%v %s", isErr, s)
	}
	if s, isErr := toolText(t, got["7"]); !isErr || !strings.Contains(s, "path") {
		t.Errorf("build without deck: isError=%v %s", isErr, s)
	}
	if e := got["8"]["error"].(map[string]any); e["code"].(float64) != -32601 {
		t.Errorf("unknown method: %v", e)
	}
	if _, ok := got["9"]["result"]; !ok {
		t.Errorf("ping: %v", got["9"])
	}
}

func TestMCPBadArguments(t *testing.T) {
	got := mcpSession(t,
		call(1, "check", map[string]any{"path": "x.json", "bogus": true}),
		call(2, "build", map[string]any{"path": "a.bx.md", "source": "x"}),
		call(3, "nope", nil),
		`not json`,
	)
	if s, isErr := toolText(t, got["1"]); !isErr || !strings.Contains(s, "bogus") {
		t.Errorf("unknown key: %v %s", isErr, s)
	}
	if s, isErr := toolText(t, got["2"]); !isErr || !strings.Contains(s, "not both") {
		t.Errorf("path and source: %v %s", isErr, s)
	}
	if e := got["3"]["error"].(map[string]any); e["code"].(float64) != -32602 {
		t.Errorf("unknown tool: %v", e)
	}
	if e := got["null"]["error"].(map[string]any); e["code"].(float64) != -32700 {
		t.Errorf("parse error: %v", e)
	}
}

// panicFS は画像を読もうとすると panic する(ライブラリの中の panic の試験用の差し込み)。
type panicFS struct{}

func (panicFS) Open(name string) (fs.File, error) { panic("injected panic reading " + name) }

// 道具の中で panic が起きても、サーバは isError の結果を返して動き続け、次の要求に
// 正しく答える。panic の後と、要求を続けた後に、字体の設定が元に戻っている。
func TestMCPToolPanic(t *testing.T) {
	fontsBefore := boxloom.NewPPTXBuilder(1, 1).Fonts
	mcpToolFuncs["test-panic"] = func(json.RawMessage) (any, error) { panic("injected panic") }
	// 組み立ての途中(テンプレートの字体に替えた後、画像を読むところ)で panic する。
	mcpToolFuncs["test-panic-build"] = func(json.RawMessage) (any, error) {
		src := "---\ntitle: x\ntemplate: dark\n---\n\n# A\n\n![a](a.png)\n"
		deck, err := boxloom.ReadDeck("deck.bx.md", []byte(src))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		return boxloom.Build(&buf, deck, boxloom.BuildOptions{Assets: panicFS{}})
	}
	t.Cleanup(func() {
		delete(mcpToolFuncs, "test-panic")
		delete(mcpToolFuncs, "test-panic-build")
	})
	out := filepath.Join(t.TempDir(), "deck.pptx")
	deck := "../../../examples/md/demo-deck.bx.md"
	var log bytes.Buffer
	var stdout bytes.Buffer
	lines := []string{
		call(1, "check", map[string]any{"path": deck}),
		call(2, "test-panic", nil),
		call(3, "test-panic-build", nil),
		call(4, "check", map[string]any{"path": deck}),
		call(5, "build", map[string]any{"path": deck, "template": "light", "output": out}),
		call(6, "check", map[string]any{"path": deck, "template": "dark"}),
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	}
	if c := runMCP(nil, strings.NewReader(strings.Join(lines, "\n")+"\n"), &stdout, &log); c != 0 {
		t.Fatalf("exit %d", c)
	}
	got := map[string]map[string]any{}
	for _, l := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		id, _ := json.Marshal(m["id"])
		got[string(id)] = m
	}
	if len(got) != 7 {
		t.Fatalf("answers = %d, want 7 (the server must keep running after a panic)", len(got))
	}
	for _, id := range []string{"2", "3"} {
		s, isErr := toolText(t, got[id])
		if !isErr || !strings.Contains(s, "panic") || !strings.Contains(s, "injected panic") {
			t.Errorf("panic %s: isError=%v %s", id, isErr, s)
		}
	}
	if !strings.Contains(log.String(), "injected panic reading") {
		t.Errorf("panic in Build was not reached or not logged: %s", log.String())
	}
	// panic の前と後の check は同じ答え(字体などの状態が崩れていない)。
	first, isErr1 := toolText(t, got["1"])
	after, isErr4 := toolText(t, got["4"])
	if isErr1 || isErr4 || first != after || !strings.Contains(after, `"ok": true`) {
		t.Errorf("check before/after panic differ: isError=%v/%v\n%.300s\n%.300s", isErr1, isErr4, first, after)
	}
	if s, isErr := toolText(t, got["5"]); isErr || !strings.Contains(s, `"slides": 3`) {
		t.Errorf("build after panic: isError=%v %s", isErr, s)
	}
	if s, isErr := toolText(t, got["6"]); isErr || !strings.Contains(s, `"ok": true`) {
		t.Errorf("check after panic: isError=%v %s", isErr, s)
	}
	if _, ok := got["7"]["result"]; !ok {
		t.Errorf("ping: %v", got["7"])
	}
	if f := boxloom.NewPPTXBuilder(1, 1).Fonts; f != fontsBefore {
		t.Errorf("fonts not restored after the requests: %+v, want %+v", f, fontsBefore)
	}
}
