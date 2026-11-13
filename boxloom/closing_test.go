package boxloom

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// 表紙の写真(meta.coverImage)と終わりのスライド(meta.closing。Backlog #92)。

func coverAssets(t *testing.T) fstest.MapFS {
	return fstest.MapFS{
		"img/cover.png": {Data: pngOf(t, 320, 180)},
		"note.txt":      {Data: []byte("not an image")},
	}
}

const coverDeckMD = `---
title: 架空の街の調べ
subtitle: 利用の動き
date: 2026-11
coverImage: img/cover.png
closing: true
---

# 朝に集まる

- 朝の利用が多い
`

func coverDeckJSON(template, extra string) string {
	return fmt.Sprintf(`{"boxloom": "0.1", "template": %q,
  "meta": {"title": "架空の街の調べ", "subtitle": "利用の動き", "date": "2026-11"%s},
  "slides": [
    {"type": "cover"},
    {"type": "content", "title": "朝に集まる", "body": [{"type": "points", "items": [{"content": "朝の利用が多い"}]}]}
  ]}`, template, extra)
}

// Markdown の文書の頭の coverImage・closing は IR の meta.coverImage・meta.closing になる。
func TestCoverImageMarkdown(t *testing.T) {
	d, issues, err := ParseMarkdown("t.bx.md", []byte(coverDeckMD))
	if err != nil || len(issues) > 0 {
		t.Fatalf("parse: %v %v", err, issues)
	}
	if d.Meta.CoverImage != "img/cover.png" || !d.Meta.Closing.On {
		t.Fatalf("meta = %+v", d.Meta)
	}
	for _, c := range []struct{ line, code string }{
		{"coverImage: ../cover.png", "image-path"},
		{"coverImage: https://example.com/a.png", "image-path"},
		{"closing: yes", "invalid-value"},
	} {
		src := strings.Replace(coverDeckMD, "coverImage: img/cover.png", c.line, 1)
		_, issues, _ := ParseMarkdown("t.bx.md", []byte(src))
		found := false
		for _, is := range issues {
			found = found || is.Code == c.code
		}
		if !found {
			t.Errorf("%s: want %s, got %v", c.line, c.code, issues)
		}
	}
}

// IR の meta.coverImage・meta.closing を読み、Schema の出力にも載る。
func TestCoverImageIRAndSchema(t *testing.T) {
	d, err := ReadIR("t.json", []byte(coverDeckJSON("report", `, "coverImage": "img/cover.png", "closing": true`)))
	if err != nil {
		t.Fatal(err)
	}
	if d.Meta.CoverImage != "img/cover.png" || !d.Meta.Closing.On {
		t.Fatalf("meta = %+v", d.Meta)
	}
	out, err := Schema("report")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{`"coverImage"`, `"closing"`, "meta.coverImage", "meta.closing"} {
		if !strings.Contains(string(out), k) {
			t.Errorf("schema output lacks %s", k)
		}
	}
}

var reBgBlip = regexp.MustCompile(`<p:bg><p:bgPr><a:blipFill[^>]*><a:blip r:embed="(rId\d+)"/>(<a:srcRect[^>]*/>)?<a:stretch>`)

func buildCover(t *testing.T, template, extra string) map[string]string {
	t.Helper()
	d, err := ReadIR("t.json", []byte(coverDeckJSON(template, extra)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Build(&buf, d, BuildOptions{Assets: coverAssets(t)}); err != nil {
		t.Fatal(err)
	}
	return zipFiles(t, buf.Bytes())
}

// report の表紙は写真の上に基調色の箱(左の縦の中ほど、幅 0.53・高さ 0.45 以上)、箱の中に
// 白い字の題・副題・日付(#109)。終わりのスライドは同じ写真に同じ位置の箱だけ。
func TestReportCoverAndClosing(t *testing.T) {
	files := buildCover(t, "report", `, "coverImage": "img/cover.png", "closing": true`)
	tm, _ := LookupTemplate("report")
	W, H := tm.Values.PageW, tm.Values.PageH
	if _, ok := files["ppt/slides/slide3.xml"]; !ok {
		t.Fatal("no closing slide (slide3)")
	}
	if _, ok := files["ppt/slides/slide4.xml"]; ok {
		t.Fatal("unexpected slide4")
	}
	accent, bg := hexColor(tm.Theme["accent"]), hexColor(tm.Theme["bg"])
	reBox := regexp.MustCompile(fmt.Sprintf(`<a:off x="0" y="(\d+)"/><a:ext cx="%d" cy="(\d+)"/>`, ptToEMU(W*reportCoverBoxW)))
	boxes := map[int][2]float64{}
	media := ""
	for _, n := range []int{1, 3} {
		x := files[fmt.Sprintf("ppt/slides/slide%d.xml", n)]
		m := reBgBlip.FindStringSubmatch(x)
		if m == nil {
			t.Fatalf("slide %d: no photo background:\n%s", n, x)
		}
		// 320×180 は 960×540 と同じ縦横比なので切らない。
		if m[2] != "" {
			t.Errorf("slide %d: unexpected crop %s", n, m[2])
		}
		rels := files[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", n)]
		rm := regexp.MustCompile(`Id="` + m[1] + `" Type="[^"]+/image" Target="\.\./media/([^"]+)"`).FindStringSubmatch(rels)
		if rm == nil {
			t.Fatalf("slide %d: no image relationship %s:\n%s", n, m[1], rels)
		}
		if media != "" && rm[1] != media {
			t.Errorf("slide %d: photo %s, want the same part %s", n, rm[1], media)
		}
		media = rm[1]
		bm := reBox.FindStringSubmatch(x)
		if bm == nil || !strings.Contains(x, `<a:srgbClr val="`+accent+`"/>`) {
			t.Fatalf("slide %d: no accent box", n)
		}
		y, _ := strconv.ParseFloat(bm[1], 64)
		h, _ := strconv.ParseFloat(bm[2], 64)
		y, h = y/12700, h/12700
		boxes[n] = [2]float64{y, h}
		if h < H*reportCoverBoxH-0.1 || y < H*reportCoverMinGap-0.1 || H-(y+h) < H*reportCoverMinGap-0.1 {
			t.Errorf("slide %d: box y %.1f h %.1f touches the edge or is too small", n, y, h)
		}
	}
	cover := files["ppt/slides/slide1.xml"]
	for _, s := range []string{"架空の街の調べ", "利用の動き"} {
		i := strings.Index(cover, s)
		if i < 0 {
			t.Fatalf("cover lacks %s", s)
		}
		// 題・副題の塊は白(役割名 bg)。
		if j := strings.LastIndex(cover[:i], "<a:srgbClr val="); j < 0 || cover[j+16:j+22] != bg {
			t.Errorf("cover: %s is not drawn in bg (%s)", s, bg)
		}
	}
	if !strings.Contains(cover, `type="ctrTitle"`) || !strings.Contains(cover, `type="subTitle"`) {
		t.Error("cover: title and subtitle are not placeholders")
	}
	// 日付は箱の中(白い字)。
	dump := coverLayout(t, "report", `, "coverImage": "img/cover.png", "closing": true`)
	date := findPath(dump.Slides[0].Root, "/meta/date")
	b := boxes[1]
	if date == nil || date.Box.Y < b[0]-0.1 || date.Box.Y+date.Box.Height > b[0]+b[1]+0.1 || date.Box.X+date.Box.Width > W*reportCoverBoxW+0.1 {
		t.Errorf("date is not in the box %v: %+v", b, date)
	}
	if got := dump.Slides[2]; got.Path != "/meta/closing" || got.Type != "closing" {
		t.Errorf("closing slide in layout = %s %s", got.Path, got.Type)
	}
	// 終わりのスライドは箱だけ(文字を持たない)。
	if strings.Contains(files["ppt/slides/slide3.xml"], "<a:t>") {
		t.Error("closing slide has text")
	}
}

// 標準の表紙(light・dark)は、写真があれば全面に敷き、今の見た目を重ねる。終わりの
// スライドは地の色だけで、写真があれば敷く。
func TestStandardCoverImageAndClosing(t *testing.T) {
	for _, name := range []string{"light", "dark"} {
		plain := buildCover(t, name, "")
		photo := buildCover(t, name, `, "coverImage": "img/cover.png", "closing": true`)
		if !reBgBlip.MatchString(photo["ppt/slides/slide1.xml"]) || !reBgBlip.MatchString(photo["ppt/slides/slide3.xml"]) {
			t.Fatalf("%s: cover/closing lack the photo", name)
		}
		// 写真の地を除けば、表紙の図形は写真の無いときと同じ。
		spTree := regexp.MustCompile(`(?s)<p:spTree>.*</p:spTree>`)
		if a, b := spTree.FindString(plain["ppt/slides/slide1.xml"]), spTree.FindString(photo["ppt/slides/slide1.xml"]); a != b {
			t.Errorf("%s: the cover's shapes changed with the photo", name)
		}
		if strings.Contains(photo["ppt/slides/slide3.xml"], "<p:sp>") {
			t.Errorf("%s: the standard closing slide has shapes", name)
		}
		// 写真の無い終わりのスライドは地の色だけ(マスターの地を引き継ぐ)。
		closing := buildCover(t, name, `, "closing": true`)
		x := closing["ppt/slides/slide3.xml"]
		if strings.Contains(x, "<p:bg>") || strings.Contains(x, "<p:sp>") {
			t.Errorf("%s: closing without photo should be the plain slide:\n%s", name, x)
		}
		if _, ok := plain["ppt/slides/slide3.xml"]; ok {
			t.Errorf("%s: closing slide without meta.closing", name)
		}
	}
}

// 写真の縦横比がページと違えば、長い方の両端を同じだけ切る。
func TestCoverCrop(t *testing.T) {
	wide := &imageAsset{W: 400, H: 100}
	if got := coverCropXML(wide, 960, 540); got != `<a:srcRect l="27778" r="27778"/>` {
		t.Errorf("wide: %s", got)
	}
	tall := &imageAsset{W: 100, H: 100}
	if got := coverCropXML(tall, 960, 540); got != `<a:srcRect t="21875" b="21875"/>` {
		t.Errorf("tall: %s", got)
	}
	if got := coverCropXML(&imageAsset{W: 16, H: 9}, 960, 540); got != "" {
		t.Errorf("same aspect: %s", got)
	}
}

// check は表紙の写真を画像と同じ決まりで見る(場所は /meta/coverImage)。
func TestCheckCoverImage(t *testing.T) {
	for _, c := range []struct{ src, code string }{
		{"../cover.png", "image-path"},
		{"img/none.png", "image-missing"},
		{"note.txt", "image-format"},
	} {
		res := Check("t.json", []byte(coverDeckJSON("report", `, "coverImage": "`+c.src+`"`)), CheckOptions{Assets: coverAssets(t)})
		found := false
		for _, is := range res.Errors {
			found = found || (is.Code == c.code && is.Path == "/meta/coverImage")
		}
		if !found {
			t.Errorf("%s: want %s at /meta/coverImage, got %+v", c.src, c.code, res.Errors)
		}
	}
	res := Check("t.json", []byte(coverDeckJSON("report", `, "coverImage": "img/cover.png", "closing": true`)), CheckOptions{Assets: coverAssets(t)})
	if len(res.Errors) > 0 {
		t.Errorf("errors: %+v", res.Errors)
	}
}

func coverLayout(t *testing.T, template, extra string) *LayoutDump {
	t.Helper()
	out, err := Layout("t.json", []byte(coverDeckJSON(template, extra)), BuildOptions{Assets: coverAssets(t)})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func findPath(n *LayoutNode, path string) *LayoutNode {
	if n == nil {
		return nil
	}
	if n.Path == path {
		return n
	}
	for i := range n.Children {
		if f := findPath(n.Children[i], path); f != nil {
			return f
		}
	}
	return nil
}
