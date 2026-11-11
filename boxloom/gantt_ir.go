// IR の Gantt の data(schema/subsystems/gantt.schema.json)から GanttChart を
// 組み立てる。
//
// IR は意味だけを持つので、見た目に関わることはここ(サブシステムの側)で決める。
//   - 軸の圧縮: 目盛りの日付で区切った区間ごとに、長さの平方根に比べた幅を
//     割り当てる(長い区間ほど縮む)。
//   - 目盛りの文字: 日付から "29/12" の形を作る。範囲の両端に目盛りが無ければ足す。
//   - 注釈の位置: 上下どちらのレーンに置くか・横のずらしは、描くときに
//     planAnnotations が重なりを避けて決める(Direction を空にしておく)。

package boxloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type ganttIR struct {
	Span            irRange           `json:"span"`
	AxisBreakBefore string            `json:"axisBreakBefore"`
	Ticks           []ganttTickIR     `json:"ticks"`
	Milestones      []ganttMilestone  `json:"milestones"`
	Legend          orderedLegend     `json:"legend"`
	Groups          []ganttGroupIR    `json:"groups"`
	Annotations     []ganttAnnotation `json:"annotations"`
}

type irRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ganttTickIR struct {
	At     string `json:"at"`
	Label  Inline `json:"label"`
	Status string `json:"status"`
}

type ganttMilestone struct {
	At    string `json:"at"`
	Label Inline `json:"label"`
}

type ganttGroupIR struct {
	ID    string       `json:"id"`
	Label Lines        `json:"label"`
	Risk  *irRange     `json:"risk"`
	Rows  []ganttRowIR `json:"rows"`
}

type ganttRowIR struct {
	ID       string           `json:"id"`
	Label    Inline           `json:"label"`
	Sub      string           `json:"sub"`
	Segments []ganttSegmentIR `json:"segments"`
	Note     *struct {
		From string `json:"from"`
		To   string `json:"to"`
		Text Inline `json:"text"`
	} `json:"note"`
}

type ganttSegmentIR struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Status    string `json:"status"`
	Label     Inline `json:"label"`
	OpenEnd   bool   `json:"openEnd"`
	Tentative bool   `json:"tentative"`
}

type ganttAnnotation struct {
	At       string `json:"at"`
	Target   string `json:"target"`
	Text     Inline `json:"text"`
	Status   string `json:"status"`
	Emphasis bool   `json:"emphasis"`
}

// orderedLegend は凡例。JSON の object の書いた順を保つ(Go の map は順を失う)。
type orderedLegend []LegendItem

func (l *orderedLegend) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("legend must be an object")
	}
	var out orderedLegend
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		key := kt.(string)
		if !validLegendKey[key] {
			return fmt.Errorf("legend: unknown key %q (known: support, conditional, none, danger, risk)", key)
		}
		var v Inline
		if err := dec.Decode(&v); err != nil {
			return fmt.Errorf("legend.%s: %w", key, err)
		}
		out = append(out, LegendItem{Role: key, Label: v.Plain()})
	}
	*l = out
	return nil
}

var validStatus = map[string]bool{"support": true, "conditional": true, "none": true, "danger": true}
var validLegendKey = map[string]bool{"support": true, "conditional": true, "none": true, "danger": true, "risk": true}

// ganttFromIR は IR の data を読んで GanttChart を作る。
func ganttFromIR(raw json.RawMessage) (*GanttChart, error) {
	var in ganttIR
	if err := strictUnmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("gantt: %w", err)
	}
	if in.Span.From == "" || in.Span.To == "" {
		return nil, fmt.Errorf("gantt: span needs from and to")
	}
	if len(in.Groups) == 0 {
		return nil, fmt.Errorf("gantt: groups needs at least one group")
	}

	var firstErr error
	date := func(at, s string) time.Time {
		t, err := parseIRDate(s)
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("gantt: %s: %w", at, err)
		}
		return t
	}
	status := func(at, s string) string {
		if !validStatus[s] && firstErr == nil {
			firstErr = fmt.Errorf("gantt: %s: unknown status %q (known: support, conditional, none, danger)", at, s)
		}
		return s
	}

	from, to := date("span.from", in.Span.From), date("span.to", in.Span.To)
	if firstErr == nil && !from.Before(to) {
		return nil, fmt.Errorf("gantt: span.from must be before span.to")
	}

	chart := &GanttChart{Legend: []LegendItem(in.Legend)}

	// 目盛り。範囲の両端に無ければ、日付だけの目盛りを足す。
	tickAt := map[time.Time]bool{}
	for i, t := range in.Ticks {
		at := date(fmt.Sprintf("ticks[%d].at", i), t.At)
		role := "text_primary"
		if t.Status != "" {
			role = status(fmt.Sprintf("ticks[%d].status", i), t.Status)
		}
		chart.Ticks = append(chart.Ticks, Tick{Date: at, AutoDate: true, SubLabel: t.Label.Plain(), Role: role})
		tickAt[at] = true
	}
	for _, end := range []time.Time{from, to} {
		if !tickAt[end] {
			chart.Ticks = append(chart.Ticks, Tick{Date: end, AutoDate: true, Role: "text_muted"})
		}
	}
	sort.SliceStable(chart.Ticks, func(i, j int) bool { return chart.Ticks[i].Date.Before(chart.Ticks[j].Date) })

	for i, m := range in.Milestones {
		at := date(fmt.Sprintf("milestones[%d].at", i), m.At)
		chart.Milestones = append(chart.Milestones, Milestone{Date: at, Label: m.Label.Plain(), AutoDate: true})
	}

	// 軸を切る印。
	var cuts []time.Time
	if in.AxisBreakBefore != "" {
		chart.HasBreak = true
		chart.BreakBefore = date("axisBreakBefore", in.AxisBreakBefore)
		cuts = append(cuts, chart.BreakBefore)
	}
	for _, t := range chart.Ticks {
		cuts = append(cuts, t.Date)
	}
	chart.Scale = sqrtScale(from, to, cuts)

	// グループと行。id → 位置 の表は注釈の target を引くのに使う。
	type ref struct{ group, row int }
	ids := map[string]ref{}
	addID := func(id string, r ref) {
		if id == "" {
			return
		}
		if _, dup := ids[id]; dup && firstErr == nil {
			firstErr = fmt.Errorf("gantt: id %q is used twice", id)
		}
		ids[id] = r
	}
	for gi, g := range in.Groups {
		gp := fmt.Sprintf("groups[%d]", gi)
		if len(g.Rows) == 0 {
			return nil, fmt.Errorf("gantt: %s.rows needs at least one row", gp)
		}
		label := ""
		for i, l := range g.Label {
			if i > 0 {
				label += "\n"
			}
			label += l
		}
		group := GanttGroup{Label: label}
		if g.Risk != nil {
			group.HasRiskBand = true
			group.RiskStart = date(gp+".risk.from", g.Risk.From)
			group.RiskEnd = date(gp+".risk.to", g.Risk.To)
		}
		addID(g.ID, ref{gi, -1})
		for ri, r := range g.Rows {
			rp := fmt.Sprintf("%s.rows[%d]", gp, ri)
			row := GanttRow{Label: r.Label.Plain(), SubGroup: r.Sub}
			if r.Note != nil {
				row.HasNote = true
				row.NoteStart = date(rp+".note.from", r.Note.From)
				row.NoteEnd = date(rp+".note.to", r.Note.To)
				row.NoteText = r.Note.Text.Plain()
			} else if r.Segments == nil {
				return nil, fmt.Errorf("gantt: %s needs segments or note", rp)
			}
			for si, s := range r.Segments {
				sp := fmt.Sprintf("%s.segments[%d]", rp, si)
				row.Segments = append(row.Segments, GanttSegment{
					Start: date(sp+".from", s.From), End: date(sp+".to", s.To),
					Role: status(sp+".status", s.Status), Label: s.Label.Plain(),
					OpenEnd: s.OpenEnd, Tentative: s.Tentative,
				})
			}
			addID(r.ID, ref{gi, ri})
			group.Rows = append(group.Rows, row)
		}
		chart.Groups = append(chart.Groups, group)
	}

	// 注釈。位置は持たない(Direction が空 = サブシステムが決める)。
	for i, a := range in.Annotations {
		ap := fmt.Sprintf("annotations[%d]", i)
		ann := Annotation{
			AnchorDate:  date(ap+".at", a.At),
			Text:        a.Text.Plain(),
			Role:        "text_muted",
			Emphasis:    a.Emphasis,
			TargetGroup: -1, TargetRow: -1,
		}
		if a.Status != "" {
			ann.Role = status(ap+".status", a.Status)
		}
		if a.Target != "" {
			r, ok := ids[a.Target]
			if !ok {
				return nil, fmt.Errorf("gantt: %s.target: no group or row has id %q", ap, a.Target)
			}
			ann.TargetGroup, ann.TargetRow = r.group, r.row
		}
		chart.Annotations = append(chart.Annotations, ann)
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return chart, nil
}

// formatGanttDate は日付を値の表の dateFormat で書く(YYYY・YY・MM・M を置き換える)。
func formatGanttDate(t time.Time, format string) string {
	if format == "" {
		format = "YYYY/M"
	}
	r := strings.NewReplacer("YYYY", fmt.Sprintf("%04d", t.Year()), "YY", fmt.Sprintf("%02d", t.Year()%100),
		"MM", fmt.Sprintf("%02d", int(t.Month())), "M", fmt.Sprint(int(t.Month())))
	return r.Replace(format)
}

// checkDateFormat は dateFormat が月(M か MM)を含み、年・月を YYYY・YY・MM・M
// だけで書いているかを見る(YYY のような書き方は誤り)。
func checkDateFormat(at, f string) error {
	rest := strings.NewReplacer("YYYY", "", "YY", "", "MM", "\x00", "M", "\x00").Replace(f)
	if !strings.Contains(rest, "\x00") {
		return fmt.Errorf("%s.dateFormat: must contain the month (M or MM), got %q", at, f)
	}
	if strings.ContainsAny(rest, "Y") {
		return fmt.Errorf("%s.dateFormat: write the year as YYYY or YY, got %q", at, f)
	}
	return nil
}

// sqrtScale は from〜to を cuts の日付で区切り、区間ごとに「日数の平方根」に
// 比べた幅を割り当てる。長い区間は縮み、短い区間(目盛りが詰まったところ)は
// 広がるので、目盛りの文字が重なりにくい。
func sqrtScale(from, to time.Time, cuts []time.Time) TimeScale {
	pts := []time.Time{from, to}
	for _, c := range cuts {
		if c.After(from) && c.Before(to) {
			pts = append(pts, c)
		}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].Before(pts[j]) })
	uniq := pts[:1]
	for _, p := range pts[1:] {
		if !p.Equal(uniq[len(uniq)-1]) {
			uniq = append(uniq, p)
		}
	}
	weights := make([]float64, len(uniq)-1)
	total := 0.0
	for i := range weights {
		weights[i] = math.Sqrt(uniq[i+1].Sub(uniq[i]).Hours() / 24)
		total += weights[i]
	}
	scale := TimeScale{Points: []ctrlPoint{{uniq[0], 0}}}
	acc := 0.0
	for i, w := range weights {
		acc += w / total
		if i == len(weights)-1 {
			acc = 1
		}
		scale.Points = append(scale.Points, ctrlPoint{uniq[i+1], acc})
	}
	return scale
}
