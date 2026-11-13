// -fit の列ごとの分け(#152、親 #148 の fit-split)。
//
// -fit の順(梯子 → 分ける → 非常用の段。fit_shrink.go・fit_split.go・fit_emergency.go)の
// 最後、誤りにする前に:非常用の段でも入らない並列のまとまり(columns、2〜3列)を持つ本文の
// スライドは、列ごとに続きのスライドに分ける。
//
//   - 列の並び順のまま(role=main の列を先にはしない)、1列を1枚に。各列は全幅で組み
//     (1列だけの columns。列の題があればその枚に)、flow の印・role は持たない
//   - columns の前のブロックは最初の列の枚に、後のブロックは最後の列の枚に、columns の間の
//     ブロックは前の列の枚に付く
//   - 分けた各枚は今の fit の順で組み直す:元の密度 → 梯子 → 非常用の段のうち、分けずに入る
//     最初の段で組み、どれでも入らなければその枚を分ける(fit_split.go の splitSlide。分けた
//     枚はその段でそろう)。1枚でも入らなければ列ごとには分けない(元のまま。-fit の build は
//     誤りで、hint に「あと N pt」と直し方。容量の fitShort)
//   - 段は枚ごとに違ってよい。知らせの fit-split は最初の枚に「列ごとに分けた」と各枚の段を、
//     非常用の段で組んだ枚には fit-emergency を出す。警告と容量の場所は IR の番号
//     (/slides/S/body/N/columns/K)に戻す

package boxloom

import (
	"fmt"
	"strings"
)

// hasMultiColumns は s の本文に2列以上の columns があるか。
func hasMultiColumns(s *IRSlide) bool {
	for k := range s.Body {
		if s.Body[k].Type == "columns" && len(s.Body[k].Columns) > 1 {
			return true
		}
	}
	return false
}

// columnGroups は s の本文を列ごとの枚に分けたもの:枚ごとの本文と、そのブロックの元の場所
// (列なら key "columns"、from は列の番号)。
func columnGroups(s *IRSlide) ([][]IRBlock, [][]splitBlockRef) {
	var bodies [][]IRBlock
	var refs [][]splitBlockRef
	var cur []IRBlock
	var curRefs []splitBlockRef
	seenColumn := false
	for k, b := range s.Body {
		if b.Type != "columns" || len(b.Columns) < 2 {
			cur = append(cur, b)
			curRefs = append(curRefs, splitBlockRef{index: k})
			continue
		}
		for ci, c := range b.Columns {
			if seenColumn {
				bodies, refs = append(bodies, cur), append(refs, curRefs)
				cur, curRefs = nil, nil
			}
			seenColumn = true
			nb := b
			nb.piece = true
			nb.Flow = false
			nb.Columns = []IRColumn{{Title: c.Title, Body: c.Body}}
			cur = append(cur, nb)
			curRefs = append(curRefs, splitBlockRef{index: k, key: "columns", from: ci})
		}
	}
	bodies, refs = append(bodies, cur), append(refs, curRefs)
	return bodies, refs
}

// splitByColumns は i 枚目を列ごとに分けた枚(各枚は fit の順で組んだもの)。どれかの枚が
// 入らない・columns が無いなら nil。over・broken は元の組み方での崩れ方。
func (t *Template) splitByColumns(d *IRDeck, i int, over, broken bool) ([]IRSlide, error) {
	s := &d.Slides[i]
	if !hasMultiColumns(s) {
		return nil, nil
	}
	bodies, refs := columnGroups(s)
	steps := append(t.splitSteps(s), t.emergencySteps()...)
	var out []IRSlide
	var leads []bool // 枚が段の知らせ(fit-emergency)を出すか
	for g := range bodies {
		p := *s
		p.Body = bodies[g]
		p.split = &slideSplit{from: i, n: len(out) + 1, blocks: refs[g], byColumn: true}
		if len(out) > 0 {
			p.Title, p.Notes = t.Marks.continuedTitle(s.Title), ""
		}
		// 前の枚を並べた写し(目次・頁番号が分けた後の枚で数える)。
		dd := *d
		dd.Slides = append(append(append([]IRSlide{}, d.Slides[:i]...), out...), p)
		at := len(dd.Slides) - 1
		if err := linkTableChains(&dd); err != nil {
			return nil, err
		}
		var placed bool
		for k, st := range steps {
			ms, err := t.measureFitGroup(&dd, []int{at}, st.set)
			if err != nil {
				return nil, err
			}
			if ms == nil || ms[0].bad() {
				continue
			}
			q := p
			st.set(&q)
			sp := *p.split
			sp.step, sp.values = st.name, FitValues{Body: st.v.Body, BlockGap: st.v.BlockGap}
			if k > 0 || st.emergency {
				sp.choice = &fitChoice{step: st.name, body: st.v.Body, blockGap: st.v.BlockGap, group: []int{i}, over: over, broken: broken, emergency: st.emergency, byColumn: true}
			}
			q.split = &sp
			out, leads = append(out, q), append(leads, true)
			placed = true
			break
		}
		if placed {
			continue
		}
		// 分けずにはどの段でも入らない:その枚を分ける(fit_split.go)。
		p.split = nil
		p.Title, p.Notes = s.Title, s.Notes
		if len(out) > 0 {
			p.Notes = ""
		}
		dd.Slides[at] = p
		pieces, err := t.splitSlide(&dd, at)
		if err != nil {
			return nil, err
		}
		if pieces == nil {
			return nil, nil // この列は分けても入らない
		}
		for n, q := range pieces {
			sp := *q.split
			sp.from, sp.byColumn, sp.parts = i, true, len(pieces)
			var mapped []splitBlockRef
			for _, r := range sp.blocks {
				pr := refs[g][r.index]
				if pr.key == "" {
					pr.key, pr.from = r.key, r.from
				}
				mapped = append(mapped, pr)
			}
			sp.blocks = mapped
			if sp.choice != nil {
				c := *sp.choice
				c.group, c.byColumn = []int{i}, true
				c.over, c.broken = over, broken
				sp.choice = &c
			}
			if len(out) > 0 {
				q.Title, q.Notes = t.Marks.continuedTitle(s.Title), ""
			}
			q.split = &sp
			out, leads = append(out, q), append(leads, n == 0)
		}
	}
	// 枚数・何枚目・各枚の始まりは全部の枚が決まってから付ける。
	var starts []string
	for _, q := range out[1:] {
		r := q.split.blocks[0]
		at := fmt.Sprintf("/slides/%d/body/%d", i, r.index)
		if r.key != "" && (r.from > 0 || r.key == "columns") {
			at += fmt.Sprintf("/%s/%d", r.key, r.from)
		}
		starts = append(starts, at)
	}
	var steps2 []string
	for _, q := range out {
		steps2 = append(steps2, q.split.step)
	}
	for k := range out {
		sp := *out[k].split
		sp.n, sp.m, sp.starts, sp.stepLead = k+1, len(out), starts, leads[k]
		sp.columnSteps = steps2
		out[k].split = &sp
	}
	return out, nil
}

// fitSplitColumnsWarning は列ごとに分けたスライドの知らせの文(最初の枚)。
func fitSplitColumnsWarning(sp *slideSplit) string {
	var where []string
	for k, at := range sp.starts {
		where = append(where, fmt.Sprintf("slide %d of %d starts at %s", k+2, sp.m, at))
	}
	var steps []string
	for k, st := range sp.columnSteps {
		steps = append(steps, fmt.Sprintf("slide %d at %s", k+1, st))
	}
	return fmt.Sprintf("[FIT-SPLIT INFO] the columns do not fit even at the template's emergency -fit step, so -fit split them by column into %d slides (each column full width, in their order): %s; steps: %s. The continued slides repeat the title and lead with an n/m mark; to give each column its own title and lead, put each column on its own slide yourself",
		sp.m, strings.Join(where, ", "), strings.Join(steps, ", "))
}

// lastFitStep は -fit が試す最後の段(非常用の段、無ければ梯子の最後の段)。
func (t *Template) lastFitStep(s *IRSlide) fitStepTry {
	if em := t.emergencySteps(); len(em) > 0 {
		return em[len(em)-1]
	}
	steps := t.splitSteps(s)
	return steps[len(steps)-1]
}

// fitShortfall は -fit でも収まらなかった i 枚目を最後の段で組んだときに足りない高さ(pt)。
// 測れなければ ok が false。
func (t *Template) fitShortfall(d *IRDeck, i int) (float64, bool, error) {
	ms, err := t.measureFitGroup(d, []int{i}, t.lastFitStep(&d.Slides[i]).set)
	if err != nil || ms == nil {
		return 0, false, err
	}
	return round1(ms[0].need - ms[0].area), true, nil
}

// fitShortHint は -fit でも収まらなかった本文の直し方(容量の fitShort から)。
func fitShortHint(c SlideCapacity) string {
	if c.FitShort == nil {
		return ""
	}
	how := "内容でスライドを分ける(それぞれに題を付けて)か、図を別のスライドに移すか、文を削ってください"
	for _, b := range c.Blocks {
		if b.Type == "columns" {
			how = "columns の列を別のスライドに分ける(列ごとに題とリードを付けて)か、図を別のスライドに移すか、文を削ってください"
		}
	}
	if *c.FitShort <= overflowEps {
		return "-fit の最後の段(非常用の段まで)でも図が崩れます。" + how
	}
	return fmt.Sprintf("-fit の最後の段(非常用の段まで)・分け・列ごとの分けでも、あと %.1fpt 足りません。%s", *c.FitShort, how)
}

// anyFitBroken は警告の文のどれかが図の崩れ(fitBrokenCodes)か。
func anyFitBroken(warnings []string) bool {
	for _, w := range warnings {
		if fitBrokenCode(w) != "" {
			return true
		}
	}
	return false
}
