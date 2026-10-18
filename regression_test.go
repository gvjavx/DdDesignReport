package ddreport

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Each test here pins down a bug that was found in an audit and fixed.

func TestDeeplyNestedExpressionCannotCrash(t *testing.T) {
	sc := scope{t: &Template{}}
	for name, expr := range map[string]string{
		"parentheses":    strings.Repeat("(", 2_000_000) + "1" + strings.Repeat(")", 2_000_000),
		"minus signs":    strings.Repeat("-", 2_000_000) + "1",
		"function calls": strings.Repeat("sum(", 500_000) + "1" + strings.Repeat(")", 500_000),
	} {
		text := "{{" + expr + "}}"
		if got := sc.resolve(text); got != text { // too deep to be an expression: left as written
			t.Errorf("%s: resolved to %d bytes", name, len(got))
		}
	}
	// nesting people actually write still works
	if got := sc.resolve("{{((((1 + 2)))) * -(-(3))}}"); got != "9" {
		t.Errorf("ordinary nesting = %q", got)
	}
}

func TestStackedGrowingTexts(t *testing.T) {
	r := load(t, `{}`, nil)
	long := strings.Repeat("word ", 12) // 6 lines of 12pt in a 60pt wide box
	band := func(a, b string) ([]Element, float64) {
		return r.place(Band{Height: 60, Elements: []Element{
			{Type: "text", Y: 0, W: 60, H: 12, Text: a, Grow: true},
			{Type: "text", Y: 20, W: 60, H: 12, Text: b, Grow: true},
			{Type: "text", Y: 40, W: 60, H: 12, Text: "END"},
			{Type: "rect", Y: 0, W: 60, H: 60}, // a frame around all three
		}}, scope{t: r.t})
	}
	for _, c := range []struct {
		a, b               string
		second, end, total float64
	}{
		{"x", "x", 20, 40, 60},
		{long, "x", 80, 100, 120},  // the first pushes both
		{"x", long, 20, 100, 120},  // the second pushes only what is under it
		{long, long, 80, 160, 180}, // pushes add up
	} {
		els, h := band(c.a, c.b)
		if els[1].Y != c.second || els[2].Y != c.end || h != c.total || els[3].H != c.total {
			t.Errorf("a=%d b=%d chars: second at %v, END at %v, band %v, frame %v; want %v, %v, %v", len(c.a), len(c.b), els[1].Y, els[2].Y, h, els[3].H, c.second, c.end, c.total)
		}
	}

	// side by side: the taller one decides, they do not add up
	els, h := r.place(Band{Height: 20, Elements: []Element{
		{Type: "text", X: 0, Y: 0, W: 60, H: 12, Text: long, Grow: true},
		{Type: "text", X: 70, Y: 0, W: 60, H: 12, Text: strings.Repeat("word ", 4), Grow: true},
		{Type: "line", Y: 14, W: 130},
	}}, scope{t: r.t})
	if els[0].H != 72 || els[1].H != 24 || els[2].Y != 74 || h != 80 {
		t.Errorf("side by side: heights %v and %v, line at %v, band %v", els[0].H, els[1].H, els[2].Y, h)
	}

	// a two-row table whose cells both wrap keeps its rows apart
	r = load(t, `{"bands":{"detail":{"height":40,"elements":[{"type":"table","w":66,"h":40,"border":1,"grow":true,"cells":[["{{a}}"],["{{a}}"]]}]}}}`, nil)
	els, h = r.place(r.t.Bands.Detail, scope{t: r.t, ctx: []map[string]any{{"a": long}}})
	var cells []Element
	for _, e := range els {
		if e.Type == "text" {
			cells = append(cells, e)
		}
	}
	grown := cells[0].H - 16
	if grown <= 0 || cells[1].Y != 22+grown || h != 40+2*grown {
		t.Errorf("table: first cell %v high, second at %v, band %v", cells[0].H, cells[1].Y, h)
	}
	for _, e := range els {
		if e.Type == "line" && e.H > e.W && e.H != h { // the vertical rules span the grown table
			t.Errorf("vertical rule is %v high, band is %v", e.H, h)
		}
	}
}

func values(cells [][]cell) [][]any {
	out := make([][]any, len(cells))
	for i, row := range cells {
		out[i] = make([]any, len(row))
		for j, c := range row {
			out[i][j] = c.v
		}
	}
	return out
}

func TestTabularKeepsEveryTextApart(t *testing.T) {
	// a receipt: the name on one line, quantity and amount on the next
	receipt := `{"bands":{
	  "detail":{"height":24,"elements":[
	    {"type":"text","x":116,"y":11,"w":90,"h":10,"text":"{{qty * price}}","align":"right"},
	    {"type":"text","x":0,"y":11,"w":110,"h":10,"text":"{{qty}} x {{price}}"},
	    {"type":"text","x":0,"y":0,"w":206,"h":11,"text":"{{name}}"}]},
	  "summary":{"height":60,"elements":[
	    {"type":"text","x":0,"y":8,"w":100,"h":14,"text":"TOTAL"},
	    {"type":"text","x":106,"y":8,"w":100,"h":14,"text":"{{sum(qty * price)}}","align":"right"},
	    {"type":"text","x":0,"y":26,"w":100,"h":10,"text":"Cash"},
	    {"type":"text","x":106,"y":26,"w":100,"h":10,"text":"{{cash}}","align":"right"}]}}}`
	cells, widths := load(t, receipt, map[string]any{"cash": 100, "rows": []any{
		map[string]any{"name": "Nasi", "qty": 2, "price": 22},
		map[string]any{"name": "Teh", "qty": 1, "price": 5},
	}}).table()
	want := [][]any{
		{"Nasi", "2 x 22", 44.0}, // reading order: name, then the second line left to right
		{"Teh", "1 x 5", 5.0},
		{nil, "TOTAL", 49.0}, // each line of the summary is its own row, the amount under the amounts
		{nil, "Cash", 100.0},
	}
	if got := values(cells); !reflect.DeepEqual(got, want) {
		t.Errorf("receipt =\n%v\nwant\n%v", got, want)
	}
	if !reflect.DeepEqual(widths, []float64{206, 110, 90}) {
		t.Errorf("widths = %v", widths)
	}

	// a table with two rows per data row
	grid := `{"bands":{"detail":{"height":40,"elements":[{"type":"table","w":120,"h":40,"cells":[["{{a}}","{{b}}"],["{{c}}","{{d}}"]]}]}}}`
	if csv := string(render(t, []byte(grid), []map[string]any{{"a": 1, "b": 2, "c": 3, "d": 4}}, "csv")); csv != "1,2,3,4\n" {
		t.Errorf("two-row table csv = %q", csv)
	}
}

func TestTabularForDocumentsWithoutRows(t *testing.T) {
	letter := []byte(`{"bands":{
	  "header":{"height":40,"elements":[{"type":"text","x":0,"y":0,"w":300,"h":20,"text":"<b>{{org}}</b>"}]},
	  "summary":{"height":100,"elements":[
	    {"type":"text","x":200,"y":0,"w":100,"h":12,"text":"{{date}}","align":"right"},
	    {"type":"text","x":0,"y":0,"w":60,"h":12,"text":"Number"},
	    {"type":"text","x":62,"y":1,"w":100,"h":12,"text":"{{no}}"},
	    {"type":"text","x":0,"y":40,"w":300,"h":40,"text":"{{body}}"}]},
	  "footer":{"height":20,"elements":[{"type":"text","text":"page {{page}}"}]}}}`)
	data := map[string]any{"org": "Yayasan", "date": "14 Okt", "no": 42, "body": "Dengan hormat"}
	if csv := string(render(t, letter, data, "csv")); csv != "Yayasan\nNumber,42,14 Okt\nDengan hormat\n" {
		t.Errorf("letter csv = %q", csv)
	}
	s := sheet(t, render(t, letter, data, "xlsx"))
	if strings.Count(s, "<row>") != 3 || !strings.Contains(s, `<c s="1" t="inlineStr"><is><t xml:space="preserve">Yayasan<`) || !strings.Contains(s, `<v>42</v>`) {
		t.Errorf("letter xlsx: %s", s)
	}
}

func TestAggregateInEveryRowStaysLinear(t *testing.T) {
	const n = 20000
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{"g": i % 4, "q": 1, "p": 2}
	}
	tpl := []byte(`{"groupBy":"g","bands":{"detail":{"height":12,"elements":[{"type":"text","w":100,"h":10,"text":"{{q * p / sum(q * p) * 100:3}}"}]},
	  "groupFooter":{"height":12,"elements":[{"type":"text","w":100,"h":10,"text":"{{sum(q * p)}}"}]}}}`)
	start := time.Now()
	out := string(render(t, tpl, rows, "csv"))
	if took := time.Since(start); took > 5*time.Second { // was minutes when every row recomputed the total
		t.Errorf("%d rows took %v", n, took)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// each of the 4 groups: 5000 rows of 0.02% and a footer of 10000
	if len(lines) != n+4 || lines[0] != "0.02" || lines[5000] != "10000" || lines[len(lines)-1] != "10000" {
		t.Errorf("%d lines, first %q, footer %q", len(lines), lines[0], lines[5000])
	}
}

func TestFieldNamesThatAreNotWords(t *testing.T) {
	sc := scope{
		t:    &Template{Thousands: ",", Decimal: "."},
		rows: []any{map[string]any{"unit-price": 5.0, "qty": 2.0}, map[string]any{"unit-price": 10.0, "qty": 1.0}},
		ctx:  []map[string]any{{"unit-price": 5.0, "nama barang": "Kopi", "unit": 9.0, "price": 2.0, "qty": 3.0, "a.b": "dotted"}},
	}
	for text, want := range map[string]string{
		"{{unit-price}}":              "5", // the field, not unit minus price
		"{{unit-price:2}}":            "5.00",
		"{{nama barang}}":             "Kopi",
		"{{unit - price}}":            "7", // with spaces it is not a field name here: a subtraction
		"{{[unit-price] * qty}}":      "15",
		"{{[nama barang]}}!":          "Kopi!",
		"{{[a.b]}}":                   "dotted",
		"{{sum([unit-price] * qty)}}": "20",
		"{{[missing] * 2}}|":          "|",
		"{{[unclosed}}":               "{{[unclosed}}",
	} {
		if got := sc.resolve(text); got != want {
			t.Errorf("resolve(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestDecimalsAndInfinity(t *testing.T) {
	sc := scope{t: &Template{Thousands: ",", Decimal: "."}, ctx: []map[string]any{{"x": 1234.5, "big": 1e308}}}
	for text, want := range map[string]string{
		"{{x:10}}":         "1,234.5000000000",
		"{{x:12}}":         "1,234.500000000000",
		"{{x:99}}":         "1,234.500000000000", // capped at 12
		"{{x:0}}":          "1,235",
		"[{{big * 10}}]":   "[]", // not "+Inf"
		"[{{big * 10:2}}]": "[]",
		"[{{-big * 10}}]":  "[]",
	} {
		if got := sc.resolve(text); got != want {
			t.Errorf("resolve(%q) = %q, want %q", text, got, want)
		}
	}
	if got := (&Template{}).number(1/zero(), 2) + str(1/zero()); got != "" {
		t.Errorf("infinity printed as %q", got)
	}
	// twelve decimals have a spreadsheet style too
	tpl := []byte(`{"bands":{"detail":{"height":10,"elements":[{"type":"text","text":"{{x:12}}"}]}}}`)
	if s := sheet(t, render(t, tpl, []map[string]any{{"x": 1.5}}, "xlsx")); !strings.Contains(s, `<c s="26"><v>1.5</v></c>`) {
		t.Errorf("xlsx: %s", s)
	}
	if s := string(render(t, tpl, []map[string]any{{"x": 1.5}}, "xls")); !strings.Contains(s, `<Style ss:ID="s26"><NumberFormat ss:Format="#,##0.000000000000"/></Style>`) {
		t.Error("xls: no style for 12 decimals")
	}
}

func zero() float64 { return 0 }

func TestLiteralAngleBrackets(t *testing.T) {
	e := Element{Text: "use &lt;b> for <b>bold</b>, &lt;/i> and 1 < 2 &lt; 3"}
	want := []run{{"use <b> for ", false, false, false}, {"bold", true, false, false}, {", </i> and 1 < 2 &lt; 3", false, false, false}}
	if got := e.parse(); !reflect.DeepEqual(got, want) {
		t.Errorf("parse = %+v", got)
	}
	if got := e.plain(); got != "use <b> for bold, </i> and 1 < 2 &lt; 3" {
		t.Errorf("plain = %q", got)
	}
}

func TestUnsupportedCharactersAreMarked(t *testing.T) {
	r := load(t, `{}`, nil)
	for in, want := range map[string]string{
		"kopi":      "kopi",
		"café – €5": "caf\xe9 \x96 \x805", // Latin-1 and friends are fine
		"end.":      "end.",               // a real full stop stays
		"日本 ok 😀":   "?? ok ?",
		"Привет":    "??????",
	} {
		if got := r.tr(in); got != want {
			t.Errorf("tr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImagesMustBeDataURIs(t *testing.T) {
	for name, tpl := range map[string]string{
		"remote image":      `{"bands":{"header":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"https://example.com/logo.png"}]}}}`,
		"script url":        `{"bands":{"detail":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"javascript:alert(1)"}]}}}`,
		"not base64":        `{"bands":{"detail":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"data:image/svg+xml,<svg/>"}]}}}`,
		"remote background": `{"page":{"backgroundImage":"http://example.com/bg.png"}}`,
	} {
		for _, format := range []string{"pdf", "html", "csv"} {
			if _, err := Render([]byte(tpl), nil, format); err == nil || !strings.Contains(err.Error(), "data:image") {
				t.Errorf("%s as %s: err = %v", name, format, err)
			}
		}
	}
	// a picture that has not been chosen yet is simply empty
	if _, err := Render([]byte(`{"bands":{"header":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":""}]}}}`), nil, "pdf"); err != nil {
		t.Errorf("empty image: %v", err)
	}
}

func TestHTMLBreaksLinesLikeThePDF(t *testing.T) {
	// a grown text gets exactly as many lines in HTML as its height was measured
	// for, whatever font the browser picks; long words are cut like in the PDF
	out := string(render(t, []byte(`{"bands":{"detail":{"height":12,"elements":[
	  {"type":"text","w":40,"h":12,"grow":true,"text":"abcdefghijklmnopqrstuvwxyz one two"}]}}}`), []any{1}, "html"))
	m := regexp.MustCompile(`height:([\d.]+)pt;[^>]*>([^<]*)</div>`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no text in %s", out)
	}
	lines := strings.Count(m[2], "\n") + 1
	if lines < 3 || m[1] != fmt.Sprint(float64(lines)*12) {
		t.Errorf("%d lines in a %spt box: %q", lines, m[1], m[2])
	}
	if !strings.Contains(out, "white-space:pre}") {
		t.Error("the browser may still wrap the lines again")
	}
}

func TestManyBarcodesStaySmall(t *testing.T) {
	rows := make([]map[string]any, 300)
	for i := range rows {
		rows[i] = map[string]any{"q": i}
	}
	tpl := []byte(`{"page":{"width":200,"height":120},"bands":{"detail":{"height":110,"elements":[{"type":"barcode","w":100,"h":100,"text":"https://example.com/p/{{q}}"}]}}}`)
	out := render(t, tpl, rows, "html")
	if per := len(out) / len(rows); per > 6000 { // was about 13 KB per code as separate elements
		t.Errorf("%s per QR code in HTML", fmt.Sprint(per, " bytes"))
	}
}

// The example in examples/ and the ready-made sales template are the same report.
func TestExampleMatchesTheSalesTemplate(t *testing.T) {
	example := mustJSON(t, "examples/sales.json")
	file := mustJSON(t, "designer/templates/sales-report.json").(map[string]any)
	if !reflect.DeepEqual(example.(map[string]any)["bands"], file["template"].(map[string]any)["bands"]) {
		t.Error("examples/sales.json and designer/templates/sales-report.json have drifted apart")
	}
	if !reflect.DeepEqual(mustJSON(t, "examples/data.json"), file["data"]) {
		t.Error("examples/data.json and the template's sample data have drifted apart")
	}
}
