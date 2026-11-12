package boxloom

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// reportPairsDeck は架空の対の1枚:左に2行ほどの「分かったこと」5つ、右にそれぞれ
// 3項目の打ち手(Backlog #78 の試しと同じ量)。
func reportPairsDeck() []byte {
	var b strings.Builder
	b.WriteString("---\ntitle: 読む資料の試し\n---\n\n# 窓口の待ち時間を見直す\n\n")
	b.WriteString(":::lead\n聞き取りと記録から分かったことごとに、打ち手を3つずつ決めた\n:::\n\n")
	b.WriteString(":::::pairs 分かったこと | 打ち手\n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, ":::subject\n分かったこと %d:月曜と金曜の午前に来る人が全体の半分を占め、そのうち三割は書類の不足で二度来ている。待ちの長さは昼前に最も長くなる\n:::\n", i)
		fmt.Fprintf(&b, ":::response\n- 打ち手 %d-1:予約の枠を午後に広げる\n- 打ち手 %d-2:要る書類の一覧を前もって送る\n- 打ち手 %d-3:混む時間を案内に載せる\n:::\n", i, i, i)
	}
	b.WriteString(":::::\n")
	return []byte(b.String())
}

// report は読む資料向けの密なテンプレートで、light ではみ出す量の対の1枚が収まり、
// pairs を自分の組み方で組む(layout-overflow も component-fallback も出ない)。
func TestReportFitsDensePairs(t *testing.T) {
	data := reportPairsDeck()
	res := Check("pairs.bx.md", data, CheckOptions{Template: "report"})
	if len(res.Errors) > 0 {
		t.Fatalf("errors %+v", res.Errors)
	}
	for _, w := range res.Warnings {
		switch w.Code {
		case "layout-overflow", "component-fallback", "title-wrap":
			t.Errorf("report: %+v", w)
		}
	}
	// 同じ量は light では収まらない(試しの前提)。
	overflow := false
	for _, w := range Check("pairs.bx.md", data, CheckOptions{Template: "light"}).Warnings {
		overflow = overflow || w.Code == "layout-overflow"
	}
	if !overflow {
		t.Error("light: expected layout-overflow for the dense pairs slide")
	}
}

// report の字の段は本文 12pt・注 8pt を下限とする(8pt より小さい段を使わない。
// Gantt の寸法の差し替えも同じ)。
func TestReportTypeFloor(t *testing.T) {
	data, err := valueFiles.ReadFile("templates/report.json")
	if err != nil {
		t.Fatal(err)
	}
	var f templateFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{f.Values, *f.Subsystems.Gantt} {
		rv := reflect.ValueOf(v)
		for i := 0; i < rv.NumField(); i++ {
			if rv.Type().Field(i).Tag.Get("scale") == "type" && rv.Field(i).Float() < 8 {
				t.Errorf("%s = %v (< 8pt)", rv.Type().Field(i).Name, rv.Field(i).Float())
			}
		}
	}
	v := f.Values
	for name, x := range map[string]float64{"body": v.Body, "takeawayText": v.TakeawayText, "quoteText": v.QuoteText, "optionLabel": v.OptionLabel, "columnTitle": v.ColumnTitle} {
		if x < 12 {
			t.Errorf("%s = %v (< 12pt)", name, x)
		}
	}
}
