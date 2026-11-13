package boxloom

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// pngOf は w×h の1色の PNG(試験用)。
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0x30, 0x60, 0x90, 0xFF})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testAssets(t *testing.T) fstest.MapFS {
	return fstest.MapFS{
		"wide.png":     {Data: pngOf(t, 320, 180)},
		"img/tall.png": {Data: pngOf(t, 200, 300)},
		"note.txt":     {Data: []byte("not an image")},
	}
}

func buildZip(t *testing.T, deck string, assets fstest.MapFS) (*zip.Reader, *BuildResult) {
	t.Helper()
	d, err := ReadIR("t.json", []byte(deck))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	res, err := Build(&buf, d, BuildOptions{Assets: assets})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return z, res
}

func zipText(t *testing.T, z *zip.Reader, name string) string {
	t.Helper()
	f, err := z.Open(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	b, _ := io.ReadAll(f)
	return string(b)
}

func oneSlide(body string) string {
	return `{"boxloom": "0.1", "slides": [{"type": "content", "title": "題", "body": [` + body + `]}]}`
}

// IR の読み込み:形の決まり(Schema と同じもの)を断る。
func TestReadBlocksIR(t *testing.T) {
	ok := []string{
		`{"type": "table", "header": ["a", "b"], "rows": [["1", ["x", {"type": "em", "content": "y"}]]], "caption": "説明"}`,
		`{"type": "image", "src": "img/tall.png", "alt": "縦長の図", "caption": "説明"}`,
		`{"type": "gallery", "images": [{"src": "wide.png", "alt": "a"}, {"src": "img/tall.png", "alt": "b"}]}`,
		`{"type": "columns", "columns": [{"title": "主", "role": "main", "body": [{"type": "paragraph", "content": "x"}]}, {"body": [{"type": "image", "src": "wide.png", "alt": "a"}]}]}`,
	}
	for _, b := range ok {
		if _, err := ReadIR("t.json", []byte(oneSlide(b))); err != nil {
			t.Errorf("%s: %v", b, err)
		}
	}
	bad := map[string]string{
		`{"type": "table", "header": ["a", "b"], "rows": [["1"]]}`:                                                                                                                                   "row has 1 cells",
		`{"type": "table", "header": ["a"], "rows": [["1"]], "align": ["right"]}`:                                                                                                                    "unknown field",
		`{"type": "image", "src": "wide.png", "alt": " "}`:                                                                                                                                           "alt",
		`{"type": "image", "src": "../wide.png", "alt": "a"}`:                                                                                                                                        "outside",
		`{"type": "image", "src": "http://example.invalid/a.png", "alt": "a"}`:                                                                                                                       "URL",
		`{"type": "gallery", "images": [{"src": "wide.png", "alt": "a"}]}`:                                                                                                                           "two to six",
		`{"type": "columns", "columns": [{"body": [{"type": "paragraph", "content": "x"}]}]}`:                                                                                                        "two or three",
		`{"type": "columns", "columns": [{"role": "main", "body": [{"type": "note", "content": "x"}]}, {"role": "main", "body": [{"type": "note", "content": "y"}]}]}`:                               "at most one",
		`{"type": "columns", "columns": [{"role": "wide", "body": [{"type": "note", "content": "x"}]}, {"body": [{"type": "note", "content": "y"}]}]}`:                                               "unknown role",
		`{"type": "columns", "columns": [{"width": 2, "body": [{"type": "note", "content": "x"}]}, {"body": [{"type": "note", "content": "y"}]}]}`:                                                   "unknown field",
		`{"type": "columns", "columns": [{"body": [{"type": "compare", "options": [{"label": "a", "pros": ["x"]}, {"label": "b", "pros": ["y"]}]}]}, {"body": [{"type": "note", "content": "y"}]}]}`: "cannot hold a compare",
	}
	for b, want := range bad {
		_, err := ReadIR("t.json", []byte(oneSlide(b)))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", b, want, err)
		}
	}
}

func TestCheckImageSrc(t *testing.T) {
	for _, s := range []string{"a.png", "img/a.png", "./a.png", "a b.png", "日本語/図.jpg"} {
		if err := checkImageSrc(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"", "/a.png", "../a.png", "img/../../a.png", `img\a.png`, "https://example.invalid/a.png", "file:a.png", "C:/a.png"} {
		if err := checkImageSrc(s); !errors.Is(err, errImagePath) {
			t.Errorf("%q: want image-path, got %v", s, err)
		}
	}
}

func TestLoadImage(t *testing.T) {
	fs := testAssets(t)
	a, err := loadImage(fs, "./img/tall.png")
	if err != nil || a.Format != "png" || a.W != 200 || a.H != 300 {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := loadImage(fs, "nope.png"); imageCode(err) != "image-missing" {
		t.Errorf("missing: %v", err)
	}
	if _, err := loadImage(fs, "note.txt"); imageCode(err) != "image-format" {
		t.Errorf("format: %v", err)
	}
	if _, err := loadImage(nil, "wide.png"); imageCode(err) != "image-missing" {
		t.Errorf("no assets: %v", err)
	}
}

// 本物の表(a:tbl)と本物の画像(p:pic)を書き、画像の部品と内容の種類を足す。
func TestBuildTableAndPictures(t *testing.T) {
	deck := oneSlide(`{"type": "table", "header": ["名前", "台数"], "rows": [["第1世代", "120台"], ["第2世代", "85台"]]},
		{"type": "gallery", "images": [{"src": "wide.png", "alt": "横長", "caption": "一"}, {"src": "img/tall.png", "alt": "縦長"}, {"src": "wide.png", "alt": "同じ画像"}]}`)
	z, _ := buildZip(t, deck, testAssets(t))
	slide := zipText(t, z, "ppt/slides/slide1.xml")
	if !strings.Contains(slide, `<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl>`) {
		t.Fatal("no a:tbl")
	}
	if n := strings.Count(slide, "<a:tr "); n != 3 {
		t.Errorf("rows: %d", n)
	}
	// 数だけの列(台数)は右に揃う。
	if !strings.Contains(slide, `algn="r"`) {
		t.Error("numeric column is not right-aligned")
	}
	if n := strings.Count(slide, "<p:pic>"); n != 3 {
		t.Errorf("pictures: %d", n)
	}
	if !strings.Contains(slide, `descr="縦長"`) {
		t.Error("alt text is not in descr")
	}
	// 同じ中身の画像は1つの部品にまとめる。
	var media []string
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "ppt/media/") {
			media = append(media, f.Name)
		}
	}
	if len(media) != 2 {
		t.Errorf("media parts: %v", media)
	}
	ct := zipText(t, z, "[Content_Types].xml")
	if !strings.Contains(ct, `<Default Extension="png" ContentType="image/png"/>`) || strings.Contains(ct, "jpeg") {
		t.Errorf("content types: %s", ct)
	}
	rels := zipText(t, z, "ppt/slides/_rels/slide1.xml.rels")
	if strings.Count(rels, "relationships/image") != 2 {
		t.Errorf("rels: %s", rels)
	}
}

// 画像を使わない文書の [Content_Types] は変わらない(png・jpeg の Default を足さない)。
func TestNoMediaWithoutImages(t *testing.T) {
	z, _ := buildZip(t, oneSlide(`{"type": "paragraph", "content": "文"}`), nil)
	if strings.Contains(zipText(t, z, "[Content_Types].xml"), "image/") {
		t.Error("image content type without images")
	}
}

var picExtRe = regexp.MustCompile(`<p:pic>[\s\S]*?<a:off x="(\d+)" y="(\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)

// 画像は縦横比を保ち(引き伸ばさない)、本文の枠に収まり、余りは右(左に寄せる)。
func TestImageKeepsAspectAndFits(t *testing.T) {
	z, _ := buildZip(t, oneSlide(`{"type": "image", "src": "img/tall.png", "alt": "縦長", "caption": "説明の文"}`), testAssets(t))
	m := picExtRe.FindStringSubmatch(zipText(t, z, "ppt/slides/slide1.xml"))
	if m == nil {
		t.Fatal("no picture")
	}
	num := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v / 12700 }
	x, y, w, h := num(m[1]), num(m[2]), num(m[3]), num(m[4])
	if math.Abs(w/h-200.0/300.0) > 0.01 {
		t.Errorf("aspect %.3f, want %.3f", w/h, 200.0/300.0)
	}
	tm, _ := LookupTemplate("light")
	if x != tm.Values.PagePadding {
		t.Errorf("x = %.1f, want the left edge %.1f", x, tm.Values.PagePadding)
	}
	if y+h > tm.Values.PageH-tm.Values.PagePadding {
		t.Errorf("picture goes below the page: y=%.1f h=%.1f", y, h)
	}
}

// 並列のまとまりの幅は意味(role: main)で決まる:主は columnMainWeight 倍。
func TestColumnsMainWeight(t *testing.T) {
	tm, _ := LookupTemplate("light")
	d, err := ReadIR("t.json", []byte(oneSlide(`{"type": "columns", "columns": [{"title": "主", "role": "main", "body": [{"type": "paragraph", "content": "x"}]}, {"body": [{"type": "paragraph", "content": "y"}]}]}`)))
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	b := built[0]
	computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	row := b.Body.Children[0]
	w0, w1 := row.Children[0].Box.Width, row.Children[1].Box.Width
	if math.Abs(w0/w1-tm.Values.ColumnMainWeight) > 0.01 {
		t.Errorf("widths %.1f / %.1f, want ratio %.1f", w0, w1, tm.Values.ColumnMainWeight)
	}
}

// 足りないとき、並列のまとまりの中の画像が先に縮み、上の段落は縮まない。
func TestColumnsImageShrinksBeforeText(t *testing.T) {
	tm, _ := LookupTemplate("light")
	tm.SetAssets(testAssets(t))
	d, err := ReadIR("t.json", []byte(oneSlide(`{"type": "paragraph", "content": "上の段落"},
		{"type": "columns", "columns": [{"body": [{"type": "image", "src": "img/tall.png", "alt": "縦長"}]}, {"body": [{"type": "paragraph", "content": "横の文"}]}]}`)))
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	b := built[0]
	computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	para := b.Body.Children[0]
	if want := tm.Values.Body * lineHeightMult; math.Abs(para.Box.Height-want) > 0.01 {
		t.Errorf("paragraph height %.2f, want %.2f (it must not shrink)", para.Box.Height, want)
	}
	if over := lintBodyOverflow(b.Root, b.Body, nil); len(over) != 0 {
		t.Errorf("unexpected overflow: %v", over)
	}
}

// 表の列の幅:収まるときは折り返さない幅の比で広げ、足りないときは最小の幅まで縮める。
func TestTableColumnWidths(t *testing.T) {
	tl := &tableLeaf{natural: []float64{100, 300}, minimum: []float64{50, 80}}
	got := tl.colWidths(800)
	if math.Abs(got[0]-200) > 1e-9 || math.Abs(got[1]-600) > 1e-9 {
		t.Errorf("wide: %v", got)
	}
	got = tl.colWidths(230)
	if math.Abs(got[0]+got[1]-230) > 1e-9 || got[0] < 50 || got[1] < 80 {
		t.Errorf("narrow: %v", got)
	}
}

// 短い見出しの列は、長い文の列があっても見出しの幅より狭くならない(1字ずつ縦に折れない)。
// 和文の4字の語(「地域中核」)も1行に置ける。守りたい幅が入らなければ従来の割り方に戻り、
// table-col-width の警告を出す。
func TestTableColumnFloor(t *testing.T) {
	long := strings.Repeat("架空の町の窓口で受け付けた相談を週ごとにまとめ、担当の課へ回す流れを説明する長い文。", 3)
	tm, _ := LookupTemplate("light")
	d, err := ReadIR("t.json", []byte(oneSlide(`{"type": "table", "header": ["規模感", "説明"], "rows": [["地域中核", "`+long+`"], ["小", "`+long+`"]]}`)))
	if err != nil {
		t.Fatal(err)
	}
	built, err := tm.BuildDeck(d)
	if err != nil {
		t.Fatal(err)
	}
	b := built[0]
	computeLayout(b.Root, 0, 0, tm.Values.PageW, tm.Values.PageH)
	node := b.Body.Children[0]
	leaf, ok := node.Subsystem.(*tableLeaf)
	if !ok {
		t.Fatalf("not a table leaf: %T", node.Subsystem)
	}
	ws := leaf.colWidths(node.Box.Width)
	head := maxLineWidth(leaf.cells[0][0].Runs, leaf.size) + 2*leaf.padX
	if ws[0] < head-0.01 {
		t.Errorf("short column %.1fpt is narrower than its heading %.1fpt", ws[0], head)
	}
	if n := runsLines(leaf.cells[1][0].Runs, leaf.size, ws[0]-2*leaf.padX); n != 1 {
		t.Errorf("「地域中核」 wraps into %d lines", n)
	}
	// 従来の割り方(floor なし)では見出しより狭かった(この試験が効いていること)。
	old := (&tableLeaf{natural: leaf.natural, minimum: leaf.minimum}).colWidths(node.Box.Width)
	if old[0] >= head {
		t.Errorf("the old split already gives %.1fpt >= %.1fpt; the test does not bite", old[0], head)
	}
	if w := leaf.Lint(LBox{Width: node.Box.Width}); len(w) != 0 {
		t.Errorf("unexpected warning: %v", w)
	}
	// 守りたい幅の合計が入らない幅:従来の割り方に戻し、警告する。
	narrow := leaf.floorSum() - 10
	got := leaf.colWidths(narrow)
	want := (&tableLeaf{natural: leaf.natural, minimum: leaf.minimum}).colWidths(narrow)
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("fallback col %d: %.2f, want %.2f", i, got[i], want[i])
		}
	}
	w := leaf.Lint(LBox{Width: narrow})
	if len(w) != 1 {
		t.Fatalf("warnings: %v", w)
	}
	if is := warningIssue("t.json", "slide 1: "+strings.Replace(w[0], "] ", "] body[0]: ", 1)); is.Code != "table-col-width" || is.Path != "/slides/0/body/0" {
		t.Errorf("issue: %+v", is)
	}
}

func TestNumericColumns(t *testing.T) {
	cells := func(ss ...string) []IRTableCell {
		var out []IRTableCell
		for _, x := range ss {
			out = append(out, IRTableCell{Text: plainInline(x)})
		}
		return out
	}
	rows := [][]IRTableCell{
		cells("第1世代", "120台", "2027-03", "1,200", "12.5%"),
		cells("第2世代", "85台", "—", "", "-3%"),
	}
	got := numericColumns(rows, 5)
	want := []bool{false, true, false, true, true}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d: %v, want %v", i, got[i], want[i])
		}
	}
}

// 画像のファイルの誤りは check が決まった code と場所で返す(Assets を渡さないと読めない)。
func TestCheckImageAssets(t *testing.T) {
	src := []byte(oneSlide(`{"type": "image", "src": "nope.png", "alt": "a"}, {"type": "gallery", "images": [{"src": "wide.png", "alt": "b"}, {"src": "note.txt", "alt": "c"}]}`))
	res := Check("t.json", src, CheckOptions{Assets: testAssets(t)})
	var got []string
	for _, e := range res.Errors {
		got = append(got, e.Code+" "+e.Path)
	}
	want := "image-missing /slides/0/body/0/src,image-format /slides/0/body/1/images/1/src"
	if strings.Join(got, ",") != want {
		t.Errorf("got %v", got)
	}
	ok := Check("t.json", []byte(oneSlide(`{"type": "image", "src": "wide.png", "alt": "a"}`)), CheckOptions{})
	if len(ok.Errors) != 1 || ok.Errors[0].Code != "image-missing" {
		t.Errorf("without assets: %+v", ok.Errors)
	}
}

// Markdown:表(説明の文つき)・画像・画像の並び・並列のまとまり。
func TestMarkdownBlocks(t *testing.T) {
	src := "# 題\n\n| 名前 | 台数 |\n| --- | --- |\n| `a\\|b` | 3台 |\n: 表の**説明**\n\n" +
		"![横長の図](wide.png \"図の説明\")\n\n" +
		"![一](wide.png)\n![二](img/tall.png)\n\n" +
		"::::columns\n:::column 主 {role=main}\n- 項目\n:::\n:::column\n![縦長](img/tall.png)\n:::\n::::\n"
	got, errs := mdToAny(t, src)
	if len(errs) > 0 {
		t.Fatal(MarkdownErrors(errs))
	}
	body := got.(map[string]any)["slides"].([]any)[0].(map[string]any)["body"].([]any)
	tb := body[0].(map[string]any)
	if tb["type"] != "table" || tb["rows"].([]any)[0].([]any)[0].([]any)[0].(map[string]any)["text"] != "a|b" {
		t.Errorf("table: %#v", tb)
	}
	if tb["caption"].([]any)[1].(map[string]any)["type"] != "em" {
		t.Errorf("caption: %#v", tb["caption"])
	}
	im := body[1].(map[string]any)
	if im["type"] != "image" || im["alt"] != "横長の図" || im["caption"] != "図の説明" {
		t.Errorf("image: %#v", im)
	}
	g := body[2].(map[string]any)
	if g["type"] != "gallery" || len(g["images"].([]any)) != 2 {
		t.Errorf("gallery: %#v", g)
	}
	cols := body[3].(map[string]any)["columns"].([]any)
	c0, c1 := cols[0].(map[string]any), cols[1].(map[string]any)
	if c0["title"] != "主" || c0["role"] != "main" || c1["title"] != nil || c1["body"].([]any)[0].(map[string]any)["type"] != "image" {
		t.Errorf("columns: %#v", cols)
	}
}

// 例の文書(examples/md/blocks.bx.md)は、画像を文書のフォルダから読んで組める。
func TestBlocksExampleBuilds(t *testing.T) {
	for _, tpl := range []string{"light", "dark"} {
		errs, warns, _ := CheckFileCapacity("../examples/md/blocks.bx.md", tpl)
		if len(errs) > 0 {
			t.Fatalf("%s: %+v", tpl, errs)
		}
		for _, w := range warns {
			// 横長の画像を1つだけ置いたスライド(/slides/4)は、画像の横が空く例でもある。
			if w.Code != "fill-ratio" && !(w.Code == "spare-width" && w.Path == "/slides/4/body/0") {
				t.Errorf("%s: unexpected warning %+v", tpl, w)
			}
		}
	}
	if _, err := os.Stat("../examples/ir/images/screen-wide.png"); err != nil {
		t.Fatal("the IR example needs its own images/ (src cannot leave the deck's folder)")
	}
}
