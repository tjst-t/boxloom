package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
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
