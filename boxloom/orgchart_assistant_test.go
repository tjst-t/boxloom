package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// 組織図の補佐(units[].assistant、#131)の Markdown。総括の横に副総括。
const orgAssistantMD = `---
title: 補佐
---

# 推進の体制

` + "```orgchart" + `
root: {title: 運営委員会}
units:
  - title: 総括
    detail: 架空 一郎
    assistant: {title: 副総括, detail: 架空 二郎}
    members:
      - {title: 企画}
      - {title: 調査}
      - {title: 広報}
  - title: 事務局
    members:
      - {title: 経理}
` + "```\n"

// 補佐を持つ組織図が check・build を通り、補佐の箱と、2段目の箱に繋がった破線の
// コネクタが出る。3段目の並び(箱の x と幅)は補佐の無いときと同じ。
func TestOrgChartAssistant(t *testing.T) {
	for _, tmpl := range []string{"light", "dark", "report"} {
		path := filepath.Join(t.TempDir(), "a.bx.md")
		src := []byte(orgAssistantMD)
		errs, warns, _ := checkCapacity(path, src, tmpl, nil, nil)
		if len(errs) > 0 {
			t.Fatalf("%s: errors %+v", tmpl, errs)
		}
		for _, w := range warns {
			if w.Code != "fill-ratio" && w.Code != "spare-width" {
				t.Errorf("%s: unexpected warning %s: %s", tmpl, w.Code, w.Message)
			}
		}
		d, err := ReadDeck(path, src)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := Build(&buf, d, BuildOptions{Template: tmpl}); err != nil {
			t.Fatalf("%s: build: %v", tmpl, err)
		}
		xml := zipEntry(t, buf.Bytes(), "ppt/slides/slide2.xml")
		if !strings.Contains(xml, ">副総括<") {
			t.Errorf("%s: no assistant box", tmpl)
		}
		if n := strings.Count(xml, `<a:prstDash val="dash"/>`); n != 1 {
			t.Errorf("%s: %d dashed lines, want 1 (the assistant connector)", tmpl, n)
		}
		// 根 → 2つ、総括 → 3つ、事務局 → 1つ、補佐 1つ。すべて両端が箱に繋がる。
		if n := strings.Count(xml, "<p:cxnSp>"); n != 7 || strings.Count(xml, "<a:stCxn ") != 7 || strings.Count(xml, "<a:endCxn ") != 7 {
			t.Errorf("%s: %d connectors, want 7 glued at both ends", tmpl, n)
		}
	}

	with := `{"root": {"title": "委員会"}, "units": [
	  {"title": "総括", "assistant": {"title": "副総括", "detail": "架空 二郎"}, "members": [{"title": "企画"}, {"title": "調査"}]},
	  {"title": "事務局", "members": [{"title": "経理"}]}]}`
	without := strings.Replace(with, `"assistant": {"title": "副総括", "detail": "架空 二郎"}, `, "", 1)
	for _, tmpl := range []string{"light", "dark"} {
		defer useFonts(mustTemplate(tmpl).Fonts)()
		a := figureFromIR(t, tmpl, "orgchart", with).(*OrgChartSubsystem)
		b := figureFromIR(t, tmpl, "orgchart", without).(*OrgChartSubsystem)
		for _, w := range []float64{912, 480, a.Measure(Constraint{}).Min.Width} {
			ga, gb := a.geom(w), b.geom(w)
			for i := range ga.memberX {
				for j := range ga.memberX[i] {
					if ga.memberX[i][j] != gb.memberX[i][j] || ga.memberWs[i] != gb.memberWs[i] {
						t.Errorf("%s %.0f: members of unit %d moved", tmpl, w, i)
					}
				}
			}
			// 補佐は2段目の箱の右にあり、隣の列の箱と重ならない。
			v := a.vals()
			if ga.asstX[0] < ga.unitX[0]+ga.unitW[0] || ga.asstW[0] <= 0 || ga.asstX[0]+ga.asstW[0]+v.UnitGap > ga.unitX[1]+1e-6 {
				t.Errorf("%s %.0f: assistant at %.1f+%.1f overlaps (unit %.1f+%.1f, next %.1f)", tmpl, w,
					ga.asstX[0], ga.asstW[0], ga.unitX[0], ga.unitW[0], ga.unitX[1])
			}
			pl := a.place(LBox{Width: w, Height: 360})
			if ab, ub := pl.assts[0], pl.units[0]; ab.Height > ub.Height || ab.Y < ub.Y {
				t.Errorf("%s %.0f: assistant box %+v is not inside the unit row %+v", tmpl, w, ab, ub)
			}
		}
		ma, mb := a.Measure(Constraint{}), b.Measure(Constraint{})
		if ma.Min.Width <= mb.Min.Width || ma.Natural.Width <= mb.Natural.Width {
			t.Errorf("%s: Measure width does not include the assistant (%+v vs %+v)", tmpl, ma, mb)
		}
	}
}

// 補佐の知らないキー・2つ目の補佐・空の題は誤り。
func TestOrgChartAssistantInvalid(t *testing.T) {
	for _, data := range []string{
		`{"root": {"title": "a"}, "units": [{"title": "b", "assistant": {"title": "c", "status": "danger"}}]}`,
		`{"root": {"title": "a"}, "units": [{"title": "b", "assistant": [{"title": "c"}, {"title": "d"}]}]}`,
		`{"root": {"title": "a"}, "units": [{"title": "b", "assistant": {"detail": "c"}}]}`,
	} {
		if _, err := mustTemplate("light").Subsystems.Build("orgchart", []byte(data)); err == nil {
			t.Errorf("no error for %s", data)
		}
	}
	src := strings.Replace(orgAssistantMD, "assistant: {title: 副総括, detail: 架空 二郎}", "assistant: {title: 副総括, role: 補佐}", 1)
	if _, issues, _ := ParseMarkdown("a.bx.md", []byte(src)); len(issues) == 0 {
		t.Error("unknown key in assistant parsed without error")
	}
}

func zipEntry(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("no %s", name)
	return ""
}
