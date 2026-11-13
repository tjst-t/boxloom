package boxloom

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// #100 report の細かな見た目:リードは太字でない・群のセルは縦書きの帯・図の説明は上に基調色。

// shapeWithText は text を含む p:sp の XML("" なら無い)。
func shapeWithText(slide, text string) string {
	for _, sp := range strings.Split(slide, "<p:sp>")[1:] {
		sp = sp[:strings.Index(sp, "</p:sp>")]
		if strings.Contains(sp, text) {
			return sp
		}
	}
	return ""
}

var offYRe = regexp.MustCompile(`<a:off x="-?\d+" y="(-?\d+)"/>`)

func offY(t *testing.T, x string) int {
	t.Helper()
	m := offYRe.FindStringSubmatch(x)
	if m == nil {
		t.Fatalf("no a:off in %.80s", x)
	}
	y, _ := strconv.Atoi(m[1])
	return y
}

func TestReportLeadNotBold(t *testing.T) {
	for _, tmpl := range []string{"report"} {
		slide := builtSlideXML(t, reportHeaderDeck, tmpl, 2)
		sp := shapeWithText(slide, "午前の混雑")
		if sp == "" {
			t.Fatal("lead shape not found")
		}
		if strings.Contains(sp, `b="1"`) {
			t.Errorf("%s: lead runs must not be bold: %s", tmpl, sp)
		}
	}
}

func TestReportTableGroupVertical(t *testing.T) {
	slide := builtSlideXML(t, reportCalloutsDeck, "report", 2)
	rows := tableCellsXML(slide)
	for _, r := range []int{1, 2, 3} {
		if pr := cellPart(rows[r][0], "tcPr"); !strings.Contains(pr, `vert="eaVert"`) {
			t.Errorf("group cell row %d must be vertical: %.120s", r, pr)
		}
	}
	for r, row := range rows {
		for c := 1; c < len(row); c++ {
			if strings.Contains(row[c], "vert=") {
				t.Errorf("cell %d,%d must not be vertical", r, c)
			}
		}
	}
	// 帯の列の幅は縦の1行(tableText × 行の高さ + 左右の余白)で、余りの幅を受け取らない。
	tm, _ := LookupTemplate("report")
	v := tm.Values
	restore := useFonts(tm.Fonts) // 行の高さは report の fonts.lineHeight
	want := ptToEMU(vertBandWidth(v.TableText, v.TableCellPadX))
	restore()
	m := regexp.MustCompile(`<a:gridCol w="(\d+)"/>`).FindAllStringSubmatch(slide, -1)
	if len(m) != 3 {
		t.Fatalf("gridCol = %d", len(m))
	}
	if got, _ := strconv.Atoi(m[0][1]); int64(got) != int64(want) {
		t.Errorf("band column width = %d EMU, want %d", got, want)
	}
	// dark・light は横書きのまま。
	for _, tmpl := range []string{"dark", "light"} {
		if s := builtSlideXML(t, reportCalloutsDeck, tmpl, 2); strings.Contains(s, "eaVert") {
			t.Errorf("%s: group cells must stay horizontal", tmpl)
		}
	}
}

// 縦書きの名前が結合した行より長ければ、群の最後の行を高くする。
func TestTableVerticalCellGrowsRows(t *testing.T) {
	leaf := &tableLeaf{size: 10, padX: 4, padY: 2, cells: [][]TableCell{
		{{Runs: []TextRun{{Text: "群"}}}, {Runs: []TextRun{{Text: "a"}}}},
		{{Runs: []TextRun{{Text: "とても長い群の名前です"}}, RowSpan: 1, Vert: true}, {Runs: []TextRun{{Text: "b"}}}},
	}}
	hs, _ := leaf.rowHeights([]float64{20, 100})
	need := leaf.cells[1][0].vertLength(10) + 2*leaf.padY
	if hs[1] < need {
		t.Errorf("row height %.1f < vertical text %.1f", hs[1], need)
	}
}

const reportImageDeck = `{"boxloom": "0.1", "slides": [
 {"type": "content", "title": "題", "body": [{"type": "image", "src": "wide.png", "alt": "図", "caption": "図の説明のひとつ"}]},
 {"type": "content", "title": "題", "body": [{"type": "gallery", "images": [
   {"src": "wide.png", "alt": "a", "caption": "左の説明"}, {"src": "wide.png", "alt": "b", "caption": "右の説明"}]}]}]}`

func builtIRSlides(t *testing.T, src, tmpl string) []string {
	t.Helper()
	d, err := ReadIR("t.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, d, BuildOptions{Template: tmpl, Assets: testAssets(t)}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for i := 1; ; i++ {
		f, err := zr.Open("ppt/slides/slide" + strconv.Itoa(i) + ".xml")
		if err != nil {
			return out
		}
		b, _ := io.ReadAll(f)
		f.Close()
		out = append(out, string(b))
	}
}

func TestReportImageCaptionAbove(t *testing.T) {
	for _, tmpl := range []string{"report", "light"} {
		slides := builtIRSlides(t, reportImageDeck, tmpl)
		tm, _ := LookupTemplate(tmpl)
		for i, cap := range []string{"図の説明のひとつ", "左の説明"} {
			s := slides[i]
			sp := shapeWithText(s, cap)
			pic := s[strings.Index(s, "<p:pic>"):]
			if sp == "" {
				t.Fatalf("%s: caption %q not found", tmpl, cap)
			}
			capY, picY := offY(t, sp), offY(t, pic)
			if tmpl == "report" {
				if capY >= picY {
					t.Errorf("report: caption %q (y %d) must be above the picture (y %d)", cap, capY, picY)
				}
				if got := fillOfXML(sp[strings.Index(sp, "<a:r>"):]); got != tm.Theme["accent"] {
					t.Errorf("report: caption color = %s, want accent %s", got, tm.Theme["accent"])
				}
			} else if capY <= picY {
				t.Errorf("%s: caption %q must stay below the picture", tmpl, cap)
			}
		}
	}
}
