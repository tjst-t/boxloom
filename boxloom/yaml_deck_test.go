package boxloom

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// yamlFromJSON は JSON の IR を、yaml.go の部分集合の YAML(ブロックの書き方)に
// 写す(試験用)。文字列は要るときだけ引用する。
func yamlFromJSON(t *testing.T, b []byte) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var root orderedJSON
	if err := dec.Decode(&root); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	emitYAML(&out, root.v, 0, false)
	return []byte(out.String())
}

// orderedJSON は JSON をキーの順を保って読む。
type orderedJSON struct{ v any }

type orderedObj struct {
	keys []string
	vals []any
}

func (o *orderedJSON) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := readOrdered(dec)
	o.v = v
	return err
}

func readOrdered(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		o := &orderedObj{}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := readOrdered(dec)
			if err != nil {
				return nil, err
			}
			o.keys = append(o.keys, k.(string))
			o.vals = append(o.vals, v)
		}
		_, err := dec.Token()
		return o, err
	case json.Delim('['):
		a := []any{}
		for dec.More() {
			v, err := readOrdered(dec)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		_, err := dec.Token()
		return a, err
	}
	return tok, nil
}

var plainUnsafe = regexp.MustCompile(`^(true|false|null|-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?)$`)

func yamlScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case json.Number:
		return string(x)
	case string:
		if x == "" || plainUnsafe.MatchString(x) || strings.ContainsAny(x[:1], "-{}[]\"'#&*!|>@`%?:,~ ") ||
			strings.Contains(x, ": ") || strings.Contains(x, " #") || strings.HasSuffix(x, ":") ||
			strings.HasSuffix(x, " ") || strings.ContainsAny(x, "\n\t\r") {
			b, _ := marshalNoEscape(x)
			return string(b)
		}
		return x
	}
	panic("unexpected")
}

func emitYAML(w *strings.Builder, v any, ind int, inSeq bool) {
	pad := strings.Repeat(" ", ind)
	switch x := v.(type) {
	case *orderedObj:
		if len(x.keys) == 0 {
			w.WriteString("{}\n")
			return
		}
		for i, k := range x.keys {
			if i > 0 || !inSeq {
				w.WriteString(pad)
			}
			w.WriteString(yamlScalar(k) + ":")
			emitChild(w, x.vals[i], ind+2)
		}
	case []any:
		if len(x) == 0 {
			w.WriteString("[]\n")
			return
		}
		for i, it := range x {
			if i > 0 || !inSeq {
				w.WriteString(pad)
			}
			w.WriteString("- ")
			switch it.(type) {
			case *orderedObj, []any:
				emitYAML(w, it, ind+2, true)
			default:
				w.WriteString(yamlScalar(it) + "\n")
			}
		}
	default:
		w.WriteString(yamlScalar(v) + "\n")
	}
}

func isEmptyContainer(v any) bool {
	switch x := v.(type) {
	case *orderedObj:
		return len(x.keys) == 0
	case []any:
		return len(x) == 0
	}
	return false
}

func emitChild(w *strings.Builder, v any, ind int) {
	switch v.(type) {
	case *orderedObj, []any:
		if isEmptyContainer(v) {
			w.WriteString(" ")
			emitYAML(w, v, ind, true)
			return
		}
		w.WriteString("\n")
		emitYAML(w, v, ind, false)
	default:
		w.WriteString(" " + yamlScalar(v) + "\n")
	}
}

func compactIRJSON(t *testing.T, b []byte) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// pptxParts は PPTX の部品(名前 → 中身)。
func pptxParts(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name] = data
	}
	return parts
}

func buildDeckBytes(t *testing.T, file string, src []byte, dir string) []byte {
	t.Helper()
	d, err := ReadDeck(file, src)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, d, BuildOptions{Assets: os.DirFS(dir)}); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return buf.Bytes()
}

func samePPTXParts(t *testing.T, name string, a, b []byte) {
	t.Helper()
	pa, pb := pptxParts(t, a), pptxParts(t, b)
	var names []string
	for k := range pa {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(pa) != len(pb) {
		t.Errorf("%s: %d parts vs %d parts", name, len(pa), len(pb))
	}
	for _, k := range names {
		if !bytes.Equal(pa[k], pb[k]) {
			t.Errorf("%s: part %s differs", name, k)
		}
	}
}

// TestYAMLExampleSameAsJSON は examples/yaml/ の deck が、同じ名前の
// examples/ir/ の JSON と同じ IR になり、build の PPTX の部品がバイト単位で
// 同じになることを確かめる。
func TestYAMLExampleSameAsJSON(t *testing.T) {
	files, _ := filepath.Glob("../examples/yaml/*.yaml")
	if len(files) == 0 {
		t.Fatal("no examples/yaml/*.yaml")
	}
	for _, yf := range files {
		jf := filepath.Join("../examples/ir", strings.TrimSuffix(filepath.Base(yf), ".yaml")+".json")
		ysrc, err := os.ReadFile(yf)
		if err != nil {
			t.Fatal(err)
		}
		jsrc, err := os.ReadFile(jf)
		if err != nil {
			t.Fatal(err)
		}
		ir, errs := YAMLToIR(yf, ysrc)
		if len(errs) > 0 {
			t.Fatalf("%s: %v", yf, errs)
		}
		if compactIRJSON(t, ir) != compactIRJSON(t, jsrc) {
			t.Errorf("%s: IR differs from %s", yf, jf)
		}
		samePPTXParts(t, yf, buildDeckBytes(t, yf, ysrc, "../examples/yaml"), buildDeckBytes(t, jf, jsrc, "../examples/ir"))
		if r := Check(yf, ysrc, CheckOptions{Assets: os.DirFS("../examples/yaml")}); !r.OK {
			t.Errorf("%s: check: %v", yf, r.Errors)
		}
	}
}

// TestYAMLMirrorsAllIRExamples は examples/ir/ の全部の JSON を YAML に写し、
// 読み戻すと同じ IR・同じ PPTX になることを確かめる(写し方が独自でないこと)。
func TestYAMLMirrorsAllIRExamples(t *testing.T) {
	files, _ := filepath.Glob("../examples/ir/*.json")
	for _, jf := range files {
		jsrc, err := os.ReadFile(jf)
		if err != nil {
			t.Fatal(err)
		}
		ysrc := yamlFromJSON(t, jsrc)
		yf := strings.TrimSuffix(jf, ".json") + ".yml"
		ir, errs := YAMLToIR(yf, ysrc)
		if len(errs) > 0 {
			t.Errorf("%s: %v\n%s", yf, errs, ysrc)
			continue
		}
		if compactIRJSON(t, ir) != compactIRJSON(t, jsrc) {
			t.Errorf("%s: IR differs", yf)
			continue
		}
		samePPTXParts(t, yf, buildDeckBytes(t, yf, ysrc, "../examples/ir"), buildDeckBytes(t, jf, jsrc, "../examples/ir"))
	}
}

func TestYAMLDeckScalars(t *testing.T) {
	src := "a: true\nb: \"true\"\nc: 1.5\nd: '2'\ne: 2026-04\nf:\ng: null\nh: yes\ni: [1, x, \"y\"]\n"
	ir, errs := parseYAMLDeckForTest(src)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	want := `{"a":true,"b":"true","c":1.5,"d":"2","e":"2026-04","f":null,"g":null,"h":"yes","i":[1,"x","y"]}`
	if got := compactIRJSON(t, ir); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func parseYAMLDeckForTest(src string) ([]byte, []Issue) {
	out, errs, _ := parseYAMLDeck("t.yaml", []byte(src))
	return out, errs
}

// YAML の誤り(部分集合の外・Schema の誤り・意味の誤り)に行と列が付く。
func TestYAMLDeckIssuePositions(t *testing.T) {
	cases := []struct {
		name, src  string
		code       string
		line, col  int
		path       string
		viaReadDck bool
	}{
		{"anchor", "boxloom: \"0.1\"\nslides: &a\n  - type: cover\n", codeYAML, 2, 9, "", true},
		{"multi-doc", "---\nboxloom: \"0.1\"\n", codeYAML, 1, 1, "", true},
		{"unknown key", "boxloom: \"0.1\"\nslides:\n  - type: content\n    title: T\n    titel: x\n", codeSchema, 5, 5, "/slides/0/titel", true},
		{"number for text", "boxloom: \"0.1\"\nslides:\n  - type: content\n    title: 2024\n", codeSchema, 0, 0, "", true},
		{"variant", "boxloom: \"0.1\"\nslides:\n  - type: content\n    title: T\n    body:\n      - type: points\n        variant: nope\n        items: [a]\n", "variant-exists", 7, 9, "/slides/0/body/0/variant", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Check("deck.yaml", []byte(c.src), CheckOptions{})
			if len(r.Errors) == 0 {
				t.Fatalf("no error")
			}
			e := r.Errors[0]
			if e.Code != c.code {
				t.Fatalf("code %q, want %q: %v", e.Code, c.code, e)
			}
			if e.Line == 0 || e.Column == 0 {
				t.Errorf("no line/column: %v", e)
			}
			if c.line != 0 && (e.Line != c.line || e.Column != c.col) {
				t.Errorf("at %d:%d, want %d:%d: %v", e.Line, e.Column, c.line, c.col, e)
			}
			if c.path != "" && e.Path != c.path {
				t.Errorf("path %q, want %q", e.Path, c.path)
			}
			if c.viaReadDck {
				_, err := ReadDeck("deck.yaml", []byte(c.src))
				me, ok := err.(MarkdownErrors)
				if !ok || len(me) == 0 || me[0].Line == 0 || me[0].Column == 0 {
					t.Errorf("ReadDeck: %v", err)
				}
			}
		})
	}
}
