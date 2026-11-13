// -fit の分け(#149、親 #148 の fit-split)。
//
// -fit(BuildOptions.Fit・CheckOptions.Fit)で梯子(fit_shrink.go)の最後の段でも収まらない
// 本文のスライド(#145 の判定:はみ出すか図が崩れる)を、分けられる境で続きのスライドに
// 分ける。IR は変えない(組み立ての中の写しの deck でスライドが増える)。
//
//   - 分けられる境:本文のブロックの間と、表は本体の行(行の群の途中では切らない。合計の
//     行は最後の枚)、箇条(points)は一番外の項目(入れ子は親ごと)、pairs は対、topics は
//     話題。columns・図(サブシステム)・画像ほかは分けない。分けた表は続きの表
//     (continues。tablechain.go)と同じく、各枚に見出しの行を繰り返し、列の幅をそろえる
//   - 分け方:前から詰め、各枚が入る最小の枚数。枚数は梯子の最後の段で決め、その枚数で
//     入る最も上の段(元の密度 → dense → fit.shrink[0] → …)で全部の枚を組む(字の大きさが
//     枚の間でそろう)。最後の段でも1つの単位(1行・1項目・分けないブロック)が1枚に
//     入らなければ分けない(元のまま。layout-overflow が残る)
//   - 続きの枚:題とリード・出典・隅の印(marker)は各枚に。2枚目からの題は値の表の
//     marks.continuedTitle、各枚の題の隣に marks.tableContinued の「n/m」。ノートは最初の
//     枚だけ。番号つきの箇条は前の枚の続きの番号から。目次・頁番号は分けた後の枚で数える
//   - 続きの表(continues)のひと続きにあるスライドは分けない(梯子だけ。fitGroupOf)
//   - 分けたスライドには info の [FIT-SPLIT INFO](code fit-split。枚数・段・各枚の始まり)を
//     最初の枚に出し、容量は枚ごとに split(何枚目・枚数・始まり)を付けて出す。警告と容量の
//     場所は IR のスライドとブロックの番号(切れ端の中の行・項目・対・話題の番号も IR の番号)

package boxloom

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// codeFitSplit は -fit で本文のスライドを続きのスライドに分けた知らせの code。
const codeFitSplit = "fit-split"

// slideSplit は分けた枚の印(IRSlide.split)。
type slideSplit struct {
	from int // 元の IR のスライドの番号
	n, m int // 何枚目(1 から)・枚数
	// blocks は枚の本文のブロックごとの元の場所:IR の本文の番号と、切れ端なら行・項目・
	// 対・話題の始まりの番号(key はその欄の名前。分けていないブロックは "")。
	blocks []splitBlockRef
	starts []string   // 2枚目からの各枚の始まり(IR の場所。/slides/S/body/N/rows/K)
	choice *fitChoice // 組んだ段(元の密度のままなら nil)
	step   string     // 段の名前(元の密度のままなら "normal" か "dense")
	values FitValues  // その段の本文の字・blockGap(知らせの文)
}

// minParts は pairs・topics の数の下限:分けた切れ端(piece)は1つでもよい。
func (b *IRBlock) minParts(n int) int {
	if b.piece {
		return 1
	}
	return n
}

// splitBlockRef は分けた枚の1つのブロックの元の場所。
type splitBlockRef struct {
	index int    // IR の本文の番号
	key   string // 切れ端の欄(rows・items・pairs・topics)。分けていなければ ""
	from  int    // 切れ端の始まり(key の欄の番号)
}

// splitUnit は分ける単位:本文のブロック block の、欄 key の [from, to)。分けない
// ブロックは key が ""(ブロックの全部)。
type splitUnit struct {
	block    int
	key      string
	from, to int
	n        int // 欄の数(切れ端が最後までか)
}

// splitKeyOf はブロックを分ける欄の名前と、その数・切ってよい位置(1〜数−1)。分けない
// ブロックは ""。
func splitKeyOf(b *IRBlock) (key string, n int, cuts []int, err error) {
	all := func(n int) []int {
		var out []int
		for c := 1; c < n; c++ {
			out = append(out, c)
		}
		return out
	}
	switch b.Type {
	case "table":
		n = len(b.Rows)
		ok := map[int]bool{}
		for _, c := range all(n) {
			ok[c] = true
		}
		// 行の群の途中では切らない(どの群の列でも群の境)。
		for _, gs := range append([][]IRTableGroup{b.Groups}, groupLists(b.GroupColumns)...) {
			if gs == nil {
				continue
			}
			bounds := map[int]bool{}
			at := 0
			for _, g := range gs {
				at += g.Rows
				bounds[at] = true
			}
			for c := range ok {
				if !bounds[c] {
					delete(ok, c)
				}
			}
		}
		for _, c := range all(n) {
			if ok[c] {
				cuts = append(cuts, c)
			}
		}
		return "rows", n, cuts, nil
	case "points":
		return "items", len(b.Items), all(len(b.Items)), nil
	case pairsType, topicsType:
		key = b.Type
		arr, err := rawArray(b.Raw, key)
		if err != nil {
			return "", 0, nil, err
		}
		return key, len(arr), all(len(arr)), nil
	}
	return "", 0, nil, nil
}

func groupLists(gcs []IRTableGroupColumn) [][]IRTableGroup {
	var out [][]IRTableGroup
	for _, gc := range gcs {
		out = append(out, gc.Groups)
	}
	return out
}

// rawArray は JSON のブロックの欄 key の配列。
func rawArray(raw json.RawMessage, key string) ([]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(m[key], &arr); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return arr, nil
}

// splitUnits は本文を分ける単位に並べる。
func splitUnits(s *IRSlide) ([]splitUnit, error) {
	var out []splitUnit
	for k := range s.Body {
		key, n, cuts, err := splitKeyOf(&s.Body[k])
		if err != nil {
			return nil, fmt.Errorf("body[%d]: %w", k, err)
		}
		if key == "" || len(cuts) == 0 {
			out = append(out, splitUnit{block: k})
			continue
		}
		at := 0
		for _, c := range append(cuts, n) {
			out = append(out, splitUnit{block: k, key: key, from: at, to: c, n: n})
			at = c
		}
	}
	return out, nil
}

// sliceGroups は群の並び gs のうち、本体の行 [from, to) にある群(境は切ってよい位置)。
func sliceGroups(gs []IRTableGroup, from, to int) []IRTableGroup {
	if gs == nil {
		return nil
	}
	var out []IRTableGroup
	at := 0
	for _, g := range gs {
		if at >= from && at+g.Rows <= to {
			out = append(out, g)
		}
		at += g.Rows
	}
	return out
}

// pieceBlock はブロック b の欄 key の [from, to) の切れ端(数 n のうち)。
func pieceBlock(b IRBlock, key string, from, to, n int) (IRBlock, error) {
	nb := b
	nb.piece = true
	nb.chain, nb.chainPos = nil, 0
	switch key {
	case "rows":
		nb.Rows = b.Rows[from:to]
		if to < n {
			nb.Total = nil // 合計の行は最後の枚
		}
		nb.Groups = sliceGroups(b.Groups, from, to)
		nb.GroupColumns = nil
		for _, gc := range b.GroupColumns {
			nb.GroupColumns = append(nb.GroupColumns, IRTableGroupColumn{Column: gc.Column, Groups: sliceGroups(gc.Groups, from, to)})
		}
		if from > 0 {
			// 続きの表:見出しの行と説明の文は最初の表のものを繰り返す(tablechain.go)。
			nb.Continues, nb.Header, nb.Caption = true, nil, nil
		}
	case "items":
		nb.Items = b.Items[from:to]
		nb.itemStart = b.itemStart + from
	case pairsType, topicsType:
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b.Raw, &m); err != nil {
			return nb, err
		}
		arr, err := rawArray(b.Raw, key)
		if err != nil {
			return nb, err
		}
		if m[key], err = json.Marshal(arr[from:to]); err != nil {
			return nb, err
		}
		if nb.Raw, err = json.Marshal(m); err != nil {
			return nb, err
		}
	}
	return nb, nil
}

// pieceSlide は s の本文を単位 us(並びの続き)だけにした写しと、ブロックごとの元の場所。
func pieceSlide(s *IRSlide, us []splitUnit) (IRSlide, []splitBlockRef, error) {
	ns := *s
	ns.Body = nil
	var refs []splitBlockRef
	for a := 0; a < len(us); {
		u := us[a]
		b := a + 1
		for b < len(us) && us[b].block == u.block {
			b++
		}
		blk := s.Body[u.block]
		ref := splitBlockRef{index: u.block}
		if u.key != "" {
			from, to := u.from, us[b-1].to
			if from > 0 || to < u.n {
				var err error
				if blk, err = pieceBlock(blk, u.key, from, to, u.n); err != nil {
					return ns, nil, fmt.Errorf("body[%d]: %w", u.block, err)
				}
				ref.key, ref.from = u.key, from
			}
		}
		ns.Body = append(ns.Body, blk)
		refs = append(refs, ref)
		a = b
	}
	return ns, refs, nil
}

// continuedTitle は2枚目からの題(marks.continuedTitle の {title} を元の題に置き換える)。
func (m TemplateMarks) continuedTitle(title Inline) Inline {
	pre, post, _ := strings.Cut(m.ContinuedTitle, "{title}")
	var out Inline
	if pre != "" {
		out = append(out, InlinePart{Text: pre})
	}
	out = append(out, title...)
	if post != "" {
		out = append(out, InlinePart{Text: post})
	}
	return out
}

// splitPieces は s の単位 us を区切り plan([始め, 終わり) の並び)で分けた枚(段 set を当て、
// 題・ノート・印を付けたもの)。m は印の枚数。
func (t *Template) splitPieces(s *IRSlide, from int, us []splitUnit, plan [][2]int, m int, set func(*IRSlide)) ([]IRSlide, error) {
	var out []IRSlide
	for k, r := range plan {
		ns, refs, err := pieceSlide(s, us[r[0]:r[1]])
		if err != nil {
			return nil, err
		}
		set(&ns)
		ns.split = &slideSplit{from: from, n: k + 1, m: m, blocks: refs}
		if k > 0 {
			ns.Title = t.Marks.continuedTitle(s.Title)
			ns.Notes = "" // ノートは最初の枚だけ
		}
		out = append(out, ns)
	}
	return out, nil
}

// splitFits は i 枚目の続きに、前の枚 prev と候補 cand を並べた写しで cand が収まるか
// (梯子と同じ測り measureFitStep。#145 の判定)。
func (t *Template) splitFits(d *IRDeck, i int, prev []IRSlide, cand IRSlide) (bool, error) {
	dd := *d
	dd.Slides = make([]IRSlide, 0, i+len(prev)+1)
	dd.Slides = append(dd.Slides, d.Slides[:i]...)
	dd.Slides = append(dd.Slides, prev...)
	dd.Slides = append(dd.Slides, cand)
	if err := linkTableChains(&dd); err != nil {
		return false, err
	}
	m, err := t.measureFitStep(&dd, len(dd.Slides)-1)
	return m.ok && !m.bad(), err
}

// splitPlan は i 枚目の単位 us を、段 set で前から詰めて分ける区切り(各枚は入る限り
// 多くの単位)。1つの単位でも1枚に入らない、または limit(0 なら無し)枚を超えるなら nil。
func (t *Template) splitPlan(d *IRDeck, i int, us []splitUnit, set func(*IRSlide), limit int) ([][2]int, error) {
	s := &d.Slides[i]
	var prev []IRSlide
	var plan [][2]int
	fits := func(a, b int) (bool, error) {
		cand, err := t.splitPieces(s, i, us, [][2]int{{a, b}}, len(prev)+2, set)
		if err != nil {
			return false, err
		}
		c := cand[0]
		c.split.n = len(prev) + 1
		if len(prev) > 0 {
			c.Title, c.Notes = t.Marks.continuedTitle(s.Title), ""
		}
		return t.splitFits(d, i, prev, c)
	}
	n := len(us)
	for a := 0; a < n; {
		if limit > 0 && len(plan) >= limit {
			return nil, nil
		}
		ok, err := fits(a, a+1)
		if err != nil || !ok {
			return nil, err
		}
		// 入る最も長い並び [a, lo)(入るかは単位を足すほど悪くなると見て、倍に伸ばして
		// から2分で探す。短い並びほど測るのが軽い)。
		lo, hi := a+1, n+1
		for k := 2; lo < n; k *= 2 {
			next := min(a+k, n)
			ok, err := fits(a, next)
			if err != nil {
				return nil, err
			}
			if !ok {
				hi = next
				break
			}
			lo = next
		}
		for hi-lo > 1 && lo < n {
			mid := (lo + hi) / 2
			ok, err := fits(a, mid)
			if err != nil {
				return nil, err
			}
			if ok {
				lo = mid
			} else {
				hi = mid
			}
		}
		pieces, err := t.splitPieces(s, i, us, [][2]int{{a, lo}}, len(prev)+2, set)
		if err != nil {
			return nil, err
		}
		pieces[0].split.n = len(prev) + 1
		if len(prev) > 0 {
			pieces[0].Title, pieces[0].Notes = t.Marks.continuedTitle(s.Title), ""
		}
		prev = append(prev, pieces[0])
		plan = append(plan, [2]int{a, lo})
		a = lo
	}
	if limit > 0 && len(plan) > limit {
		return nil, nil
	}
	return plan, nil
}

// splitSteps は i 枚目を組める段:元の密度 → (normal なら)dense → fit.shrink[0] → …。
func (t *Template) splitSteps(s *IRSlide) []fitStepTry {
	base := fitStepTry{"normal", func(*IRSlide) {}, t.Values}
	if s.dense() {
		base = fitStepTry{fitStepDense, func(*IRSlide) {}, t.densed().Values}
	}
	steps := []fitStepTry{base}
	if !s.dense() && t.Dense != (DenseValues{}) {
		steps = append(steps, fitStepTry{fitStepDense, func(s *IRSlide) { s.Density = densityDense }, t.densed().Values})
	}
	for k := range t.Fit.Shrink {
		r := fitRef{n: k + 1}
		steps = append(steps, fitStepTry{r.name(), func(s *IRSlide) { s.fit = r }, r.values(t).apply(t.Values)})
	}
	return steps
}

// splitSlide は i 枚目が梯子のどの段でも収まらないとき、分けた枚を返す。分けない・
// 分けられないなら nil。
func (t *Template) splitSlide(d *IRDeck, i int) ([]IRSlide, error) {
	s := &d.Slides[i]
	if s.Type != "content" || len(s.Body) == 0 || len(fitGroupOf(d, i)) > 1 {
		return nil, nil
	}
	steps := t.splitSteps(s)
	var broken, over bool
	// 最後の段から測る(ほとんどのスライドはそこで収まり、ほかの段を測らずに済む)。
	order := append([]fitStepTry{steps[len(steps)-1]}, steps[:len(steps)-1]...)
	whole := map[string]fitMeasure{} // 段ごとの、分けずに組んだときの測り値
	for k, st := range order {
		ms, err := t.measureFitGroup(d, []int{i}, st.set)
		if err != nil {
			return nil, err
		}
		if ms == nil {
			return nil, nil // 本文を測れない
		}
		if !ms[0].bad() {
			return nil, nil // 元のまま・梯子で収まる(fit_shrink.go)
		}
		whole[st.name] = ms[0]
		if k == 1 || len(order) == 1 {
			over, broken = ms[0].over, ms[0].broken // 元の密度で組んだときの崩れ方
		}
	}
	us, err := splitUnits(s)
	if err != nil || len(us) < 2 {
		return nil, err
	}
	last := steps[len(steps)-1]
	want, err := t.splitPlan(d, i, us, last.set, 0)
	if err != nil || len(want) < 2 {
		return nil, err
	}
	for k, st := range steps {
		plan := want
		if w := whole[st.name]; k < len(steps)-1 && w.need-float64(len(want)-1)*st.v.BlockGap > float64(len(want))*w.area+overflowEps {
			// 分けずに積んだ高さが枚数ぶんの本文の領域を越える段は、その枚数では入らない
			// (分けて減るのは切れ目のブロックの間だけ。見出しの行などは枚ごとに増える)。
			continue
		}
		if k < len(steps)-1 {
			if plan, err = t.splitPlan(d, i, us, st.set, len(want)); err != nil {
				return nil, err
			}
			if len(plan) != len(want) {
				continue
			}
		}
		pieces, err := t.splitPieces(s, i, us, plan, len(plan), st.set)
		if err != nil {
			return nil, err
		}
		var starts []string
		for _, r := range plan[1:] {
			u := us[r[0]]
			at := fmt.Sprintf("/slides/%d/body/%d", i, u.block)
			if u.key != "" && u.from > 0 {
				at += fmt.Sprintf("/%s/%d", u.key, u.from)
			}
			starts = append(starts, at)
		}
		var choice *fitChoice
		if k > 0 {
			choice = &fitChoice{step: st.name, body: st.v.Body, blockGap: st.v.BlockGap, group: []int{i}, over: over, broken: broken}
		}
		for p := range pieces {
			sp := pieces[p].split
			sp.starts, sp.choice, sp.step = starts, choice, st.name
			sp.values = FitValues{Body: st.v.Body, BlockGap: st.v.BlockGap}
		}
		return pieces, nil
	}
	return nil, nil
}

// fitSplitDeck は -fit の分けを当てた deck の写し(分けるスライドが無ければ d そのもの)と、
// 写しのスライドごとの元の IR の番号(分けなければ nil)。d はひと続きをつないだもの
// (BuildDeck の後)。
func (t *Template) fitSplitDeck(d *IRDeck) (*IRDeck, []int, error) {
	var out []IRSlide
	var origin []int
	changed := false
	for i := range d.Slides {
		pieces, err := t.splitSlide(d, i)
		if err != nil {
			return nil, nil, fmt.Errorf("slides[%d]: %w", i, err)
		}
		if pieces == nil {
			out = append(out, d.Slides[i])
			origin = append(origin, i)
			continue
		}
		out = append(out, pieces...)
		for range pieces {
			origin = append(origin, i)
		}
		changed = true
	}
	if !changed {
		return d, nil, nil
	}
	dd := *d
	dd.Slides = out
	return &dd, origin, nil
}

// fitSplitWarning は分けたスライドの知らせの文(最初の枚。warningIssue が fit-split にする)。
func fitSplitWarning(sp *slideSplit) string {
	var where []string
	for k, at := range sp.starts {
		where = append(where, fmt.Sprintf("slide %d of %d starts at %s", k+2, sp.m, at))
	}
	what := "the body overflows"
	if c := sp.choice; c != nil && c.broken {
		what = "a figure breaks"
		if c.over {
			what = "the body overflows and a figure breaks"
		}
	}
	return fmt.Sprintf("[FIT-SPLIT INFO] %s even at the template's last -fit step, so -fit split it into %d slides, all built at the %s step (body %gpt, blockGap %gpt): %s. The continued slides repeat the title and lead with an n/m mark; to choose the breaks and give each part its own title, split the slide yourself",
		what, sp.m, sp.step, sp.values.Body, sp.values.BlockGap, strings.Join(where, ", "))
}

// splitWarnRe は警告の文の本文の中の場所(body[N]: の後に rows[K]・items[K]・pairs[K]・
// topics[K] が続くこともある)。
var splitWarnRe = regexp.MustCompile(`(^|\] )body\[([0-9]+)\]((?:: (?:rows|items|pairs|topics)\[[0-9]+\])?)`)

var splitInnerRe = regexp.MustCompile(`^: (rows|items|pairs|topics)\[([0-9]+)\]$`)

// remapWarning は分けた枚の警告の文の本文の場所を IR の番号にする。
func (sp *slideSplit) remapWarning(w string) string {
	m := splitWarnRe.FindStringSubmatchIndex(w)
	if m == nil {
		return w
	}
	n, _ := strconv.Atoi(w[m[4]:m[5]])
	if n >= len(sp.blocks) {
		return w
	}
	ref := sp.blocks[n]
	inner := w[m[6]:m[7]]
	if im := splitInnerRe.FindStringSubmatch(inner); im != nil && im[1] == ref.key {
		k, _ := strconv.Atoi(im[2])
		inner = fmt.Sprintf(": %s[%d]", im[1], k+ref.from)
	}
	return w[:m[0]] + w[m[2]:m[3]] + fmt.Sprintf("body[%d]", ref.index) + inner + w[m[1]:]
}

// splitPathRe は容量の場所の本文の中の場所(/slides/S/body/N と、続く /rows/K など)。
var splitPathRe = regexp.MustCompile(`^/slides/[0-9]+/body/([0-9]+)(/(rows|items|pairs|topics)/([0-9]+))?`)

// remapPath は分けた枚の容量の場所(/slides/S/body/N/…)を IR の番号にする(S は呼ぶ側が
// 元の番号にしてある)。
func (sp *slideSplit) remapPath(p string) string {
	m := splitPathRe.FindStringSubmatch(p)
	if m == nil {
		return p
	}
	n, _ := strconv.Atoi(m[1])
	if n >= len(sp.blocks) {
		return p
	}
	ref := sp.blocks[n]
	out := fmt.Sprintf("/slides/%d/body/%d", sp.from, ref.index)
	if m[2] != "" {
		k, _ := strconv.Atoi(m[4])
		if m[3] == ref.key {
			k += ref.from
		}
		out += fmt.Sprintf("/%s/%d", m[3], k)
	}
	return out + p[len(m[0]):]
}

// SplitCapacity は -fit で分けたスライドの1枚の容量の印(SlideCapacity.Split)。
type SplitCapacity struct {
	Part   int      `json:"part"`   // 何枚目(1 から)
	Slides int      `json:"slides"` // 分けた枚数
	Starts []string `json:"starts"` // 2枚目からの各枚の始まり(IR の場所)
}

// remapCapacity は分けた枚の容量の場所を IR の番号にし、印を付ける。
func (sp *slideSplit) remapCapacity(sc *SlideCapacity) {
	sc.Path = fmt.Sprintf("/slides/%d", sp.from)
	sc.Split = &SplitCapacity{Part: sp.n, Slides: sp.m, Starts: append([]string{}, sp.starts...)}
	for j := range sc.Blocks {
		bc := &sc.Blocks[j]
		bc.Path = sp.remapPath(bc.Path)
		for k := range bc.Pairs {
			bc.Pairs[k].Path = sp.remapPath(bc.Pairs[k].Path)
		}
		for k := range bc.Topics {
			bc.Topics[k].Path = sp.remapPath(bc.Topics[k].Path)
		}
		for k := range bc.Cards {
			bc.Cards[k].Path = sp.remapPath(bc.Cards[k].Path)
		}
		for k := range bc.Columns {
			cc := &bc.Columns[k]
			cc.Path = sp.remapPath(cc.Path)
			for m := range cc.Blocks {
				cc.Blocks[m].Path = sp.remapPath(cc.Blocks[m].Path)
			}
		}
	}
}

// irIndex は組み立てたスライドの番号 i の元の IR の番号(分けていなければ i のまま)。
func irIndex(origin []int, i int) int {
	if origin == nil || i < 0 || i >= len(origin) {
		return i
	}
	return origin[i]
}
