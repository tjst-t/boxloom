package boxloom

// スライドのリード文(:::lead)。
//
// 本文のスライドの一番外側に :::lead … ::: を1つ置くと、その中の段落がスライドの
// lead(IR では文中の値)になる。題の下に置く、このスライドで言いたいことの文で、
// 本文の流れの外の枠。テンプレートが題の直下に置く。本文のスライドだけに書ける。
// 中に書けるのは段落1つだけ(改行は行末の \)。見出しの文と属性は付けられない。

// splitLead はスライドの一番外側の項目から :::lead を取り出す。
func splitLead(items []*mdItem) (body []*mdItem, leads []*mdFenceBlk) {
	for _, it := range items {
		if it.kind == mdFence && it.fence.name == "lead" {
			leads = append(leads, it.fence)
			continue
		}
		body = append(body, it)
	}
	return body, leads
}

// leadInline は :::lead の並び(ふつうは1つ)を IR の lead にする。無ければ ok=false。
func (p *mdParser) leadInline(fs []*mdFenceBlk, path string) (any, bool) {
	if len(fs) == 0 {
		return nil, false
	}
	p.mark(path, fs[0].line)
	for _, f := range fs[1:] {
		p.fail(f.line, 1, path, codeMisplaced, "リード文(:::lead)は1枚に1つだけ書けます", nil, "1つの :::lead にまとめてください")
	}
	f := fs[0]
	p.fenceAttrs(f, path, nil)
	p.noLabel(f, path)
	para := p.onePara(f, path)
	if para == nil {
		return nil, false
	}
	return p.parseInline(para.text.trimBR(), path), true
}
