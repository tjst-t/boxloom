// 「boxloom mcp」:schema・check・build を MCP(Model Context Protocol)の道具として
// 出す stdio のサーバ(docs/mcp.md)。
//
// 依存は標準ライブラリだけ。MCP の stdio は「1行に1つの JSON-RPC 2.0 の文」なので、
// SDK を入れずに書ける(単一バイナリ・依存ゼロを崩さない)。要求は1つずつ順に
// 処理する(ライブラリは Build・Check を並行して呼べない。docs/library.md)。
//
// 道具の中身はコマンドの build・check・schema と同じ入口を呼ぶ。ファイルの読み書きは
// ここ(コマンドの側)で受け持ち、相対パスはサーバの作業フォルダから解く。
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"boxloom"
)

// mcpProtocolVersions は答えられる MCP の版(新しい順)。クライアントが言った版が
// この中にあればそれを、無ければ先頭を返す。
var mcpProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// mcpVersion はサーバの版(serverInfo)。
const mcpVersion = "0.1.0"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// runMCP は「boxloom mcp」:stdin から要求を読み、stdout に答えを書く。
// stderr はログ(MCP の決まりで、クライアントは stdout だけを読む)。
func runMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: boxloom mcp   # stdio の MCP サーバ(docs/mcp.md)")
		return 2
	}
	if err := serveMCP(stdin, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "boxloom mcp:", err)
		return 1
	}
	return 0
}

// serveMCP は入力が尽きるまで要求に答える。
func serveMCP(in io.Reader, out, logw io.Writer) error {
	r := bufio.NewReader(in)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if resp := handleMCPLine(line, logw); resp != nil {
				if werr := enc.Encode(resp); werr != nil {
					return werr
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// handleMCPLine は1行(1つの JSON-RPC の文)に答える。通知(id の無いもの)には
// nil を返す(答えない)。
func handleMCPLine(line []byte, logw io.Writer) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error: " + err.Error()}}
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		// 通知(notifications/initialized・notifications/cancelled など)。答えない。
		return nil
	}
	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if req.JSONRPC != "2.0" || req.Method == "" {
		resp.Error = &rpcError{-32600, "invalid request"}
		return resp
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		ver := mcpProtocolVersions[0]
		for _, v := range mcpProtocolVersions {
			if v == p.ProtocolVersion {
				ver = v
			}
		}
		resp.Result = map[string]any{
			"protocolVersion": ver,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "boxloom", "version": mcpVersion},
			"instructions":    mcpInstructions,
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": mcpTools()}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			resp.Error = &rpcError{-32602, "invalid params: " + err.Error()}
			return resp
		}
		tool, ok := mcpToolFuncs[p.Name]
		if !ok {
			resp.Error = &rpcError{-32602, fmt.Sprintf("unknown tool %q", p.Name)}
			return resp
		}
		fmt.Fprintf(logw, "boxloom mcp: tools/call %s\n", p.Name)
		resp.Result = callMCPTool(tool, p.Arguments, logw)
	default:
		resp.Error = &rpcError{-32601, "method not found: " + req.Method}
	}
	return resp
}

// mcpInstructions は initialize で渡す、道具の使い方の短い説明。
const mcpInstructions = `Boxloom turns a semantic deck (Boxloom Markdown .bx.md, YAML, or IR JSON) into a native, editable PPTX. ` +
	`Loop: call schema first (what the template accepts), write the deck, call check and fix every error and the warnings you can ` +
	`(use capacity.remaining / remainingBodyLines to decide whether to add, cut or split a slide; the engine never shrinks or moves content by itself), ` +
	`then call build. Relative paths are resolved from the server's working directory.`

// mcpTool は1つの道具:引数を受けて、結果の JSON(か誤り)を返す。
type mcpTool func(args json.RawMessage) (any, error)

var mcpToolFuncs = map[string]mcpTool{
	"schema": mcpSchema,
	"check":  mcpCheck,
	"build":  mcpBuild,
}

// callMCPTool は道具を呼び、MCP の CallToolResult にする。道具の誤りは
// JSON-RPC の誤りではなく isError で返す(エージェントが読んで直せるように)。
//
// 道具の中(ライブラリ)で panic が起きても、サーバは落とさず isError の結果で
// 返して次の要求に答え続ける(長く動かすサーバなので、1つの文書のせいで
// セッション全体を切らない)。字体などのプロセスの状態は、ライブラリの側が
// defer で戻すので、panic の後も元のまま(TestMCPToolPanic)。
// panic の要旨と呼び出しの跡は logw(stderr)にも書く。
func callMCPTool(tool mcpTool, args json.RawMessage, logw io.Writer) (result map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(logw, "boxloom mcp: panic: %v\n%s", r, debug.Stack())
			result = map[string]any{"isError": true, "content": []map[string]any{{"type": "text",
				"text": fmt.Sprintf("internal error (panic) in boxloom: %v. This is a bug in boxloom, not in the deck; the server keeps running.", r)}}}
		}
	}()
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	v, err := tool(args)
	if err != nil {
		return map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": err.Error()}}}
	}
	var text string
	if b, ok := v.([]byte); ok {
		text = string(b)
	} else {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": err.Error()}}}
		}
		text = buf.String()
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// 道具の引数の Schema(tools/list)。
var (
	mcpTemplateProp = map[string]any{
		"type":        "string",
		"description": "Template name (" + strings.Join(boxloom.Templates(), ", ") + ") or path to a template value table (.json). Omit to use the deck's own template (or light).",
	}
	mcpDeckProps = map[string]any{
		"path":     map[string]any{"type": "string", "description": "Deck file: .bx.md / .md (Boxloom Markdown), .yaml / .yml (YAML deck) or .json (IR). Images (image/gallery src) are read relative to its folder."},
		"source":   map[string]any{"type": "string", "description": "Deck contents given inline instead of path."},
		"name":     map[string]any{"type": "string", "description": "With source: file name whose extension picks the syntax (default deck.bx.md). Images are read relative to the server's working directory."},
		"template": mcpTemplateProp,
	}
)

func mcpTools() []map[string]any {
	with := func(base map[string]any, extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	return []map[string]any{
		{
			"name":        "schema",
			"description": "Return the vocabulary the templates accept as one JSON document: Markdown syntax rules, front-matter keys, fenced blocks, subsystems (figure code blocks) and the IR JSON Schema. Same as `boxloom schema`. Call this before writing a deck; anything not listed is an error.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"template": mcpTemplateProp}, "additionalProperties": false},
		},
		{
			"name":        "check",
			"description": "Check a deck without writing a PPTX: errors and warnings (each with path, line, code, message, hint) and, per slide, the body capacity (bodyArea, need, remaining pt, remainingBodyLines, per-block lines and overflow). Same as `boxloom check -format json`. Give either path or source.",
			"inputSchema": map[string]any{"type": "object", "properties": with(mcpDeckProps, map[string]any{
				"strict": map[string]any{"type": "boolean", "description": "Treat warnings as errors (ok becomes false)."},
				"fit":    map[string]any{"type": "boolean", "description": "Measure as build with fit does: an overflowing body slide is packed down the template's ladder (dense, then fit.shrink); packed slides are listed in info as fit-applied and get capacity.fit. A last resort: shorten or split first."},
			}), "additionalProperties": false},
		},
		{
			"name":        "build",
			"description": "Build a native, editable PPTX from a deck and write it to output. Returns the written path, slide count, template and layout warnings. Same as `boxloom build -o`. Give either path or source. A deck with errors is not written; run check first. A deck whose body overflows a slide (layout-overflow) is not written either: the error lists each overflow with its hint; fix them, or pass allowOverflow to build anyway.",
			"inputSchema": map[string]any{"type": "object", "properties": with(mcpDeckProps, map[string]any{
				"output":        map[string]any{"type": "string", "description": "PPTX file to write. Default: out/<deck name>.pptx under the server's working directory."},
				"allowOverflow": map[string]any{"type": "boolean", "description": "Write the PPTX even if a slide overflows (layout-overflow stays in warnings). Default false: overflow is an error."},
				"fit":           map[string]any{"type": "boolean", "description": "Pack an overflowing body slide down the template's ladder (dense, then fit.shrink) and build it at the first step that fits; packed slides are listed in info. If the last step still overflows, it stays layout-overflow. A last resort: shorten or split first."},
			}), "additionalProperties": false},
		},
	}
}

// decodeArgs は引数を読む。知らないキーは誤り(Schema の additionalProperties と同じ)。
func decodeArgs(args json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid arguments: %v", err)
	}
	return nil
}

func mcpSchema(args json.RawMessage) (any, error) {
	var a struct {
		Template string `json:"template"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	tname, tvalues, tissue := templateFlag(a.Template)
	if tissue != nil {
		return nil, errors.New(tissue.String())
	}
	if tvalues != nil {
		return boxloom.SchemaTemplate(tvalues)
	}
	return boxloom.Schema(tname)
}

// mcpDeckArgs は check と build に共通の引数。
type mcpDeckArgs struct {
	Path     string  `json:"path"`
	Source   *string `json:"source"`
	Name     string  `json:"name"`
	Template string  `json:"template"`
}

// load は文書の名前・中身・画像を読む口を返す。
func (a mcpDeckArgs) load() (file string, data []byte, assets fs.FS, err error) {
	switch {
	case a.Path != "" && a.Source != nil:
		return "", nil, nil, errors.New("give either path or source, not both")
	case a.Path != "":
		data, err = os.ReadFile(a.Path)
		if err != nil {
			return "", nil, nil, err
		}
		return a.Path, data, deckAssets(a.Path), nil
	case a.Source != nil:
		file = a.Name
		if file == "" {
			file = "deck.bx.md"
		}
		return file, []byte(*a.Source), os.DirFS("."), nil
	}
	return "", nil, nil, errors.New("give path (a deck file) or source (deck contents)")
}

func mcpCheck(args json.RawMessage) (any, error) {
	var a struct {
		mcpDeckArgs
		Strict bool `json:"strict"`
		Fit    bool `json:"fit"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	tname, tvalues, tissue := templateFlag(a.Template)
	if tissue != nil {
		return &boxloom.CheckResult{OK: false, Errors: []boxloom.Issue{*tissue}, Warnings: []boxloom.Issue{}, Capacity: []boxloom.SlideCapacity{}}, nil
	}
	file, data, assets, err := a.load()
	if err != nil {
		return nil, err
	}
	return boxloom.Check(file, data, boxloom.CheckOptions{Template: tname, Values: tvalues, Strict: a.Strict, Assets: assets, Fit: a.Fit}), nil
}

// mcpBuildResult は build の道具の結果。
type mcpBuildResult struct {
	Output   string   `json:"output"`
	Slides   int      `json:"slides"`
	Template string   `json:"template"`
	Warnings []string `json:"warnings"`
	Info     []string `json:"info,omitempty"`
}

func mcpBuild(args json.RawMessage) (any, error) {
	var a struct {
		mcpDeckArgs
		Output        string `json:"output"`
		AllowOverflow bool   `json:"allowOverflow"`
		Fit           bool   `json:"fit"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	tname, tvalues, tissue := templateFlag(a.Template)
	if tissue != nil {
		return nil, errors.New(tissue.String())
	}
	file, data, assets, err := a.load()
	if err != nil {
		return nil, err
	}
	deck, err := boxloom.ReadDeck(file, data)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	res, err := boxloom.Build(&buf, deck, boxloom.BuildOptions{Template: tname, Values: tvalues, Assets: assets, AllowOverflow: a.AllowOverflow, Fit: a.Fit})
	var oe *boxloom.OverflowError
	if errors.As(err, &oe) {
		for i := range oe.Issues {
			oe.Issues[i].File = file
		}
		return nil, oe
	}
	if err != nil {
		return nil, err
	}
	out := a.Output
	if out == "" {
		out = filepath.Join("out", deckBaseName(file)+".pptx")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return nil, err
	}
	if abs, err := filepath.Abs(out); err == nil {
		out = abs
	}
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return mcpBuildResult{Output: out, Slides: res.Slides, Template: res.Template, Warnings: warnings, Info: res.Info}, nil
}
