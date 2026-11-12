// boxloom check: JSON Schema では書けない IR の検査。
//
//	go run ./cmd/boxloom check [-format text|json] [-capacity] [-template dark|light|report] [-strict] deck.json|deck.yaml|deck.bx.md ...
//
// 誤りと警告は docs/content-syntax.md の5章の形(Issue)で出す:
//
//	{"ok": false,
//	 "errors":   [{"file": "deck.bx.md", "line": 40, "path": "/slides/2/body/0/data/...", "code": "date-order", "message": "..."}],
//	 "warnings": [{"file": "deck.json", "path": "/slides/1", "code": "fill-ratio", "message": "..."}],
//	 "capacity": [{"file": "deck.json", "path": "/slides/1", "bodyArea": 413.4, "need": 210.2, ...}]}
//
// capacity は本文を持つスライドごとの容量(capacity.go、docs/ir.md の「容量の計測」)。
//
// IR(JSON)を検査したときは line・column を出さない。.yaml・.yml(IR の写し)
// を検査したときは、IR の場所から引いた YAML の line・column を添える。.md を検査したときは、
// 記法の誤りならパーサーの誤り(line・column つき)を、意味の誤りなら IR の
// 場所から引いた Markdown の行(line だけ)を添える。
//
// 誤りは最初の1件で止めず、全部集める。誤りが無ければ組み立て(build)まで通し、
// そのときの警告(本文の埋まり具合・サブシステムの最小の大きさ・注釈の重なり・
// テーマに無い役割名)も code つきで出す。終了コードは 0=誤りなし、1=誤りあり
// (-strict なら警告もこちら)、2=使い方の誤り。
//
// 形(キー・型・enum)の検査は Schema(tools/validate_ir.py)の仕事。ここは
// 「形は正しいが意味が通らない」もの(id の重なり・指す先の無い target・
// 日付の前後・範囲の外の日付・暦に無い日付・テンプレートに無い variant)を見る。
// Go の IR の読み込み(ir.go)が断るもの(知らないキーなど)も、場所つきの
// 誤りとして出す。

package boxloom

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CheckResult は1回の check の結果。誤りと警告は Issue(docs/content-syntax.md
// の5章の形)。
type CheckResult struct {
	OK       bool    `json:"ok"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
	// Capacity は本文を持つスライドごとの容量(capacity.go)。誤りがあって
	// 組み立てまで通らなかったファイルの分は入らない。
	Capacity []SlideCapacity `json:"capacity"`
}

// CheckOptions は Check の選択。
type CheckOptions struct {
	// Template は IR の template より優先するテンプレートの名前(空なら IR の
	// template、それも無ければ light)。
	Template string
	// Values は外から読んだテンプレートの値の表(ParseTemplate)。nil でなければ
	// Template と IR の template より優先する。
	Values *Template
	// Strict は警告も誤りとして扱う(CheckResult.OK が false になる)。
	Strict bool
	// Assets は画像(image・gallery の src)を読む fs.FS。根は文書のフォルダ。
	// nil なら画像を使う文書は image-missing になる(BuildOptions.Assets と同じ)。
	Assets fs.FS
}

// Check は1つの文書(file の名前、data の中身)を検査する。file の拡張子が
// .md なら Boxloom Markdown として読み、記法の誤りがあればそれを返す。無ければ
// できた IR を検査し、誤りと警告に Markdown の行を添える(IR の場所から、その
// 場所を作った Markdown の行を引く)。.yaml・.yml なら YAML の deck(IR の写し)
// として読み、誤りと警告に YAML の行と列を添える。どちらでもなければ IR(JSON)
// として読む。
//
// 誤りが無ければ組み立て(build)まで通し、そのときの警告と本文の容量も返す。
// 返す CheckResult の切れ端は nil にならない(JSON で [] になる)。
func Check(file string, data []byte, opt CheckOptions) *CheckResult {
	defer useFonts(activeFonts)() // 組み立てが替えた字体を、終わったら戻す
	errs, warns, caps := checkCapacity(file, data, opt.Template, opt.Values, opt.Assets)
	res := &CheckResult{Errors: []Issue{}, Warnings: []Issue{}, Capacity: []SlideCapacity{}}
	res.Errors = append(res.Errors, errs...)
	res.Warnings = append(res.Warnings, warns...)
	res.Capacity = append(res.Capacity, caps...)
	res.OK = len(res.Errors) == 0 && (!opt.Strict || len(res.Warnings) == 0)
	return res
}

// checkCapacity は Check の中身。.md なら容量のスライドとブロックにも
// Markdown の行を添える。
func checkCapacity(file string, b []byte, templateOverride string, values *Template, assets fs.FS) (errs, warns []Issue, caps []SlideCapacity) {
	if isYAMLPath(file) {
		return checkYAMLCapacity(file, b, templateOverride, values, assets)
	}
	if !isMarkdownPath(file) {
		return checkIRCapacity(file, b, templateOverride, values, assets)
	}
	ir, mdErrs, lines := ParseMarkdownWithMap(b, file)
	if len(mdErrs) > 0 {
		return mdErrs, nil, nil
	}
	errs, warns, caps = checkIRCapacity(file, ir, templateOverride, values, assets)
	for _, list := range [][]Issue{errs, warns} {
		for i := range list {
			list[i].Line = lines.Line(list[i].Path)
		}
	}
	addCapacityLines(caps, lines)
	return errs, warns, caps
}

// checkYAMLCapacity は YAML の deck の check。記法の誤りはそのまま返し、
// 無ければ IR として検査して、誤りと警告に YAML の行と列を、容量に行を添える。
func checkYAMLCapacity(file string, b []byte, templateOverride string, values *Template, assets fs.FS) (errs, warns []Issue, caps []SlideCapacity) {
	ir, yErrs, pos := parseYAMLDeck(file, b)
	if len(yErrs) > 0 {
		return yErrs, nil, nil
	}
	errs, warns, caps = checkIRCapacity(file, ir, templateOverride, values, assets)
	for _, list := range [][]Issue{errs, warns} {
		for i := range list {
			list[i].Line, list[i].Column = pos.Pos(list[i].Path)
			if list[i].Code == codeSchema && list[i].Hint == "" {
				list[i].Hint = yamlTypeHint(list[i].Message)
			}
		}
	}
	addCapacityLines(caps, pos)
	return errs, warns, caps
}

// addCapacityLines は容量のスライド・ブロック・列に、元の文書の行を添える。
func addCapacityLines(caps []SlideCapacity, lines interface{ Line(string) int }) {
	for i := range caps {
		caps[i].Line = lines.Line(caps[i].Path)
		for j := range caps[i].Blocks {
			bc := &caps[i].Blocks[j]
			bc.Line = lines.Line(bc.Path)
			for k := range bc.Pairs {
				bc.Pairs[k].Line = lines.Line(bc.Pairs[k].Path)
			}
			for k := range bc.Columns {
				cc := &bc.Columns[k]
				cc.Line = lines.Line(cc.Path)
				for m := range cc.Blocks {
					cc.Blocks[m].Line = lines.Line(cc.Blocks[m].Path)
				}
			}
		}
	}
}

// CheckIR はバイト列の IR を検査する(試験から直接呼べるように分けてある)。
func CheckIR(file string, b []byte, templateOverride string) (errs, warns []Issue) {
	errs, warns, _ = CheckIRCapacity(file, b, templateOverride)
	return errs, warns
}

// CheckIRCapacity は CheckIR に容量を足したもの。誤りがあれば容量は空。
// 画像のファイルは読めない(画像を使う文書は image-missing。読ませるなら Check と
// CheckOptions.Assets を使う)。
func CheckIRCapacity(file string, b []byte, templateOverride string) (errs, warns []Issue, caps []SlideCapacity) {
	return checkIRCapacity(file, b, templateOverride, nil, nil)
}

// checkIRCapacity は CheckIRCapacity の中身。values(外から読んだ値の表)が
// あれば、名前より優先する。assets は画像を読む fs.FS。
func checkIRCapacity(file string, b []byte, templateOverride string, values *Template, assets fs.FS) (errs, warns []Issue, caps []SlideCapacity) {
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		return []Issue{{File: file, Path: "", Message: "not JSON: " + err.Error(), Code: "json"}}, nil, nil
	}
	c := &checker{file: file, images: &imageCache{fsys: assets}}

	// テンプレート(variant の検査と組み立てに使う)。
	name, _ := getPath(doc, "template").(string)
	at := "/template"
	if templateOverride != "" {
		name, at = templateOverride, ""
	}
	tm, terr := values, error(nil)
	if tm == nil {
		tm, terr = LookupTemplate(name)
	}
	if terr != nil {
		c.add(at, terr.Error(), "template-exists")
	}

	c.walkDoc(doc, tm)

	// Go の読み込みが断るもの(知らないキー・必須の欄など)。意味の検査と同じ
	// 誤りを二重に出さないよう、意味の誤りが無いときだけ見る。
	if len(c.errs) > 0 {
		return c.errs, nil, nil
	}
	deck, perr := ParseIR(b)
	if perr != nil {
		c.add(errPointer(perr), perr.Error(), "schema")
		return c.errs, nil, nil
	}
	tm.images = c.images // check で読んだ画像をそのまま使う
	pages, warnings, caps, berr := tm.renderPages(deck)
	if berr == nil {
		berr = WriteDeck(io.Discard, pages, "")
	}
	if berr != nil {
		c.add(locationToPointer(berr.Error()), berr.Error(), "build")
		return c.errs, nil, nil
	}
	// テンプレートを指さなければ(-template も値の表も IR の template も無い)、型の語彙
	// だけで検査する:組み方の無い型を代わりで組んだこと(component-fallback)は言わない。
	designated := values != nil || templateOverride != "" || getPath(doc, "template") != nil
	for _, w := range warnings {
		is := warningIssue(file, w)
		if is.Code == "component-fallback" && !designated {
			continue
		}
		warns = append(warns, is)
	}
	// 代わりの組み方で組んだブロックの中の場所(書き換えた columns/…)は、元の IR の
	// 場所に写す。hint は写した容量から作る。
	if fb := fallbackBlocks(deck, tm); len(fb) > 0 {
		for i := range warns {
			warns[i].Path = remapFallbackPath(warns[i].Path, fb)
		}
		remapFallbackCapacity(caps, fb)
	}
	addLayoutHints(warns, caps)
	for i := range caps {
		caps[i].File = file
	}
	return c.errs, warns, caps
}

type checker struct {
	file   string
	errs   []Issue
	images *imageCache
}

func (c *checker) add(at, msg, rule string) {
	c.errs = append(c.errs, Issue{File: c.file, Path: at, Message: msg, Code: rule})
}

func (c *checker) addIssue(is Issue) {
	is.File = c.file
	c.errs = append(c.errs, is)
}

// --- 文書の歩き方 ---

func (c *checker) walkDoc(doc any, tm *Template) {
	// id は文書全体で一意(Schema の id の説明どおり)。
	ids := map[string]string{} // id → 最初に出た場所
	walkAll(doc, "", func(ptr string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		id, ok := m["id"].(string)
		if !ok {
			return
		}
		if first, dup := ids[id]; dup {
			c.add(ptr+"/id", fmt.Sprintf("id %q is already used at %s", id, first), "unique-id")
			return
		}
		ids[id] = ptr
	})

	if meta, ok := getPath(doc, "meta").(map[string]any); ok {
		c.checkNotEmpty(meta, "/meta", "confidential", "disclaimer")
	}
	slides, _ := getPath(doc, "slides").([]any)
	for si, s := range slides {
		sp := fmt.Sprintf("/slides/%d", si)
		sm, _ := s.(map[string]any)
		c.checkVariant(sm, sp, tm)
		switch sm["type"] {
		case "section":
			c.checkNotEmpty(sm, sp, "title")
		case "statement":
			c.checkNotEmpty(sm, sp, "content")
		}
		c.checkNotEmpty(sm, sp, "source", "lead")
		body, _ := sm["body"].([]any)
		for bi, blk := range body {
			bp := fmt.Sprintf("%s/body/%d", sp, bi)
			bm, _ := blk.(map[string]any)
			c.checkBodyBlock(bm, bp, tm, ids)
		}
	}

	// 日付の範囲(from/to)は、どこにあっても前後を見る。
	walkAll(doc, "", func(ptr string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		fs, ok1 := m["from"].(string)
		ts, ok2 := m["to"].(string)
		if !ok1 || !ok2 {
			return
		}
		f, e1 := parseIRDate(fs)
		t, e2 := parseIRDate(ts)
		if e1 != nil || e2 != nil {
			return // 日付そのものの誤りは checkDates が出す
		}
		if !f.Before(t) {
			c.add(ptr, fmt.Sprintf("from (%s) must be before to (%s)", fs, ts), "date-order")
		}
	})
	c.checkDates(doc)
}

// checkBodyBlock は本文の1つのブロックの意味の検査。標準でない型は、型の語彙の
// Inner が返す中のブロックと欄を、IR の場所つきで同じ検査に通す。
func (c *checker) checkBodyBlock(bm map[string]any, bp string, tm *Template, ids map[string]string) {
	c.checkVariant(bm, bp, tm)
	c.checkBlock(bm, bp, tm)
	switch bm["type"] {
	case "takeaway":
		c.checkNotEmpty(bm, bp, "content")
	case "quote":
		c.checkNotEmpty(bm, bp, "content", "source")
	case "stat":
		items, _ := bm["items"].([]any)
		for ii, it := range items {
			im, _ := it.(map[string]any)
			c.checkNotEmpty(im, fmt.Sprintf("%s/items/%d", bp, ii), "label")
		}
	}
	if bm["type"] == "subsystem" {
		if data, ok := bm["data"].(map[string]any); ok {
			switch bm["subsystem"] {
			case "gantt":
				c.checkGantt(data, bp+"/data", ids)
			case "shapes":
				c.checkShapes(data, bp+"/data", ids, tm)
			case "matrix":
				c.checkMatrix(data, bp+"/data")
			}
		}
	}
	if name, ok := bm["type"].(string); ok && !blockTypes.isStandard(name) {
		if d, ok := blockTypes.decl(name); ok && d.Inner != nil {
			for _, part := range d.Inner(bm) {
				p := bp + part.Path
				if part.Block {
					c.checkBodyBlock(part.Map, p, tm, ids)
				}
				c.checkNotEmpty(part.Map, p, part.Texts...)
			}
		}
	}
}

// checkBlock は表・画像・並列のまとまりの、Schema で書けない決まりを見る:
// 表の行のセルの数(table-shape)、画像のファイル(image-path・image-missing・
// image-format)、まとまりの中のブロック(同じ検査と variant)。
func (c *checker) checkBlock(bm map[string]any, bp string, tm *Template) {
	img := func(m map[string]any, p string) {
		if alt, ok := m["alt"].(string); ok && strings.TrimSpace(alt) == "" {
			c.addIssue(Issue{Path: p + "/alt", Code: "image-alt", Message: "the image has no alternative text (alt)",
				Hint: "画像が何を示すかを1文で書いてください(読み上げと、画像が無いときに使います)"})
		}
		src, ok := m["src"].(string)
		if !ok {
			return
		}
		if _, err := c.images.get(src); err != nil {
			code := imageCode(err)
			if code == "" {
				code = "image-missing"
			}
			hint := "画像は文書のファイルと同じフォルダ(かその下)に置き、文書からの相対パスで書きます"
			if code == "image-format" {
				hint = "PNG か JPEG にしてください"
			}
			c.addIssue(Issue{Path: p + "/src", Code: code, Message: err.Error(), Hint: hint})
		}
	}
	switch bm["type"] {
	case "subsystem":
		// グラフは本文にも並列のまとまりの中にも置ける。
		if bm["subsystem"] == "chart" {
			if data, ok := bm["data"].(map[string]any); ok {
				c.checkChart(data, bp+"/data")
			}
		}
	case "table":
		header, _ := bm["header"].([]any)
		rows, _ := bm["rows"].([]any)
		groups, grouped := bm["groups"].([]any)
		want := len(header)
		if grouped {
			want--
			c.checkTableGroups(groups, len(header), len(rows), bp)
		}
		for ri, r := range rows {
			cells, ok := r.([]any)
			if ok && len(header) > 0 && len(cells) != want {
				msg := fmt.Sprintf("row has %d cells but the header has %d (every row has one cell per column)", len(cells), len(header))
				if grouped {
					msg = fmt.Sprintf("row has %d cells but the header has %d (with groups, every row has one cell per column except the group column: %d)", len(cells), len(header), want)
				}
				c.add(fmt.Sprintf("%s/rows/%d", bp, ri), msg, "table-shape")
			}
		}
		if cells, ok := bm["total"].([]any); ok && len(header) > 0 && len(cells) != len(header) {
			c.add(bp+"/total", fmt.Sprintf("the total row has %d cells but the header has %d (every row has one cell per column)", len(cells), len(header)), "table-shape")
		}
	case "image":
		img(bm, bp)
	case "gallery":
		images, _ := bm["images"].([]any)
		for i, x := range images {
			if m, ok := x.(map[string]any); ok {
				img(m, fmt.Sprintf("%s/images/%d", bp, i))
			}
		}
	case "columns":
		cols, _ := bm["columns"].([]any)
		for ci, col := range cols {
			cm, _ := col.(map[string]any)
			body, _ := cm["body"].([]any)
			for k, blk := range body {
				p := fmt.Sprintf("%s/columns/%d/body/%d", bp, ci, k)
				m, _ := blk.(map[string]any)
				c.checkVariant(m, p, tm)
				if m["type"] != "columns" {
					c.checkBlock(m, p, tm)
				}
			}
		}
	}
}

// checkNotEmpty は、意味がその文だけで決まる欄(章の題・主張の1文・結論・引用の
// 言葉と出どころ・数の説明)に、見える文字があるかを見る(empty-text)。Schema は
// インラインの "" や空白だけの文字列を通してしまう。
func (c *checker) checkNotEmpty(m map[string]any, ptr string, keys ...string) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue // 欠けた欄は Schema と IR の読み込みが断る
		}
		if strings.TrimSpace(inlinePlainAny(v)) == "" {
			c.add(ptr+"/"+k, fmt.Sprintf("%s has no visible text", k), "empty-text")
		}
	}
}

// inlinePlainAny は JSON のままのインライン(文字列か、文字列と要素の並び)の文字。
func inlinePlainAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, p := range x {
			b.WriteString(inlinePlainAny(p))
		}
		return b.String()
	case map[string]any:
		if t, ok := x["text"].(string); ok {
			return t
		}
		return inlinePlainAny(x["content"])
	}
	return ""
}

func (c *checker) checkVariant(m map[string]any, ptr string, tm *Template) {
	v, ok := m["variant"].(string)
	if !ok || tm == nil {
		return
	}
	if !tm.Variants[v] {
		known := make([]string, 0, len(tm.Variants))
		for k := range tm.Variants {
			known = append(known, k)
		}
		sort.Strings(known)
		c.add(ptr+"/variant", fmt.Sprintf("template %q has no variant %q (known: %v)", tm.Name, v, known), "variant-exists")
	}
}

// checkDates は日付の形をした文字列が、暦の上で実在する日かを見る
// (Schema の正規表現は 2026-02-30 を通してしまう)。
var irDateRe = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])(-(0[1-9]|[12][0-9]|3[01]))?$`)

func (c *checker) checkDates(doc any) {
	walkAll(doc, "", func(ptr string, v any) {
		s, ok := v.(string)
		if !ok || !irDateRe.MatchString(s) {
			return
		}
		if _, err := parseIRDate(s); err != nil {
			c.add(ptr, fmt.Sprintf("%q is not a real date", s), "date-valid")
		}
	})
}

// checkGantt は Gantt の data の意味を見る: target の指す先と、範囲(span)の
// 外にはみ出す日付。
func (c *checker) checkGantt(data map[string]any, ptr string, docIDs map[string]string) {
	// このチャートの中の id(グループと行)。
	local := map[string]bool{}
	groups, _ := data["groups"].([]any)
	for _, g := range groups {
		gm, _ := g.(map[string]any)
		if id, ok := gm["id"].(string); ok {
			local[id] = true
		}
		rows, _ := gm["rows"].([]any)
		for _, r := range rows {
			rm, _ := r.(map[string]any)
			if id, ok := rm["id"].(string); ok {
				local[id] = true
			}
		}
	}
	anns, _ := data["annotations"].([]any)
	for i, a := range anns {
		am, _ := a.(map[string]any)
		tgt, ok := am["target"].(string)
		if !ok || local[tgt] {
			continue
		}
		ap := fmt.Sprintf("%s/annotations/%d/target", ptr, i)
		if where, elsewhere := docIDs[tgt]; elsewhere {
			c.add(ap, fmt.Sprintf("target %q is not a group or row of this chart (it is at %s)", tgt, where), "target-exists")
		} else {
			c.add(ap, fmt.Sprintf("no group or row has id %q", tgt), "target-exists")
		}
	}

	span, _ := data["span"].(map[string]any)
	sf, e1 := dateAt(span, "from")
	st, e2 := dateAt(span, "to")
	if e1 != nil || e2 != nil || !sf.Before(st) {
		return // 範囲そのものが壊れていれば、はみ出しは測れない(ほかの規則が出す)
	}
	within := func(p, s string) {
		t, err := parseIRDate(s)
		if err != nil {
			return
		}
		if t.Before(sf) || t.After(st) {
			c.add(p, fmt.Sprintf("%s is outside the chart span %s .. %s", s, fmtYM(sf), fmtYM(st)), "within-span")
		}
	}
	if s, ok := data["axisBreakBefore"].(string); ok {
		within(ptr+"/axisBreakBefore", s)
	}
	for _, key := range []string{"ticks", "milestones", "annotations"} {
		arr, _ := data[key].([]any)
		for i, x := range arr {
			xm, _ := x.(map[string]any)
			if s, ok := xm["at"].(string); ok {
				within(fmt.Sprintf("%s/%s/%d/at", ptr, key, i), s)
			}
		}
	}
	rangeWithin := func(p string, m map[string]any) {
		for _, k := range []string{"from", "to"} {
			if s, ok := m[k].(string); ok {
				within(p+"/"+k, s)
			}
		}
	}
	for gi, g := range groups {
		gm, _ := g.(map[string]any)
		gp := fmt.Sprintf("%s/groups/%d", ptr, gi)
		if r, ok := gm["risk"].(map[string]any); ok {
			rangeWithin(gp+"/risk", r)
		}
		rows, _ := gm["rows"].([]any)
		for ri, r := range rows {
			rm, _ := r.(map[string]any)
			rp := fmt.Sprintf("%s/rows/%d", gp, ri)
			if n, ok := rm["note"].(map[string]any); ok {
				rangeWithin(rp+"/note", n)
			}
			segs, _ := rm["segments"].([]any)
			for si, s := range segs {
				sm, _ := s.(map[string]any)
				rangeWithin(fmt.Sprintf("%s/segments/%d", rp, si), sm)
			}
		}
	}
}

// checkShapes は shapes の data の意味を見る: 座標が枠の中にあるか(shape-bounds)、
// 役割名がテーマにあるか(role-exists)、コネクタの from・to がこの図の図形を
// 指すか(target-exists・connector-self)。id の重複は walkDoc が見る(unique-id)。
func (c *checker) checkShapes(data map[string]any, ptr string, docIDs map[string]string, tm *Template) {
	items, _ := data["items"].([]any)
	local := map[string]bool{} // この図の中の、図形(矩形の場所を持つもの)の id
	for _, x := range items {
		xm, _ := x.(map[string]any)
		if k, _ := xm["kind"].(string); contains(shapeBoxedKinds, k) {
			if id, ok := xm["id"].(string); ok {
				local[id] = true
			}
		}
	}
	num := func(m map[string]any, k string) (float64, bool) {
		f, ok := m[k].(float64)
		return f, ok
	}
	const eps = 1e-9
	for i, x := range items {
		xm, _ := x.(map[string]any)
		ip := fmt.Sprintf("%s/items/%d", ptr, i)
		kind, _ := xm["kind"].(string)

		// 座標: 1つずつ 0〜1、x+w・y+h も 1 以下。
		for _, k := range []string{"x", "y", "w", "h"} {
			if f, ok := num(xm, k); ok && (f < 0 || f > 1) {
				c.add(ip+"/"+k, fmt.Sprintf("%s = %v is outside 0..1 (coordinates are relative to the frame)", k, f), "shape-bounds")
			}
		}
		for _, pr := range [][2]string{{"x", "w"}, {"y", "h"}} {
			p0, ok1 := num(xm, pr[0])
			sz, ok2 := num(xm, pr[1])
			if ok1 && ok2 && p0 >= 0 && sz <= 1 && p0+sz > 1+eps {
				c.add(ip+"/"+pr[1], fmt.Sprintf("%s+%s = %.6g goes beyond the frame (must be at most 1)", pr[0], pr[1], p0+sz), "shape-bounds")
			}
		}
		if kind == "line" || kind == "arrow" {
			for _, k := range []string{"from", "to"} {
				pt, _ := xm[k].([]any)
				for j, v := range pt {
					if f, ok := v.(float64); ok && (f < 0 || f > 1) {
						c.add(fmt.Sprintf("%s/%s/%d", ip, k, j), fmt.Sprintf("%v is outside 0..1 (points are relative to the frame)", f), "shape-bounds")
					}
				}
			}
		}

		// 役割名: このテンプレートのテーマにあるもの。
		if tm != nil {
			for _, k := range shapeRoleKeys {
				r, ok := xm[k].(string)
				if !ok {
					continue
				}
				if _, known := tm.Theme[r]; !known {
					msg := fmt.Sprintf("template %q has no role %q", tm.Name, r)
					if hexColorLikeRe.MatchString(r) || !roleNameRe.MatchString(r) {
						msg = fmt.Sprintf("%q is not a role name (colors are written as theme role names)", r)
					}
					c.addIssue(Issue{Path: ip + "/" + k, Code: "role-exists",
						Message:  msg,
						Expected: sortedKeys(tm.Theme), Hint: "色はテーマの役割名で書きます"})
				}
			}
		}

		// コネクタの指す先。
		if kind != "connector" {
			continue
		}
		from, _ := xm["from"].(string)
		for _, k := range []string{"from", "to"} {
			id, ok := xm[k].(string)
			if !ok || local[id] {
				continue
			}
			if where, elsewhere := docIDs[id]; elsewhere {
				c.add(ip+"/"+k, fmt.Sprintf("%s %q is not a shape of this canvas (it is at %s)", k, id, where), "target-exists")
			} else {
				c.add(ip+"/"+k, fmt.Sprintf("no shape has id %q", id), "target-exists")
			}
		}
		if to, _ := xm["to"].(string); to != "" && to == from && local[to] {
			c.add(ip+"/to", fmt.Sprintf("a connector must join two different shapes (both ends are %q)", to), "connector-self")
		}
	}
}

// checkMatrix は matrix の各行のセルの数が列の数と同じかを見る(matrix-cells)。
func (c *checker) checkMatrix(data map[string]any, ptr string) {
	cols, ok := data["columns"].([]any)
	if !ok {
		return
	}
	rows, _ := data["rows"].([]any)
	for i, r := range rows {
		rm, _ := r.(map[string]any)
		cells, ok := rm["cells"].([]any)
		if !ok || len(cells) == len(cols) {
			continue
		}
		c.add(fmt.Sprintf("%s/rows/%d/cells", ptr, i),
			fmt.Sprintf("row has %d cells but there are %d columns (one cell per column)", len(cells), len(cols)), "matrix-cells")
	}
	if e, ok := data["emphasis"].(string); ok {
		var names []string
		for _, col := range cols {
			names = append(names, inlinePlainAny(col))
		}
		if j, msg := matrixEmphasisColumn(names, e); j < 0 {
			c.add(ptr+"/emphasis", msg, "matrix-emphasis")
		}
	}
}

// checkChart は比べ方とデータの組み合わせを見る(chart-values・chart-emphasis)。
// 中身は chart.go の chartProblems(組み立ての読み込みと同じもの)。形が Schema に
// 合わないもの(型の違い)はここでは見ず、読み込み(schema)に任せる。
func (c *checker) checkChart(data map[string]any, ptr string) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	var in chartIR
	if json.Unmarshal(b, &in) != nil {
		return
	}
	for _, pr := range chartProblems(&in) {
		c.add(ptr+pr.at, pr.msg, pr.code)
	}
}

func dateAt(m map[string]any, k string) (time.Time, error) {
	s, ok := m[k].(string)
	if !ok {
		return time.Time{}, fmt.Errorf("missing %s", k)
	}
	return parseIRDate(s)
}

func fmtYM(t time.Time) string {
	if t.Day() == 1 {
		return t.Format("2006-01")
	}
	return t.Format("2006-01-02")
}

// --- 道具 ---

// walkAll は JSON の木を深さ優先で歩き、すべての値に JSON Pointer を添えて f を呼ぶ。
// object のキーは名前の順に歩く(出力の順を決まったものにするため)。
func walkAll(v any, ptr string, f func(ptr string, v any)) {
	f(ptr, v)
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkAll(x[k], ptr+"/"+escapePointer(k), f)
		}
	case []any:
		for i, e := range x {
			walkAll(e, ptr+"/"+strconv.Itoa(i), f)
		}
	}
}

func escapePointer(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}

func getPath(v any, key string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m[key]
}

// locationToPointer は ir.go / gantt_ir.go / template.go の誤りの文の頭にある場所
// (例 "slides[2]: body[0]: gantt: groups[0].rows[1].segments[0].status: ...")を
// JSON Pointer(/slides/2/body/0/data/groups/0/rows/1/segments/0/status)に変える。
// 場所として読めない部分に来たらそこで止める。
var locSegRe = regexp.MustCompile(`^[A-Za-z]+(\[[0-9]+\])?(\.[A-Za-z]+(\[[0-9]+\])?)*$`)
var locPartRe = regexp.MustCompile(`([A-Za-z]+)|\[([0-9]+)\]`)

func locationToPointer(msg string) string {
	ptr := ""
	for _, seg := range strings.Split(msg, ": ") {
		if seg == "gantt" || seg == "shapes" || contains(figureKinds, seg) {
			ptr += "/data"
			continue
		}
		if !locSegRe.MatchString(seg) || !strings.ContainsAny(seg, "[.") {
			break
		}
		for _, m := range locPartRe.FindAllStringSubmatch(seg, -1) {
			if m[1] != "" {
				ptr += "/" + m[1]
			} else {
				ptr += "/" + m[2]
			}
		}
	}
	// encoding/json の「知らないキー」は、そのキーまで指す。
	if m := unknownFieldRe.FindStringSubmatch(msg); m != nil {
		ptr += "/" + escapePointer(m[1])
	}
	return ptr
}

var unknownFieldRe = regexp.MustCompile(`json: unknown field "([^"]+)"`)

// warningIssue は RenderDeck の警告の文("slide 2: [FILL-RATIO WARNING] ...")を
// 1件の警告にする。rule は角括弧の中の名前から作る(FILL-RATIO → fill-ratio)。
// 新しい種類の警告が増えても、この形で書かれていればそのまま rule になる。
var warnRe = regexp.MustCompile(`^slide ([0-9]+): \[([A-Z-]+) WARNING\] (.*)$`)

var warnBlockRe = regexp.MustCompile(`^body\[([0-9]+)\]: (.*)$`)

var warnColumnRe = regexp.MustCompile(`^columns\[([0-9]+)\]: body\[([0-9]+)\]: (.*)$`)

// warnPairRe は対の並びの中の場所(pairs[K]: か pairs[K]: subject: )。
var warnPairRe = regexp.MustCompile(`^pairs\[([0-9]+)\]: (?:(subject|response): )?(.*)$`)

var warnShapeItemRe = regexp.MustCompile(`^shapes: items\[([0-9]+)\]`)

var warnGanttMilestoneRe = regexp.MustCompile(`^gantt: milestones\[([0-9]+)\]`)
var warnStatItemRe = regexp.MustCompile(`^stat: items\[([0-9]+)\]`)

var warnGanttSegRe = regexp.MustCompile(`^gantt: groups\[([0-9]+)\]\.rows\[([0-9]+)\]\.segments\[([0-9]+)\]`)

// warnFigureRe は図のサブシステム(process・orgchart・matrix・funnel)の警告の
// 頭の場所(steps[2]・units[0].members[1]・rows[1].cells[2] など)。
// 場所のあとは「:」か「 (」が続く(「chart: needs at least …」のような最小の大きさの
// 警告は場所を持たないので、ブロックを指す)。
var warnFigureRe = regexp.MustCompile(`^(?:process|orgchart|matrix|funnel|chart): ([a-z]+(?:\[[0-9]+\])?(?:\.[a-z]+(?:\[[0-9]+\])?)*)(?::| \()`)

var warnRuleNames = map[string]string{"SUBSYSTEM": "subsystem-min", "ROLE": "theme-role"}

// figureWarnPointer は警告の頭の場所(units[0].members[1])を JSON Pointer
// (/units/0/members/1)にする。
func figureWarnPointer(loc string) string {
	ptr := ""
	for _, m := range locPartRe.FindAllStringSubmatch(loc, -1) {
		if m[1] != "" {
			ptr += "/" + m[1]
		} else {
			ptr += "/" + m[2]
		}
	}
	return ptr
}

func warningIssue(file, w string) Issue {
	m := warnRe.FindStringSubmatch(w)
	if m == nil {
		return Issue{File: file, Message: w, Code: "warning"}
	}
	n, _ := strconv.Atoi(m[1])
	rule := warnRuleNames[m[2]]
	if m[2] == "SUBSYSTEM" && strings.Contains(m[3], "overlaps another annotation") {
		// Gantt の注釈の置き場所が無かった(lintSubsystems が拾う SubsystemLinter の警告)
		rule = "annotation-overlap"
	}
	if m[2] == "SUBSYSTEM" && strings.Contains(m[3], "gantt: milestones[") && strings.Contains(m[3], ") overlaps milestones[") {
		// Gantt の節目の文字が近すぎて、段をずらしても重なった(GanttSubsystem.Lint)
		rule = "milestone-overlap"
	}
	if m[2] == "SUBSYSTEM" && strings.Contains(m[3], "gantt: groups[") && strings.Contains(m[3], "is not drawn") {
		// Gantt の区間の文を帯が狭すぎて置けなかった(GanttSubsystem.Lint)
		rule = "gantt-segment-label"
	}
	shapes := m[2] == "SUBSYSTEM" && strings.Contains(m[3], "shapes: items[")
	if shapes && strings.Contains(m[3], ") overlaps items[") {
		rule = "shapes-overlap" // shapes の図形どうしが重なった
	}
	if shapes && strings.Contains(m[3], "text does not fit") {
		rule = "shapes-text-overflow" // shapes の図形の文が収まらない
	}
	if shapes && strings.Contains(m[3], "meets a side at a shallow angle") {
		rule = "shapes-connector-angle" // shapes のコネクタが辺に浅く入り、角で終わったように見える
	}
	inBlock := m[3] // 本文の中の場所(body[2]: …)を除いた文
	if bm := warnBlockRe.FindStringSubmatch(inBlock); bm != nil {
		inBlock = bm[2]
	}
	figure := m[2] == "SUBSYSTEM" && warnFigureRe.MatchString(inBlock)
	if figure && strings.Contains(m[3], "text does not fit") {
		rule = "figure-text-overflow" // 図のサブシステムの箱・セルの文が収まらない
	}
	if figure && strings.Contains(m[3], "label does not fit its place on the category axis") {
		rule = "chart-labels" // グラフの項目の名前がアプリの置く場所に入りきらない
	}
	if figure && strings.Contains(m[3], "has no legend text") {
		rule = "figure-legend" // 図の箱の状態を使ったのに凡例の文が無い
	}
	if figure && strings.Contains(m[3], "a funnel is for values that decrease") {
		rule = "funnel-increase" // funnel の数が前の段より増えた
	}
	if rule == "" {
		rule = strings.ToLower(m[2])
	}
	path, msg := fmt.Sprintf("/slides/%d", n-1), m[3]
	// 題の警告(title: ...)は題を指す。
	if rest, ok := strings.CutPrefix(msg, "title: "); ok {
		path, msg = path+"/title", rest
	}
	// 文の頭に本文の中の場所(body[2]: ...)があれば、そのブロックまで指す。
	if bm := warnBlockRe.FindStringSubmatch(msg); bm != nil {
		path, msg = path+"/body/"+bm[1], bm[2]
		// 並列のまとまりの中のブロック(columns[C]: body[K]: ...)。
		if cm := warnColumnRe.FindStringSubmatch(msg); cm != nil {
			path, msg = path+"/columns/"+cm[1]+"/body/"+cm[2], cm[3]
		}
		// 対の並びの中(pairs[K]: … か pairs[K]: subject: …)。
		if pm := warnPairRe.FindStringSubmatch(msg); pm != nil {
			path, msg = path+"/pairs/"+pm[1], pm[3]
			if pm[2] != "" {
				path += "/" + pm[2]
			}
		}
		// shapes の警告は、最初に名指しした図形まで指す。
		if im := warnShapeItemRe.FindStringSubmatch(msg); im != nil {
			path += "/data/items/" + im[1]
		}
		// stat の数と単位の警告は、その数まで指す。
		if sm := warnStatItemRe.FindStringSubmatch(msg); sm != nil {
			path += "/items/" + sm[1]
		}
		// Gantt の区間の文の警告は、その区間まで指す。
		if mm := warnGanttMilestoneRe.FindStringSubmatch(msg); mm != nil {
			path += "/data/milestones/" + mm[1]
		}
		if gm := warnGanttSegRe.FindStringSubmatch(msg); gm != nil {
			path += "/data/groups/" + gm[1] + "/rows/" + gm[2] + "/segments/" + gm[3]
		}
		// 図のサブシステムの警告は、名指しした要素まで指す。
		if fm := warnFigureRe.FindStringSubmatch(msg); fm != nil {
			path += "/data" + figureWarnPointer(fm[1])
		}
	}
	return Issue{File: file, Path: path, Message: msg, Code: rule}
}

// checkTableGroups は表の行の群(groups)の決まりを見る(table-groups): 群の列の
// ほかに列があること、名前が空でないこと、行の数の和が本体の行の数であること。
func (c *checker) checkTableGroups(groups []any, cols, rows int, bp string) {
	if cols > 0 && cols < 2 {
		c.add(bp+"/groups", "the table has only the group column (groups need at least one more column)", "table-groups")
	}
	sum := 0
	for gi, g := range groups {
		gm, _ := g.(map[string]any)
		if gm == nil {
			return
		}
		if l, ok := gm["label"]; ok && strings.TrimSpace(inlinePlainAny(l)) == "" {
			c.add(fmt.Sprintf("%s/groups/%d/label", bp, gi), "the group label is empty", "table-groups")
		}
		n, ok := gm["rows"].(float64)
		if !ok {
			return
		}
		sum += int(n)
	}
	if sum != rows {
		c.add(bp+"/groups", fmt.Sprintf("the groups cover %d rows but the table has %d body rows (the rows of the groups add up to the body rows)", sum, rows), "table-groups")
	}
}
