package boxloom

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

// 評価の列の短い語(「p.35」)は、Min の幅を割っても行の見出しが一番長い語の幅を保てる
// うちは折れずに入る(#145)。以前は Min をわずかに割ると評価の列と行の見出しを最小の
// 比で配り、0.x pt の不足が最小の大きさの検査と字の検査を黙って通り、アプリで1字ずつ
// 折れて行の高さが数倍になった。いまは行の見出し(題は字の境で折れるだけ)が先に譲る。
// Min の幅(丸めの誤差 overflowEps の内)を守れば最小の大きさの警告も出ない。
func TestMatrixShortWordNoCliff(t *testing.T) {
	const data = `{"columns": ["東", "西", "北", "南", "中", "外"], %s
  "rows": [
    {"label": "人手の不足と採用の遅れ", %s "cells": [{"status": "support", "text": "p.35 採用の窓口が遠い"}, {"status": "none", "text": "p.135"},
      {"status": "conditional", "text": "p.35"}, {"status": "none", "text": "p.35"}, {"status": "danger"}, {"status": "support", "text": "p.9"}]},
    {"label": "設備", %s "cells": [{"status": "none", "text": "p.35"}, {"status": "danger", "text": "p.12"}, {"status": "support"},
      {"status": "none"}, {"status": "none", "text": "p.135"}, {"status": "none"}]}]}`
	plain := []any{"", "", ""}
	detail := []any{`"detailColumn": "説明",`, `"detail": "採用が遅れて窓口の人手が足りない",`, `"detail": "機械が古い",`}
	cases := []struct {
		name string
		args []any
		mark bool
	}{{"filled", plain, false}, {"mark", plain, true}, {"detailColumn", detail, false}}
	for _, c := range cases {
		d, err := matrixFromIR(json.RawMessage(fmt.Sprintf(data, c.args...)))
		if err != nil {
			t.Fatal(err)
		}
		s := &MatrixSubsystem{Data: d}
		if c.mark {
			v := *defaultMatrixValues()
			v.CellStyle = "mark"
			s.V = &v
		}
		need := 0.0
		for _, r := range d.Rows {
			for _, cell := range r.Cells {
				need = math.Max(need, runsMinWidth(itemRuns(cell.Text, "", nil), s.cellTextSize()))
			}
		}
		minW := s.Measure(Constraint{}).Min.Width
		floor := s.widths().colFloor
		sawNarrow := false
		for w := minW - 3; w < minW+8; w += 0.1 {
			g := s.geom(w)
			if tw := s.cellTextWidth(g.colW); tw < need-floatEps {
				t.Fatalf("%s: width %.2f (Min %.2f): cell text width %.2f < the longest word %.2f; the word breaks letter by letter", c.name, w, minW, tw, need)
			}
			sawNarrow = sawNarrow || g.colW < floor-floatEps
			if w < minW-overflowEps {
				continue // Min を割った幅は subsystem-min が知らせる(それでも語は折れない)
			}
			if ws := lintSubsystemMin(&Spec{Kind: KindSubsystem, Subsystem: s, Box: LBox{Width: w, Height: 1000}}); len(ws) > 0 {
				t.Fatalf("%s: width %.2f: %v", c.name, w, ws)
			}
		}
		if !sawNarrow {
			t.Errorf("%s: the scan never reached the narrow case (evaluation columns below their floor); the example does not test the cliff", c.name)
		}
	}
}
