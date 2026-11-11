package boxloom

// スライドの出典(:::source)。
//
// スライドの本文の一番外側に :::source … ::: を1つ置くと、その中の段落がスライドの
// source(IR では文中の値)になる。本文の流れの外の枠で、テンプレートが本文の下・
// フッターの上に置く。本文と主張のスライドに書ける(区切りと表紙は持たない)。
// 中に書けるのは段落1つだけ(改行は行末の \)。見出しの文と属性は付けられない。

// splitSource はスライドの一番外側の項目から :::source を取り出す。
func splitSource(items []*mdItem) (body []*mdItem, sources []*mdFenceBlk) {
	for _, it := range items {
		if it.kind == mdFence && it.fence.name == "source" {
			sources = append(sources, it.fence)
			continue
		}
		body = append(body, it)
	}
	return body, sources
}

// sourceInline は :::source の並び(ふつうは1つ)を IR の source にする。無ければ ok=false。
func (p *mdParser) sourceInline(fs []*mdFenceBlk, path string) (any, bool) {
	if len(fs) == 0 {
		return nil, false
	}
	p.mark(path, fs[0].line)
	for _, f := range fs[1:] {
		p.fail(f.line, 1, path, codeMisplaced, "出典(:::source)は1枚に1つだけ書けます", nil, "1つの :::source にまとめてください(改行は行末の \\)")
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
