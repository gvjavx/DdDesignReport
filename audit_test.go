package ddreport

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// texts are the resolved texts of an HTML render, in order.
func texts(t *testing.T, tpl string, data any) string {
	t.Helper()
	out := string(render(t, []byte(tpl), data, "html"))
	var got []string
	for _, part := range strings.Split(out, `;">`)[1:] {
		if i := strings.Index(part, "</div>"); i >= 0 {
			got = append(got, styleTag.ReplaceAllString(part[:i], "")) // the words, not their bold or italic
		}
	}
	return strings.Join(got, " | ")
}

func oneText(s string) string {
	return `{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":500,"h":14,"text":` + fmt.Sprintf("%q", s) + `}]}}}`
}

func TestNumbersSentAsText(t *testing.T) {
	rows := []any{map[string]any{"price": "18000", "qty": "2", "code": "007"}, map[string]any{"price": 2000.0, "qty": 1.0, "code": "008"}}
	if got := texts(t, oneText("{{price:2}} {{price * qty}} {{sum(price)}} {{code}}"), rows); !strings.HasPrefix(got, "18,000.00 36000 20000 007") {
		t.Errorf("got %q", got)
	}
	// a code stays text in a spreadsheet, a number asked for with decimals becomes a number
	sc := scope{&Template{}, nil, []map[string]any{{"code": "007", "price": "18000"}}}
	if c := sc.cell(Element{Text: "{{code}}"}); c.v != "007" {
		t.Errorf("code cell = %+v", c)
	}
	if c := sc.cell(Element{Text: "{{price:0}}"}); c.v != 18000.0 {
		t.Errorf("price cell = %+v", c)
	}
}

func TestNoBinaryLeftovers(t *testing.T) {
	rows := []any{map[string]any{"a": 0.1, "b": 0.2, "n": 19.99}}
	if got := texts(t, oneText("{{a + b}} {{n * 100}} {{n * 3}}"), rows); got != "0.3 1999 59.97" {
		t.Errorf("got %q", got)
	}
}

func TestExpressionLanguage(t *testing.T) {
	sc := scope{&Template{Locale: "id", now: time.Date(2026, 10, 19, 8, 5, 0, 0, time.UTC)}, nil, []map[string]any{{
		"paid": true, "total": 150.0, "status": "lunas", "jumlah_ü": 3.0, "date": "2026-10-19", "ts": 1792396800.0, "order": 5.0, "note": "x",
	}}}
	for text, want := range map[string]string{
		"{{if(paid, 'LUNAS', 'BELUM')}}":             "LUNAS",
		"{{if(total > 100 and paid, 'besar')}}":      "besar",
		"{{if(total > 1000 or not paid, 'a', 'b')}}": "b",
		"{{if(status = 'lunas', 1, 0)}}":             "1",
		"{{status != 'lunas'}}":                      "false",
		"{{upper(status)}}":                          "LUNAS",
		"{{round(total / 7, 2)}}":                    "21.43",
		"{{jumlah_ü * 2}}":                           "6",
		"{{order + 1}}":                              "6", // a field that starts like "or"
		"{{note}}":                                   "x", // and one that starts like "not"
		"{{date:dd MMMM yyyy}}":                      "19 Oktober 2026",
		"{{date:EEEE, d MMM yy}}":                    "Senin, 19 Okt 26",
		"{{now:dd/MM/yyyy HH:mm}}":                   "19/10/2026 08:05",
		"{{now:'jam' HH}}":                           "jam 08",
		"{{date:yyyy}}":                              "2026",
	} {
		if got := sc.resolve(text); got != want {
			t.Errorf("resolve(%q) = %q, want %q", text, got, want)
		}
	}
	sc.t.Locale = ""
	if got := sc.resolve("{{date:d MMMM}}"); got != "19 October" {
		t.Errorf("english date = %q", got)
	}
}

func TestShowIf(t *testing.T) {
	tpl := `{"bands":{"detail":{"height":20,"elements":[
	  {"type":"text","w":100,"h":14,"text":"{{name}}"},
	  {"type":"text","x":110,"w":100,"h":14,"text":"LUNAS","showIf":"paid"},
	  {"type":"table","x":220,"w":100,"h":14,"border":1,"cells":[["T"]],"showIf":"{{total > 10}}"},
	  {"type":"text","x":330,"w":100,"h":14,"text":"typo","showIf":"paid ="}]}}}`
	rows := []any{map[string]any{"name": "a", "paid": true, "total": 20.0}, map[string]any{"name": "b", "paid": false, "total": 5.0}}
	if got := texts(t, tpl, rows); got != "a | LUNAS | T | typo | b | typo" {
		t.Errorf("got %q", got)
	}
	if html := string(render(t, []byte(tpl), rows[1:], "html")); strings.Contains(html, "border-top") || strings.Contains(html, "border-left") {
		t.Error("a hidden table left its lines")
	}
	if csv := string(render(t, []byte(tpl), rows, "csv")); csv != "a,LUNAS,T,typo\nb,,,typo\n" {
		t.Errorf("csv = %q", csv)
	}
}

func TestTextLongerThanAPage(t *testing.T) {
	var words []string
	for i := 1; i <= 120; i++ {
		words = append(words, fmt.Sprint("kata", i))
	}
	tpl := `{"page":{"width":300,"height":300,"margin":20},"bands":{
	  "header":{"height":20,"elements":[{"type":"text","w":200,"h":14,"text":"{{page}}/{{pages}}"}]},
	  "summary":{"height":20,"elements":[{"type":"text","w":120,"h":14,"grow":true,"align":"justify","text":"` + strings.Join(words, " ") + `"}]}}}`
	html := string(render(t, []byte(tpl), nil, "html"))
	for i := 1; i <= 120; i++ {
		if !strings.Contains(html, fmt.Sprint("kata", i, " ")) && !strings.Contains(html, fmt.Sprint("kata", i, "<")) && !strings.Contains(html, fmt.Sprint("kata", i, "\n")) {
			t.Fatalf("kata%d is lost", i)
		}
	}
	pages := strings.Count(html, `<div class="p">`)
	if pages < 2 || !strings.Contains(html, fmt.Sprintf(">%d/%d<", pages, pages)) {
		t.Errorf("%d pages, last header missing", pages)
	}
	if !strings.Contains(html, "word-spacing:") {
		t.Error("justified lines carried over lost their spacing")
	}
}

func TestFooterThatGrows(t *testing.T) {
	tpl := `{"page":{"width":300,"height":200,"margin":10},"bands":{
	  "detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{x}}"}]},
	  "footer":{"height":20,"elements":[{"type":"text","w":100,"h":14,"grow":true,"text":"a long footer text that wraps over several lines in a narrow box"}]}}}`
	r := load(t, tpl, []map[string]any{{"x": 1}})
	r.initPDF()
	for _, e := range r.paginate(1)[0] {
		if e.Y+e.H > 190+0.01 {
			t.Errorf("%q ends at %v, below the bottom margin", e.Text, e.Y+e.H)
		}
	}
}

func TestGroupHeaderOnEveryPageOfItsGroup(t *testing.T) {
	var rows []map[string]any
	for i := range 14 {
		g := "A"
		if i >= 12 {
			g = "B"
		}
		rows = append(rows, map[string]any{"g": g, "x": float64(i)})
	}
	tpl := `{"page":{"width":300,"height":260,"margin":20},"groupBy":"g","bands":{
	  "groupHeader":{"height":20,"elements":[{"type":"text","w":200,"h":14,"text":"== {{g}} =="}]},
	  "detail":{"height":20,"elements":[{"type":"text","w":200,"h":14,"text":"row {{x}}"}]}}}`
	r := load(t, tpl, rows)
	r.initPDF()
	pages := r.paginate(0)
	if len(pages) != 2 || pages[1][0].Text != "== A ==" {
		t.Errorf("page 2 starts with %q", pages[1][0].Text)
	}
}

func TestGroupBandsWithoutGroupBy(t *testing.T) {
	tpl := `{"bands":{"groupHeader":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"Daftar"}]},
	  "detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{x}}"}]},
	  "groupFooter":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{count()}} baris"}]}}}`
	if got := texts(t, tpl, []any{map[string]any{"x": "a"}, map[string]any{"x": "b"}}); got != "Daftar | a | b | 2 baris" {
		t.Errorf("got %q", got)
	}
}

func TestWrongRowsKey(t *testing.T) {
	tpl := `{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{x}}"}]}}}`
	_, err := Render([]byte(tpl), map[string]any{"items": []any{map[string]any{"x": 1}}}, "pdf")
	if err == nil || !strings.Contains(err.Error(), `"items"`) {
		t.Errorf("err = %v", err)
	}
	// a letter without rows, and data with rows under the right name, are fine
	if _, err := Render([]byte(oneText("x")), map[string]any{"rows": []any{}, "tags": []any{"a"}}, "pdf"); err != nil {
		t.Error(err)
	}
}

func TestBandsDoNotFitSaysWhy(t *testing.T) {
	_, err := Render([]byte(`{"page":{"width":300,"height":200},"bands":{"header":{"height":150},"detail":{"height":80}}}`), nil, "pdf")
	if err == nil || !strings.Contains(err.Error(), "header 150") || !strings.Contains(err.Error(), "detail 80") || !strings.Contains(err.Error(), "200pt") {
		t.Errorf("err = %v", err)
	}
}

func TestNoThousandsSeparator(t *testing.T) {
	tpl := `{"thousands":"none","bands":{"detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{n:2}}"}]}}}`
	if got := texts(t, tpl, []any{map[string]any{"n": 1234567.5}}); got != "1234567.50" {
		t.Errorf("got %q", got)
	}
}

func TestSortBy(t *testing.T) {
	tpl := `{"sortBy":"g, -n","bands":{"detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{g}}{{n}}"}]}}}`
	rows := []any{map[string]any{"g": "b", "n": 1.0}, map[string]any{"g": "a", "n": "2"}, map[string]any{"g": "a", "n": 10.0}, map[string]any{"n": 5.0}}
	if got := texts(t, tpl, rows); got != "a10 | a2 | b1 | 5" {
		t.Errorf("got %q", got)
	}
}

func TestBandPagesAndNewPage(t *testing.T) {
	tpl := `{"page":{"width":200,"height":200,"margin":10},"bands":{
	  "header":{"height":20,"pages":"first","elements":[{"type":"text","w":100,"h":14,"text":"TITLE"}]},
	  "detail":{"height":30,"elements":[{"type":"text","w":100,"h":14,"text":"{{x}}"}]},
	  "summary":{"height":20,"newPage":true,"elements":[{"type":"text","w":100,"h":14,"text":"SUM"}]},
	  "footer":{"height":20,"pages":"last","elements":[{"type":"text","w":100,"h":14,"text":"END {{page}}"}]}}}`
	var rows []map[string]any
	for i := range 6 {
		rows = append(rows, map[string]any{"x": float64(i)})
	}
	html := string(render(t, []byte(tpl), rows, "html"))
	pages := strings.Split(html, `<div class="p">`)[1:]
	if len(pages) != 3 || strings.Count(html, "TITLE") != 1 || !strings.Contains(pages[0], "TITLE") ||
		strings.Count(html, "END") != 1 || !strings.Contains(pages[2], "END 3") || !strings.Contains(pages[2], "SUM") || strings.Contains(pages[2], ">5<") {
		t.Errorf("%d pages:\n%s", len(pages), strings.Join(pages, "\n---\n"))
	}
}

func TestPicturesFromData(t *testing.T) {
	png := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	tpl := `{"bands":{"detail":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"{{photo}}"}]}}}`
	rows := []any{map[string]any{"photo": png}, map[string]any{"photo": "https://example.com/a.png"}, map[string]any{"photo": "data:image/png;base64,AAAA"}, map[string]any{}}
	if n := strings.Count(string(render(t, []byte(tpl), rows, "html")), "<img"); n != 1 {
		t.Errorf("%d pictures, want only the good one", n)
	}
	render(t, []byte(tpl), rows, "pdf") // the broken ones leave their space empty instead of failing
	for _, kind := range []string{"svg+xml", "webp"} {
		bad := `{"bands":{"detail":{"height":20,"elements":[{"type":"image","w":10,"h":10,"src":"data:image/` + kind + `;base64,AAAA"}]}}}`
		if _, err := Render([]byte(bad), nil, "html"); err == nil || !strings.Contains(err.Error(), "PNG, JPEG or GIF") {
			t.Errorf("%s: err = %v", kind, err)
		}
	}
}

func TestVAlignJustifyAndDiagonal(t *testing.T) {
	tpl := `{"bands":{"detail":{"height":60,"elements":[
	  {"type":"text","w":100,"h":40,"text":"mid","valign":"middle"},
	  {"type":"text","x":110,"w":100,"h":40,"text":"low","valign":"bottom"},
	  {"type":"text","x":220,"w":60,"h":40,"align":"justify","text":"aa bb cc dd ee ff"},
	  {"type":"line","x":300,"w":50,"h":40,"slope":"up"}]}}}`
	html := string(render(t, []byte(tpl), []any{1}, "html"))
	for _, want := range []string{"padding-top:14pt;", "padding-top:28pt;", "word-spacing:", `<line x1="0" y1="40" x2="50" y2="0"`} {
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	render(t, []byte(tpl), []any{1}, "pdf")
}

func TestPieLegendAndLineDots(t *testing.T) {
	var rows []any
	for i := range 20 {
		rows = append(rows, map[string]any{"c": fmt.Sprint("k", i), "v": float64(20 - i)})
	}
	square := chart(&Template{}, Element{W: 140, H: 140, Chart: "pie", Label: "c", Value: "v"}, rows[:3])
	for _, e := range square {
		if e.Type == "text" && e.W < 40 {
			t.Errorf("square pie legend only %vpt wide", e.W)
		}
	}
	lots := chart(&Template{Locale: "id"}, Element{W: 300, H: 100, Chart: "pie", Label: "c", Value: "v"}, rows)
	var legend []string
	for _, e := range lots {
		if e.Y+e.H > 100.01 {
			t.Errorf("%s %q reaches %v, below the chart", e.Type, e.Text, e.Y+e.H)
		}
		if e.Type == "text" {
			legend = append(legend, e.Text)
		}
	}
	if len(legend) != 9 || legend[0] != "k0: 20" || !strings.HasPrefix(legend[8], "Lainnya: ") {
		t.Errorf("legend %q", legend)
	}
	dots := 0
	for _, e := range chart(&Template{}, Element{W: 300, H: 100, Chart: "line", Label: "c", Value: "v"}, rows[:1]) {
		if e.Type == "rect" {
			dots++
		}
	}
	if dots != 1 {
		t.Errorf("a line chart with one value has %d dots", dots)
	}
}

func TestBarcodeText(t *testing.T) {
	els := bars(Element{Type: "barcode", W: 100, H: 40, Format: "code128", Text: "ABC-123", ShowText: true})
	if len(els) != 2 || els[0].H != 30 || els[1].Text != "ABC-123" || els[1].Y != 31 {
		t.Errorf("%+v", els)
	}
	if els := bars(Element{Type: "barcode", W: 100, H: 40, Format: "code128", Text: "ABC"}); len(els) != 1 || els[0].H != 40 {
		t.Errorf("without text: %+v", els)
	}
	_ = bytes.Contains
}
