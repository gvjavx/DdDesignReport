package ddreport

import (
	"strings"
	"testing"
)

func TestExpressions(t *testing.T) {
	sc := scope{
		t: &Template{Thousands: ".", Decimal: ","},
		rows: []any{
			map[string]any{"qty": 2.0, "price": 1500.0},
			map[string]any{"qty": 3.0, "price": 1000.0, "disc": 500.0},
			map[string]any{"qty": "n/a", "price": 99.0}, // not a number: skipped by aggregates
		},
		ctx: []map[string]any{{"qty": 4.0, "price": 2500.0, "name": "Kopi", "zero": 0.0, "tax": map[string]any{"rate": 0.11}, "inf": 7.0}},
	}
	for text, want := range map[string]string{
		"{{qty * price}}":                            "10000",
		"{{qty * price:2}}":                          "10.000,00",
		"{{ qty*price : 0 }}":                        "10.000",
		"{{price - 500}}":                            "2000",
		"{{price + qty * 10}}":                       "2540", // * before +
		"{{(price + qty) * 10}}":                     "25040",
		"{{price / qty}}":                            "625",
		"{{10 - 4 - 3}}":                             "3", // left to right
		"{{100 / 10 / 5}}":                           "2",
		"{{-qty}}":                                   "-4",
		"{{-(qty + 1) * 2}}":                         "-10",
		"{{price * tax.rate:0}}":                     "275",
		"{{1.5 * 2}}":                                "3",
		"{{inf + 1}}":                                "8", // a field called inf is a field, not infinity
		"{{sum(qty * price)}}":                       "6000",
		"{{sum(qty * price):2}}":                     "6.000,00",
		"{{sum(qty * price) / count()}}":             "2000",
		"{{sum(qty) + sum(price)}}":                  "2604",
		"{{avg(qty * 2)}}":                           "5",
		"{{max(price - 1000)}}/{{min(price)}}":       "500/99",
		"{{count(disc)}} of {{count()}}":             "1 of 3", // count(x) counts the rows that have x
		"{{qty * price / sum(qty * price) * 100:1}}": "166,7",
		// no value rather than a wrong one
		"[{{price / zero}}]": "[]",
		"[{{name * 2}}]":     "[]",
		"[{{missing + 1}}]":  "[]",
		"[{{sum()}}]":        "[]",
		"[{{avg(missing)}}]": "[]",
		// not expressions: left alone
		"{{qty price}}":  "{{qty price}}",
		"{{qty *}}":      "{{qty *}}",
		"{{(qty}}":       "{{(qty}}",
		"{{qty)}}":       "{{qty)}}",
		"{{round(qty)}}": "{{round(qty)}}",
		"{{sum(qty}}":    "{{sum(qty}}",
		"{{ }}":          "{{ }}",
		"{{a:b}}":        "{{a:b}}",
		"{{qty % 2}}":    "{{qty % 2}}",
	} {
		if got := sc.resolve(text); got != want {
			t.Errorf("resolve(%q) = %q, want %q", text, got, want)
		}
	}

	// a computed number stays a number in spreadsheets
	if c := sc.cell(Element{Text: "{{qty * price:2}}"}); c != (cell{10000.0, false, 2}) {
		t.Errorf("cell = %+v", c)
	}
	if c := sc.cell(Element{Text: "{{price / zero}}"}); c != (cell{"", false, -1}) {
		t.Errorf("cell without a value = %+v", c)
	}
}

func TestExpressionsInAReport(t *testing.T) {
	tpl := []byte(`{"groupBy":"cat","bands":{
	  "detail":{"height":20,"elements":[{"type":"text","x":0,"w":100,"h":12,"text":"{{name}}"},{"type":"text","x":200,"w":100,"h":12,"text":"{{qty * price:0}}","align":"right"}]},
	  "groupFooter":{"height":20,"elements":[{"type":"text","x":200,"w":100,"h":12,"text":"{{sum(qty * price):0}}","align":"right"}]},
	  "summary":{"height":140,"elements":[{"type":"chart","x":0,"y":0,"w":300,"h":120,"chart":"bar","label":"cat","value":"qty * price"}]}}}`)
	data := []map[string]any{
		{"cat": "A", "name": "x", "qty": 2, "price": 1500},
		{"cat": "B", "name": "y", "qty": 1, "price": 700},
		{"cat": "A", "name": "z", "qty": 3, "price": 1000},
	}
	if csv := string(render(t, tpl, data, "csv")); csv != "x,3000\nz,3000\n,6000\ny,700\n,700\n" {
		t.Errorf("csv = %q", csv)
	}
	html := string(render(t, tpl, data, "html"))
	for _, want := range []string{">3,000<", ">6,000<", ">700<"} { // rows, subtotal, and the chart's value labels
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if n := strings.Count(html, ">6,000<"); n != 2 { // once as the subtotal, once above the chart's bar
		t.Errorf("6,000 appears %d times, want 2", n)
	}
}
