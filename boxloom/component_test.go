package boxloom

import (
	"reflect"
	"sort"
	"testing"
)

// 標準の部品は docs/layers.md の5章の一式で、囲みは defaultBlocks と同じものを名乗る。
func TestStandardComponents(t *testing.T) {
	r := defaultComponents()
	want := []string{"columns", "compare", "gallery", "image", "note", "paragraph", "points", "quote", "stat", "subsystem", "table", "takeaway"}
	if got := r.names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	var fences []BlockDecl
	for _, n := range r.names() {
		d, ok := r.decl(n)
		if !ok {
			t.Fatalf("%s: no decl", n)
		}
		if d.Schema != nil {
			t.Errorf("%s: standard component has its own Schema", n)
		}
		fences = append(fences, d.Markdown...)
	}
	sort.Slice(fences, func(i, j int) bool { return fences[i].Name < fences[j].Name })
	if !reflect.DeepEqual(fences, defaultBlocks()) {
		t.Errorf("fences of the standard components differ from defaultBlocks")
	}
	for _, n := range r.names() {
		alt, err := r.components[n].fallback(&IRBlock{Type: n})
		if alt != nil || err != nil {
			t.Errorf("%s: standard component has a fallback", n)
		}
	}
}

func TestComponentRegistry(t *testing.T) {
	r := defaultComponents()
	if err := r.register("note", componentDecl{}, componentFunc(nil)); err == nil {
		t.Error("register of an existing name succeeded")
	}
	if err := r.override("pairs", componentFunc(nil)); err == nil {
		t.Error("override of an unknown name succeeded")
	}
	if _, err := r.build(componentContext{}, &IRBlock{Type: "pairs"}); err == nil || err.Error() != `unknown block type "pairs"` {
		t.Errorf("unknown type: err = %v", err)
	}

	// テンプレートは写しを持つ。差し替えは名乗るものを変えず、ほかのテンプレートにも及ばない。
	dark, light := mustTemplate("dark"), mustTemplate("light")
	before, _ := dark.components.decl("note")
	marker := Col(nil)
	if err := dark.components.override("note", componentFunc(func(componentContext, *IRBlock) (*Spec, error) {
		return marker, nil
	})); err != nil {
		t.Fatal(err)
	}
	if after, _ := dark.components.decl("note"); !reflect.DeepEqual(before, after) {
		t.Error("override changed the decl")
	}
	b := &IRBlock{Type: "note", Content: Inline{{Text: "x"}}}
	s, err := dark.buildBlock(b)
	if err != nil || s != marker || s.irType != "note" {
		t.Errorf("overridden note: spec %p (want %p), irType %q, err %v", s, marker, s.irType, err)
	}
	s, err = light.buildBlock(b)
	if err != nil || s == marker {
		t.Errorf("override leaked into another template (err %v)", err)
	}
	if again, _ := standardComponents.components["note"].build(componentContext{t: light}, b); again == marker {
		t.Error("override leaked into the standard table")
	}
}
