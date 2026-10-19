package ddreport

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// load builds a report the way Render does, ready for layout.
func load(t *testing.T, template string, data any) *report {
	t.Helper()
	r, err := newReport([]byte(template), data)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.initPDF(); err != nil {
		t.Fatal(err)
	}
	return r
}

func rowsOf(amounts ...float64) []any {
	out := make([]any, len(amounts))
	for i, a := range amounts {
		out[i] = map[string]any{"amt": a}
	}
	return out
}

func TestNumber(t *testing.T) {
	en, id, plain := &Template{Thousands: ",", Decimal: "."}, &Template{Thousands: ".", Decimal: ","}, &Template{Decimal: "."}
	for _, c := range []struct {
		t    *Template
		f    float64
		dec  int
		want string
	}{
		{en, 1234567.891, 2, "1,234,567.89"},
		{id, 1234567.891, 2, "1.234.567,89"},
		{plain, 1234567.891, 2, "1234567.89"},
		{en, 1234567.5, 0, "1,234,568"},
		{en, 999, 0, "999"},
		{en, 1000, 0, "1,000"},
		{en, 0, 2, "0.00"},
		{en, -1234.5, 1, "-1,234.5"},
		{en, -0.001, 2, "0.00"}, // rounds to zero: no minus sign
		{en, 0.5, 3, "0.500"},
	} {
		if got := c.t.number(c.f, c.dec); got != c.want {
			t.Errorf("number(%v, %d) = %q, want %q", c.f, c.dec, got, c.want)
		}
	}
}

func TestLookup(t *testing.T) {
	row := map[string]any{"name": "Kopi", "customer": map[string]any{"city": "Bandung"}, "zero": 0.0}
	root := map[string]any{"title": "Report", "name": "root name"}
	for _, c := range []struct {
		path string
		want any
	}{
		{"name", "Kopi"},             // first context wins
		{"title", "Report"},          // falls through to the next one
		{"customer.city", "Bandung"}, // dotted path
		{"customer.missing", nil},    // missing leaf
		{"name.deeper", "root name"}, // row's name is a string, so "name.deeper" is nil there... and in root too
		{"missing", nil},
		{"zero", 0.0}, // a zero value is still a value
	} {
		got := lookup(c.path, row, root)
		if c.path == "name.deeper" {
			c.want = nil
		}
		if got != c.want {
			t.Errorf("lookup(%q) = %v, want %v", c.path, got, c.want)
		}
	}
	if got := lookup("a.b", nil, nil); got != nil {
		t.Errorf("lookup in nil contexts = %v", got)
	}
}

func TestStr(t *testing.T) {
	for _, c := range []struct {
		v    any
		want string
	}{{nil, ""}, {"x", "x"}, {1500000.0, "1500000"}, {0.1, "0.1"}, {true, "true"}, {3, "3"}} {
		if got := str(c.v); got != c.want {
			t.Errorf("str(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	sc := scope{
		t:    &Template{Thousands: ".", Decimal: ","},
		rows: append(rowsOf(10, 2.5, 30), map[string]any{"amt": "not a number"}),
		ctx:  []map[string]any{{"name": "Kopi", "price": 18000.0, "c": map[string]any{"n": "Budi"}}},
	}
	for text, want := range map[string]string{
		"plain text":                    "plain text",
		"{{name}}":                      "Kopi",
		"{{ name }}":                    "Kopi",
		"Hi {{c.n}}, {{name}}!":         "Hi Budi, Kopi!",
		"[{{missing}}]":                 "[]",
		"{{price}}":                     "18000",
		"{{price:2}}":                   "18.000,00",
		"{{price:0}}":                   "18.000",
		"{{name:2}}":                    "Kopi", // a format on a non-number is ignored
		"{{sum(amt)}}":                  "42.5",
		"{{sum(amt):1}}":                "42,5",
		"{{avg(amt):2}}":                "14,17", // the non-numeric row is not counted
		"{{min(amt)}}/{{max(amt)}}":     "2.5/30",
		"{{count()}}":                   "4",
		"{{ sum( amt ) : 0 }}":          "43",
		"{{sum(nothing)}}":              "0",
		"{{avg(nothing)}}{{min(none)}}": "",
		"{{not a placeholder}}":         "{{not a placeholder}}",
		"{single}":                      "{single}",
	} {
		if got := sc.resolve(text); got != want {
			t.Errorf("resolve(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestCell(t *testing.T) {
	sc := scope{t: &Template{Thousands: ",", Decimal: "."}, rows: rowsOf(1, 2), ctx: []map[string]any{{"price": 12.5, "name": "Kopi"}}}
	for _, c := range []struct {
		e    Element
		want cell
	}{
		{Element{Text: "{{price}}"}, cell{12.5, false, -1, "", ""}},  // stays a number
		{Element{Text: "{{price:2}}"}, cell{12.5, false, 2, "", ""}}, // with its format
		{Element{Text: "{{sum(amt):0}}", Bold: true}, cell{3.0, true, 0, "", ""}},
		{Element{Text: "Rp {{price:2}}"}, cell{"Rp 12.50", false, -1, "", ""}}, // mixed text is a string
		{Element{Text: "{{name}}"}, cell{"Kopi", false, -1, "", ""}},
		{Element{Text: "<b>{{price}}</b>"}, cell{12.5, true, -1, "", ""}}, // tags are not part of the value
		{Element{Text: "a <i>b</i>"}, cell{"a b", false, -1, "", ""}},
	} {
		if got := sc.cell(c.e); got != c.want {
			t.Errorf("cell(%q) = %+v, want %+v", c.e.Text, got, c.want)
		}
	}
}

func TestParse(t *testing.T) {
	for _, c := range []struct {
		e    Element
		want []run
	}{
		{Element{Text: "plain"}, []run{{"plain", false, false, false}}},
		{Element{Text: "a <b>b</b> c"}, []run{{"a ", false, false, false}, {"b", true, false, false}, {" c", false, false, false}}},
		{Element{Text: "<b><i>x</i>y</b><u>z</u>"}, []run{{"x", true, true, false}, {"y", true, false, false}, {"z", false, false, true}}},
		{Element{Text: "a<i>b</i>", Bold: true}, []run{{"a", true, false, false}, {"b", true, true, false}}}, // element style applies to every run
		{Element{Text: "<b>never closed"}, []run{{"never closed", true, false, false}}},
		{Element{Text: "1 < 2 <br> <x>"}, []run{{"1 < 2 <br> <x>", false, false, false}}}, // only b, i, u are tags
		{Element{Text: ""}, nil},
	} {
		if got := c.e.parse(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("parse(%q) = %+v, want %+v", c.e.Text, got, c.want)
		}
	}
	if got := (Element{Text: "a <b>b</b> <u>c</u>"}).plain(); got != "a b c" {
		t.Errorf("plain = %q", got)
	}
}

func TestMargins(t *testing.T) {
	top, right, bottom, left := Page{Margin: 36, MarginTop: 50, MarginLeft: 20}.margins()
	if top != 50 || right != 36 || bottom != 36 || left != 20 {
		t.Errorf("margins = %v %v %v %v", top, right, bottom, left)
	}
}

func TestColor(t *testing.T) {
	for in, want := range map[string]string{
		"#aBc123":                "#aBc123",
		"":                       "#000000",
		"red":                    "#000000",
		"#fff":                   "#000000",
		`#000000;background:url`: "#000000", // nothing but #rrggbb reaches a style attribute
	} {
		if got := color(in, "#000000"); got != want {
			t.Errorf("color(%q) = %q, want %q", in, got, want)
		}
	}
	if r, g, b := rgb("#ff8000"); r != 255 || g != 128 || b != 0 {
		t.Errorf("rgb = %d %d %d", r, g, b)
	}
	if r, g, b := rgb("nonsense"); r+g+b != 0 {
		t.Errorf("rgb of invalid colour = %d %d %d, want black", r, g, b)
	}
}

func TestElementDefaults(t *testing.T) {
	var e Element
	if e.size() != 10 || e.align() != "left" || e.leading() != 1.2 || e.stroke() != 1 {
		t.Errorf("defaults = %v %v %v %v", e.size(), e.align(), e.leading(), e.stroke())
	}
	e = Element{FontSize: 14, Align: "right", LineHeight: 2, Border: 3}
	if e.size() != 14 || e.align() != "right" || e.leading() != 2 || e.stroke() != 3 {
		t.Errorf("set values = %v %v %v %v", e.size(), e.align(), e.leading(), e.stroke())
	}
	if (Element{Align: "justify"}).align() != "left" {
		t.Error("unknown alignment should fall back to left")
	}
}

func TestGroups(t *testing.T) {
	rows := []any{
		map[string]any{"cat": "B", "n": 1.0},
		map[string]any{"cat": "A", "n": 2.0},
		map[string]any{"cat": "B", "n": 3.0},
		map[string]any{"n": 4.0}, // no key: its own group
	}
	got := (&Template{GroupBy: "cat"}).groups(rows)
	if len(got) != 3 || len(got[0]) != 2 || len(got[1]) != 1 || len(got[2]) != 1 {
		t.Fatalf("groups = %v", got)
	}
	if got[0][1].(map[string]any)["n"] != 3.0 || got[1][0].(map[string]any)["cat"] != "A" {
		t.Errorf("groups keep neither first-appearance nor row order: %v", got)
	}
	if all := (&Template{}).groups(rows); len(all) != 1 || len(all[0]) != 4 {
		t.Errorf("without GroupBy want one group with every row, got %v", all)
	}
	if none := (&Template{GroupBy: "cat"}).groups(nil); len(none) != 0 {
		t.Errorf("no rows: want no groups, got %v", none)
	}
}

// expand lays out the tables of a band as the report does, without data and without growing.
func expand(b Band) Band {
	r := &report{t: &Template{}}
	var out []Element
	for _, e := range b.Elements {
		if e.Type != "table" {
			out = append(out, e)
			continue
		}
		els, _ := r.grid(e, scope{r.t, nil, nil}, false, false)
		out = append(out, els...)
	}
	b.Elements = out
	return b
}

// edges are the distinct positions of the horizontal and the vertical lines.
func edges(b Band) (horizontal, vertical []float64) {
	for _, e := range b.Elements {
		switch {
		case e.Type == "line" && e.H > e.W && !slices.Contains(vertical, e.X):
			vertical = append(vertical, e.X)
		case e.Type == "line" && e.H <= e.W && !slices.Contains(horizontal, e.Y):
			horizontal = append(horizontal, e.Y)
		}
	}
	return
}

func TestExpandTable(t *testing.T) {
	count := func(b Band) map[string]int {
		n := map[string]int{}
		for _, e := range b.Elements {
			n[e.Type]++
		}
		return n
	}
	b := Band{Elements: []Element{
		{Type: "rect"},
		{Type: "table", X: 10, Y: 5, W: 300, H: 40, Border: 1, Fill: "#eeeeee", Bold: true, FontSize: 9,
			Cells: [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, Cols: []float64{2, 1, 1}, Aligns: []string{"left", "right"}},
	}}
	b = expand(b)
	// the plain rect, the table's background, 6 cells with their edges: lines at 4 x and 3 y positions
	h, v := edges(b)
	if n := count(b); n["rect"] != 2 || n["text"] != 6 || len(v) != 4 || len(h) != 3 || n["table"] != 0 {
		t.Fatalf("expanded to %v, lines at %v and %v", n, h, v)
	}
	var texts []Element
	for _, e := range b.Elements {
		if e.Type == "text" {
			texts = append(texts, e)
		}
	}
	a, bb, d := texts[0], texts[1], texts[3] // row by row
	if a.Text != "a" || d.Text != "d" || bb.Text != "b" {
		t.Errorf("cell order = %q %q %q", a.Text, d.Text, bb.Text)
	}
	if a.X != 13 || a.W != 144 || bb.X != 163 || bb.W != 69 { // columns 150, 75, 75 wide, 3pt padding
		t.Errorf("column geometry: a at %v wide %v, b at %v wide %v", a.X, a.W, bb.X, bb.W)
	}
	if a.Y != 7 || d.Y != 27 || a.H != 16 { // rows 20 high, 2pt padding
		t.Errorf("row geometry: a.Y %v, d.Y %v, a.H %v", a.Y, d.Y, a.H)
	}
	if !a.Bold || a.FontSize != 9 || a.Fill != "" || bb.Align != "right" || texts[5].Align != "" {
		t.Errorf("cell style: %+v / %+v", a, bb)
	}

	plain := Band{Elements: []Element{{Type: "table", W: 100, H: 20, Cells: [][]string{{"x", "y"}}, Cols: []float64{1}}}}
	plain = expand(plain)
	if n := count(plain); n["text"] != 2 || n["line"] != 0 || n["rect"] != 0 {
		t.Errorf("borderless table expanded to %v", n)
	}
	if plain.Elements[0].W != 44 { // a width that is not given is 1, like the others
		t.Errorf("fallback column width = %v", plain.Elements[0].W)
	}

	empty := Band{Elements: []Element{{Type: "table", W: 100, H: 20}}}
	empty = expand(empty)
	if len(empty.Elements) != 0 {
		t.Errorf("empty table should disappear, got %v", empty.Elements)
	}
}

func TestLayout(t *testing.T) {
	r := load(t, `{}`, nil)
	words := func(l textLine) string {
		var s string
		for _, sp := range l.spans {
			s += sp.text
		}
		return s
	}
	lines := func(e Element) []string {
		e.Type, e.runs = "text", e.parse()
		var out []string
		for _, l := range r.layout(e) {
			out = append(out, words(l))
		}
		return out
	}
	for _, c := range []struct {
		name string
		e    Element
		want []string
	}{
		{"fits", Element{Text: "one two", W: 200}, []string{"one two"}},
		{"wraps at spaces", Element{Text: "one two three four", W: 50}, []string{"one two", "three four"}},
		{"newline", Element{Text: "one\n\ntwo", W: 200}, []string{"one", "", "two"}},
		{"no break inside styled word", Element{Text: "aaaa<b>bbbb</b>cccc dd", W: 30}, []string{"aaaabbbbcccc", "dd"}},
		{"long word is cut to fit", Element{Text: "a Supercalifragilistic b", W: 40}, []string{"a", "Supercal", "ifragilisti", "c b"}},
		{"box narrower than a letter", Element{Text: "ab", W: 1}, []string{"a", "b"}},
		{"keeps indentation", Element{Text: "  x", W: 200}, []string{"  x"}},
		{"empty", Element{Text: "", W: 200}, []string{""}},
	} {
		if got := lines(c.e); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: lines = %q, want %q", c.name, got, c.want)
		}
	}
	// a wider font size needs more lines, and widths add up
	if small, big := len(lines(Element{Text: "one two three four", W: 80})), len(lines(Element{Text: "one two three four", W: 80, FontSize: 30})); big <= small {
		t.Errorf("font size 30 wraps to %d lines, size 10 to %d", big, small)
	}
	e := Element{Text: "ab cd", W: 500}
	e.runs = e.parse()
	l := r.layout(e)[0]
	if sum := l.spans[0].w + l.spans[1].w + l.spans[2].w; math.Abs(sum-l.w) > 1e-9 || l.w <= 0 {
		t.Errorf("line width %v, spans add up to %v", l.w, sum)
	}
}

func TestPlaceGrow(t *testing.T) {
	r := load(t, `{}`, nil)
	band := Band{Height: 20, Elements: []Element{
		{Type: "rect", Y: 0, H: 20}, // spans the text: stretches
		{Type: "line", Y: 0, H: 20}, // vertical rule: stretches
		{Type: "text", Y: 2, W: 50, H: 12, Text: "one two three four five six seven", Grow: true}, // grows
		{Type: "text", Y: 2, W: 50, H: 12, Text: "one two three four five six seven"},             // clipped, keeps its size
		{Type: "line", Y: 14, W: 100}, // below it: moves down
		{Type: "rect", Y: 0, H: 5},    // above it: untouched
	}}
	els, h := r.place(band, scope{t: r.t})
	grow := els[2].H - 12
	if grow <= 0 || math.Mod(els[2].H, 12) > 1e-9 { // whole lines of 10pt * 1.2
		t.Fatalf("text height %v, want a multiple of 12 above 12", els[2].H)
	}
	if h != 20+grow || els[0].H != 20+grow || els[1].H != 20+grow {
		t.Errorf("band %v, rect %v, rule %v: want all 20+%v", h, els[0].H, els[1].H, grow)
	}
	if els[3].H != 12 || els[4].Y != 14+grow || els[5].H != 5 || els[5].Y != 0 {
		t.Errorf("clipped text %v, line below at %v, rect above %+v", els[3].H, els[4].Y, els[5])
	}

	// nothing grows: the band is left alone
	still, h := r.place(Band{Height: 20, Elements: []Element{{Type: "text", W: 200, H: 12, Text: "short", Grow: true}, {Type: "line", Y: 14}}}, scope{t: r.t})
	if h != 20 || still[0].H != 12 || still[1].Y != 14 {
		t.Errorf("short text changed the band: h %v, %+v", h, still)
	}
}

func TestChart(t *testing.T) {
	tpl := &Template{Thousands: ".", Decimal: ","}
	rows := []any{
		map[string]any{"cat": "A", "v": 10.0},
		map[string]any{"cat": "B", "v": 30.0},
		map[string]any{"cat": "A", "v": 20.0}, // summed into A
		map[string]any{"cat": "C", "v": -5.0}, // negative: below the zero line, zero in a pie
	}
	types := func(els []Element) map[string]int {
		n := map[string]int{}
		for _, e := range els {
			n[e.Type]++
		}
		return n
	}
	base := Element{X: 100, Y: 50, W: 300, H: 120, Label: "cat", Value: "v"}

	bar := base
	bar.Chart = "bar"
	els := chart(tpl, bar, rows)
	// scale -10..30 in steps of 10: five grid lines with labels, then rect, value, label per bar, then the zero line
	if n := types(els); n["rect"] != 3 || n["text"] != 11 || n["line"] != 6 || n["poly"] != 0 {
		t.Fatalf("bar chart = %v", n)
	}
	if els[1].Text != "-10" || els[9].Text != "30" || els[9].Y != 50+10-4.5 || els[8].Y != 60 {
		t.Errorf("scale %q..%q, top label at %v", els[1].Text, els[9].Text, els[9].Y)
	}
	a, b, c := els[10], els[13], els[16]
	if a.H != b.H || a.H != 75 || a.Y != 60 || c.Y != 135 || c.H != 12.5 { // plot of 100pt for 40 units, zero at 85
		t.Errorf("bars %+v %+v %+v", a, b, c)
	}
	if axis := 15.7; math.Abs(a.X-(100+axis+(300-axis)/3*0.15)) > 1e-9 || a.Fill != palette[0] {
		t.Errorf("first bar %+v", a)
	}
	if els[11].Text != "30" || els[12].Text != "A" || els[17].Text != "-5" || els[17].Y != 50+85-9 {
		t.Errorf("labels %q %q %q at %v", els[11].Text, els[12].Text, els[17].Text, els[17].Y)
	}
	if zero := els[len(els)-1]; zero.Y != 135 || zero.Color != "#888888" {
		t.Errorf("zero line %+v", zero)
	}
	bar.Color = "#123456"
	if chart(tpl, bar, rows)[10].Fill != "#123456" {
		t.Error("bar colour not applied")
	}

	line := base
	line.Chart = "line"
	els = chart(tpl, line, rows)
	if n := types(els); n["poly"] != 1 || n["rect"] != 3 || n["text"] != 11 {
		t.Fatalf("line chart = %v", n)
	}
	for _, e := range els {
		if e.Type == "poly" && (len(e.Points) != 6 || e.Fill != "" || e.Points[1] != 10 || e.Points[5] != 97.5) {
			t.Errorf("line poly %+v", e)
		}
	}

	pie := base
	pie.Chart = "pie"
	els = chart(tpl, pie, rows)
	if n := types(els); n["poly"] != 3 || n["rect"] != 3 || n["text"] != 3 { // slices, legend swatches, legend labels
		t.Fatalf("pie chart = %v", n)
	}
	first := els[0] // A and B are both half of the pie
	if first.Fill != palette[0] || first.Points[0] != 60 || first.Points[1] != 60 || math.Abs(first.Points[2]-60) > 1e-9 || first.Points[3] != 0 {
		t.Errorf("first slice starts at %v", first.Points[:4])
	}
	last := first.Points[len(first.Points)-2:]
	if math.Abs(last[0]-60) > 1e-9 || math.Abs(last[1]-120) > 1e-9 { // half a turn later: straight down
		t.Errorf("first slice ends at %v", last)
	}
	if els[2].Text != "A: 30" {
		t.Errorf("legend %q", els[2].Text)
	}

	if got := chart(tpl, bar, nil); got != nil {
		t.Errorf("chart without rows = %v", got)
	}
	zero := chart(tpl, pie, []any{map[string]any{"cat": "A", "v": 0.0}})
	if n := types(zero); n["poly"] != 0 || n["text"] != 1 {
		t.Errorf("all-zero pie = %v", n)
	}
}

const paged = `{"page":{"width":200,"height":200,"margin":10,"marginTop":20},
  "groupBy":"g","bands":{
  "header":{"height":20,"elements":[{"type":"text","text":"{{page}}/{{pages}}","w":50,"h":12}]},
  "groupHeader":{"height":10,"elements":[{"type":"text","text":"{{g}}","w":50,"h":10}]},
  "detail":{"height":30,"elements":[{"type":"text","text":"{{n}}","x":5,"y":1,"w":50,"h":12}]},
  "footer":{"height":10,"elements":[{"type":"line","w":180}]}}}`

func TestPaginate(t *testing.T) {
	data := []map[string]any{{"g": "A", "n": 1}, {"g": "A", "n": 2}, {"g": "A", "n": 3}, {"g": "A", "n": 4}, {"g": "B", "n": 5}}
	r := load(t, paged, data)
	// 200 high, 20 top + 10 bottom margin, header 20, footer 10: 140 left for a 10pt group header and 30pt rows
	pages := r.paginate(len(r.paginate(0)))
	if len(pages) != 2 {
		t.Fatalf("%d pages, want 2", len(pages))
	}
	find := func(page []Element, text string) Element {
		for _, e := range page {
			if e.Text == text {
				return e
			}
		}
		t.Fatalf("no element %q on the page", text)
		return Element{}
	}
	if e := find(pages[0], "1/2"); e.X != 10 || e.Y != 20 { // left margin, top margin
		t.Errorf("header at %v,%v", e.X, e.Y)
	}
	find(pages[1], "2/2")
	if e := find(pages[0], "A"); e.Y != 40 {
		t.Errorf("group header at %v", e.Y)
	}
	if e := find(pages[0], "1"); e.X != 15 || e.Y != 51 {
		t.Errorf("first row at %v,%v", e.X, e.Y)
	}
	find(pages[0], "4")
	// B's header would still fit under the 4 rows, but its first row would not: both move to page 2
	if e := find(pages[1], "B"); e.Y != 40 {
		t.Errorf("second group at %v", e.Y)
	}
	if e := find(pages[1], "5"); e.Y != 51 {
		t.Errorf("row after the page break at %v", e.Y)
	}
	for _, page := range pages {
		if foot := page[1]; foot.Type != "line" || foot.Y != 180 { // after the header text: the footer, at the bottom margin
			t.Errorf("footer %+v", page[1])
		}
	}

	r.t.GroupPageBreak = true
	r.rows = r.rows[2:] // A, A, B would fit on one page
	if n := len(r.paginate(0)); n != 2 {
		t.Errorf("GroupPageBreak: %d pages, want 2", n)
	}
	r.t.GroupPageBreak = false
	if n := len(r.paginate(0)); n != 1 {
		t.Errorf("without GroupPageBreak: %d pages, want 1", n)
	}

	// no rows still gives one page with header and footer
	if pages := load(t, paged, nil).paginate(0); len(pages) != 1 || len(pages[0]) != 2 {
		t.Errorf("empty report = %v", pages)
	}

	// a row taller than the page goes on over the next pages, nothing lost, and the next row follows
	tall := load(t, `{"page":{"width":100,"height":100},"bands":{"detail":{"height":90,"elements":[{"type":"text","text":"{{s}}","w":40,"h":10,"grow":true}]}}}`,
		[]map[string]any{{"s": strings.Repeat("word ", 40)}, {"s": "x"}})
	tall.initPDF()
	pages, words := tall.paginate(0), 0
	for _, page := range pages {
		for _, e := range page {
			for _, p := range e.pieces() {
				words += strings.Count(p.text, "word")
			}
		}
	}
	last := pages[len(pages)-1]
	if len(pages) < 3 || words != 40 || last[len(last)-1].pieces()[0].text != "x" {
		t.Errorf("oversized row: %d pages, %d of 40 words, last %+v", len(pages), words, last[len(last)-1])
	}
	// one line taller than a whole page cannot be split: placed once, no endless loop
	huge := load(t, `{"page":{"width":100,"height":100},"bands":{"detail":{"height":10,"elements":[{"type":"text","text":"x","w":40,"h":10,"fontSize":200,"grow":true}]}}}`, []any{1})
	huge.initPDF()
	if n := len(huge.paginate(0)); n != 1 {
		t.Errorf("one huge line: %d pages", n)
	}

	back := load(t, `{"page":{"background":"#ffeecc","backgroundImage":"data:image/png;base64,AAAA"},"bands":{"header":{"height":10,"elements":[{"type":"rect"}]}}}`, nil).paginate(0)[0]
	if len(back) != 3 || back[0].Fill != "#ffeecc" || back[0].W != 595 || back[1].Type != "image" || back[1].H != 842 {
		t.Errorf("background elements = %+v", back)
	}
}

func TestTable(t *testing.T) {
	tpl := `{"groupBy":"g","bands":{
	  "header":{"height":20,"elements":[{"type":"text","text":"ignored title","w":100}]},
	  "columnHeader":{"height":20,"elements":[{"type":"text","x":200,"w":80,"text":"Price","align":"right","bold":true},{"type":"text","x":0,"w":150,"text":"Name"}]},
	  "groupHeader":{"height":20,"elements":[{"type":"text","x":0,"w":300,"text":"Group {{g}}"}]},
	  "detail":{"height":20,"elements":[
	    {"type":"text","x":200,"w":80,"text":"{{price:2}}","align":"right"},
	    {"type":"rect","x":0,"w":300},
	    {"type":"text","x":0,"w":150,"text":"{{name}}"}]},
	  "groupFooter":{"height":20,"elements":[
	    {"type":"text","x":100,"w":180,"text":"{{sum(price)}}","align":"right"},
	    {"type":"text","x":0,"w":60,"text":"Sub"},{"type":"text","x":70,"w":60,"text":"total"}]},
	  "summary":{"height":20,"elements":[{"type":"chart","label":"g","value":"price"},{"type":"text","x":0,"w":280,"text":"{{count()}} rows","align":"center"}]},
	  "footer":{"height":20,"elements":[{"type":"text","text":"ignored page {{page}}"}]}}}`
	data := []map[string]any{{"g": "A", "name": "x", "price": 1.5}, {"g": "B", "name": "y", "price": 2}, {"g": "A", "name": "z", "price": 3}}
	cells, widths := load(t, tpl, data).table()

	var got [][]any
	for _, row := range cells {
		line := make([]any, len(row))
		for i, c := range row {
			line[i] = c.v
		}
		got = append(got, line)
	}
	want := [][]any{
		{"Name", "Price"}, // columns follow the detail band, left to right
		{"Group A", nil},
		{"x", 1.5},
		{"z", 3.0},
		{"Sub total", 4.5}, // right-aligned total lands under Price by its right edge; two texts share column one
		{"Group B", nil},
		{"y", 2.0},
		{"Sub", 2.0},
		{"3 rows", nil}, // centred over the report: nearest column centre
	}
	want[7][0] = "Sub total"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("table =\n%v\nwant\n%v", got, want)
	}
	if !reflect.DeepEqual(widths, []float64{150, 80}) {
		t.Errorf("widths = %v", widths)
	}
	if !cells[0][1].bold || cells[0][0].bold || cells[2][1].dec != 2 || cells[4][1].dec != -1 {
		t.Errorf("cell styles: %+v %+v %+v", cells[0], cells[2], cells[4])
	}

	// without a detail band the column header defines the columns
	cells, _ = load(t, `{"bands":{"columnHeader":{"height":10,"elements":[{"type":"text","x":50,"text":"B"},{"type":"text","x":0,"text":"A"}]}}}`, nil).table()
	if len(cells) != 1 || cells[0][0].v != "A" || cells[0][1].v != "B" {
		t.Errorf("header-only table = %+v", cells)
	}
	// and with no text at all there is nothing to export
	if cells, _ := load(t, `{}`, data).table(); len(cells) != 0 {
		t.Errorf("empty template exported %v", cells)
	}
}

func TestNewReport(t *testing.T) {
	const tpl = `{"dataKey":"items"}`
	type item struct {
		Name string `json:"name"`
	}
	for name, data := range map[string]any{
		"object with custom key": map[string]any{"items": []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}},
		"array of maps":          []map[string]any{{"name": "a"}, {"name": "b"}},
		"array of structs":       []item{{"a"}, {"b"}},
		"struct": struct {
			Items []item `json:"items"`
		}{[]item{{"a"}, {"b"}}},
	} {
		r, err := newReport([]byte(tpl), data)
		if err != nil || len(r.rows) != 2 || r.rows[1].(map[string]any)["name"] != "b" {
			t.Errorf("%s: rows %v, err %v", name, r.rows, err)
		}
	}
	for name, data := range map[string]any{"nil": nil, "wrong key": map[string]any{"rows": []any{1}}, "scalar": 42, "rows not an array": map[string]any{"items": "x"}} {
		if r, err := newReport([]byte(tpl), data); err != nil || len(r.rows) != 0 {
			t.Errorf("%s: rows %v, err %v", name, r.rows, err)
		}
	}

	r, err := newReport([]byte(`{}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tt := r.t; tt.Page.Width != 595 || tt.Page.Height != 842 || tt.DataKey != "rows" || tt.Thousands != "," || tt.Decimal != "." {
		t.Errorf("defaults = %+v", tt)
	}
	// an explicit decimal sign with no thousands separator means: no grouping
	if r, _ := newReport([]byte(`{"decimal":","}`), nil); r.t.Thousands != "" {
		t.Errorf("thousands = %q, want none", r.t.Thousands)
	}

	if _, err := newReport([]byte(`{"page":`), nil); err == nil {
		t.Error("broken template JSON: want error")
	}
	if _, err := newReport([]byte(`{}`), func() {}); err == nil {
		t.Error("data that cannot be marshalled: want error")
	}
}

func TestRenderWordFeatures(t *testing.T) {
	tpl := []byte(`{
	  "page":{"width":400,"height":300,"marginTop":30,"marginRight":10,"marginBottom":10,"marginLeft":40,"background":"#fffbe6"},
	  "bands":{
	    "header":{"height":30,"elements":[
	      {"type":"text","x":0,"y":0,"w":300,"h":20,"text":"Plain <b>bold {{who}}</b> <i>it</i><u>un</u>","font":"Times","lineHeight":1.5,"fill":"#ffff00"}]},
	    "columnHeader":{"height":20,"elements":[
	      {"type":"table","x":0,"y":0,"w":300,"h":20,"border":1,"bold":true,"fill":"#eeeeee","cells":[["Name","Price"]],"cols":[2,1],"aligns":["left","right"]}]},
	    "detail":{"height":20,"elements":[
	      {"type":"table","x":0,"y":0,"w":300,"h":20,"border":1,"cells":[["{{name}}","{{price:2}}"]],"cols":[2,1],"aligns":["left","right"]}]}}}`)
	data := map[string]any{"who": "<Budi>", "rows": []map[string]any{{"name": "Kopi", "price": 18000}, {"name": "Teh", "price": 12000.5}}}

	html := string(render(t, tpl, data, "html"))
	for _, want := range []string{
		`Plain <b>bold &lt;Budi&gt;</b> <i>it</i><u>un</u>`, // styled runs, data escaped
		`font-family:'Times New Roman'`, `line-height:1.5`, `background:#ffff00`,
		`<div style="left:0pt;top:0pt;width:400pt;height:300pt;background:#fffbe6"></div>`, // page background first
		`left:43pt;top:62pt;`, // first header cell: left margin 40 + 3, top margin 30 + header 30 + 2
		`<b>Price</b>`, `>18,000.00<`, `text-align:right`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if strings.Index(html, "#fffbe6") > strings.Index(html, "Plain") {
		t.Error("html: background is not behind the content")
	}

	if pdf := render(t, tpl, data, "pdf"); !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Error("pdf: not a PDF")
	}
	if csv := string(render(t, tpl, data, "csv")); csv != "Name,Price\nKopi,18000\nTeh,12000.5\n" {
		t.Errorf("csv = %q", csv)
	}

	xls := string(render(t, tpl, data, "xls"))
	for _, want := range []string{
		`<?mso-application progid="Excel.Sheet"?>`,
		`<Column ss:Width="194"/><Column ss:Width="94"/>`,
		`<Cell ss:StyleID="s1"><Data ss:Type="String">Name</Data></Cell>`,    // bold
		`<Cell ss:StyleID="s6"><Data ss:Type="Number">12000.5</Data></Cell>`, // two decimals
		`<Style ss:ID="s6"><NumberFormat ss:Format="#,##0.00"/></Style>`,
	} {
		if !strings.Contains(xls, want) {
			t.Errorf("xls: missing %q", want)
		}
	}
}

func TestRenderIsSafeAgainstTemplateInjection(t *testing.T) {
	tpl := []byte(`{"bands":{"detail":{"height":40,"elements":[
	  {"type":"text","w":500,"h":12,"text":"<script>alert(1)</script>{{x}}","color":"red;background:url(//evil)","align":"left;position:fixed"},
	  {"type":"rect","w":10,"h":10,"fill":"#000000\"><script>","border":1,"color":"expression(1)"},
	  {"type":"poly","w":10,"h":10,"points":[0,0,5,5],"fill":"#000000\"><script>","color":"url(x)"}]}}}`)
	html := string(render(t, tpl, []map[string]any{{"x": `"><img src=x onerror=alert(1)>`}}, "html"))
	for _, bad := range []string{"<script", "<img", "evil", "expression", "position:fixed", "url("} {
		if strings.Contains(html, bad) {
			t.Errorf("html contains %q:\n%s", bad, html)
		}
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;&#34;&gt;&lt;img") {
		t.Errorf("text was not escaped:\n%s", html)
	}
	// the same template must not break the PDF either
	render(t, tpl, nil, "pdf")
}

func TestRenderErrors(t *testing.T) {
	for name, c := range map[string]struct{ tpl, format, want string }{
		"broken template":   {`{`, "pdf", "template"},
		"unknown format":    {`{}`, "docx", "unknown format"},
		"empty format":      {`{}`, "", "unknown format"},
		"detail too tall":   {`{"page":{"width":100,"height":100},"bands":{"detail":{"height":101}}}`, "pdf", "do not fit"},
		"margins too big":   {`{"page":{"width":100,"height":100,"margin":40},"bands":{"detail":{"height":30}}}`, "html", "do not fit"},
		"group too tall":    {`{"page":{"width":100,"height":100},"groupBy":"g","bands":{"groupFooter":{"height":101}}}`, "pdf", "do not fit"},
		"bad image":         {`{"bands":{"header":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"data:image/png;base64,@@@"}]}}}`, "pdf", "image"},
		"unsupported image": {`{"bands":{"header":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"data:image/webp;base64,AAAA"}]}}}`, "pdf", "PNG, JPEG or GIF"},
	} {
		if _, err := Render([]byte(c.tpl), nil, c.format); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one mentioning %q", name, err, c.want)
		}
	}
	// group bands are only checked when grouping is on, and tabular formats never care about page size
	if _, err := Render([]byte(`{"page":{"width":100,"height":100},"bands":{"groupFooter":{"height":101}}}`), nil, "pdf"); err != nil {
		t.Errorf("unused group band: %v", err)
	}
	if _, err := Render([]byte(`{"page":{"width":100,"height":100},"bands":{"detail":{"height":101}}}`), nil, "csv"); err != nil {
		t.Errorf("csv with a tall band: %v", err)
	}
}

func TestChartLabelsUseTheTemplateNumberFormat(t *testing.T) {
	tpl := &Template{Thousands: ".", Decimal: ","}
	els := chart(tpl, Element{W: 300, H: 100, Chart: "bar", Label: "c", Value: "v"}, []any{
		map[string]any{"c": "A", "v": 1296000.0},
		map[string]any{"c": "B", "v": 1234.5},
	})
	texts := map[string]bool{}
	for _, e := range els {
		texts[e.Text] = true
	}
	for _, want := range []string{"1.296.000", "1.234,50", "1.500.000"} { // values, and the top of the scale
		if !texts[want] {
			t.Errorf("no label %q in %v", want, texts)
		}
	}
}

func TestDenseChartsThinTheirLabels(t *testing.T) {
	var rows []any
	for i := range 40 {
		rows = append(rows, map[string]any{"c": fmt.Sprint("item ", i), "v": float64(i)})
	}
	for _, kind := range []string{"bar", "hbar"} {
		labels := 0
		for _, e := range chart(&Template{}, Element{W: 300, H: 100, Chart: kind, Label: "c", Value: "v"}, rows) {
			if strings.HasPrefix(e.Text, "item ") {
				labels++
			}
		}
		if labels == 0 || labels > 15 {
			t.Errorf("%s: %d of 40 labels", kind, labels)
		}
	}
}

func TestHbarNegative(t *testing.T) {
	els := chart(&Template{}, Element{W: 300, H: 50, Chart: "hbar", Label: "c", Value: "v"}, []any{
		map[string]any{"c": "up", "v": 10.0},
		map[string]any{"c": "down", "v": -10.0},
	})
	var bars []Element
	for _, e := range els {
		if e.Type == "rect" {
			bars = append(bars, e)
		}
	}
	zero := els[len(els)-1]
	if len(bars) != 2 || bars[0].X != zero.X || bars[1].X+bars[1].W != zero.X || bars[0].W != bars[1].W {
		t.Errorf("bars %+v around zero at %v", bars, zero.X)
	}
}

func TestTableStyles(t *testing.T) {
	table := Element{Type: "table", W: 100, H: 80, Border: 1, Cells: [][]string{{"h", "h"}, {"a", "a"}, {"b", "b"}, {"t", "t"}},
		Header: true, HeaderFill: "#111111", HeaderColor: "#ffffff", Footer: true, FooterFill: "#222222",
		Stripe: "#eeeeee", TextColor: "#333333", Grid: "rows"}
	b := expand(Band{Elements: []Element{table}})
	var fills []string
	look := map[string]Element{}
	for _, e := range b.Elements {
		switch {
		case e.Type == "rect" && !slices.Contains(fills, fmt.Sprint(e.Fill, "@", e.Y)):
			fills = append(fills, fmt.Sprint(e.Fill, "@", e.Y))
		case e.Type == "text":
			look[e.Text] = e
		}
	}
	// header, the second body row (striped) and the total row; rows are 20pt
	if fmt.Sprint(fills) != "[#111111@0 #eeeeee@40 #222222@60]" {
		t.Errorf("row fills %v", fills)
	}
	if h, a, f := look["h"], look["a"], look["t"]; !h.Bold || h.Color != "#ffffff" || a.Bold || a.Color != "#333333" || !f.Bold || f.Color != "" {
		t.Errorf("header %+v\nbody %+v\nfooter %+v", h, a, f)
	}
	if horizontal, vertical := edges(b); len(vertical) != 0 || len(horizontal) != 5 {
		t.Errorf("rows grid: lines at %v and %v", horizontal, vertical)
	}

	table.Grid = "outer"
	if horizontal, vertical := edges(expand(Band{Elements: []Element{table}})); len(vertical) != 2 || len(horizontal) != 2 {
		t.Errorf("outer grid: lines at %v and %v", horizontal, vertical)
	}
}

func TestBundledFonts(t *testing.T) {
	for _, name := range []string{"Roboto", "Open Sans", "Lato", "Montserrat", "Lora", "Playfair Display", "Great Vibes"} {
		if f := Families[name]; f == nil || f.Regular == nil {
			t.Errorf("%s is not bundled", name)
		}
	}
	if f := Families["Great Vibes"]; f.face(true, true) == nil { // only a regular face: everything falls back to it
		t.Error("Great Vibes has no bold italic fallback")
	}
	tpl := []byte(`{"bands":{"detail":{"height":40,"elements":[
	  {"type":"text","w":300,"h":14,"font":"Roboto","text":"Halo <b>tebal</b> Привет"},
	  {"type":"text","y":20,"w":300,"h":14,"font":"Great Vibes","italic":true,"text":"Sertifikat"}]}}}`)
	if pdf := render(t, tpl, []any{1}, "pdf"); !bytes.Contains(pdf, []byte("/utf8frobotoB")) || !bytes.Contains(pdf, []byte("/utf8fgreatvibesI")) {
		t.Error("pdf does not embed the fonts")
	}
	html := string(render(t, tpl, []any{1}, "html"))
	for _, want := range []string{"font-family:'Roboto',sans-serif", "@font-face{font-family:'Roboto';font-weight:normal;font-style:normal;", "@font-face{font-family:'Roboto';font-weight:bold;", "@font-face{font-family:'Great Vibes';font-weight:normal;font-style:italic;"} {
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if n := strings.Count(html, "@font-face"); n != 3 { // only the faces in use
		t.Errorf("%d faces embedded, want 3", n)
	}
	if plain := string(render(t, []byte(`{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"x"}]}}}`), []any{1}, "html")); strings.Contains(plain, "@font-face") {
		t.Error("a report without bundled fonts embeds one")
	}
}
