package ddreport

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func render(t *testing.T, tpl []byte, data any, format string) []byte {
	t.Helper()
	out, err := Render(tpl, data, format)
	if err != nil {
		t.Fatalf("%s: %v", format, err)
	}
	if format == "csv" { // every CSV starts with the BOM Excel needs; TestCSVForExcel checks it
		out = bytes.TrimPrefix(out, []byte("\xef\xbb\xbf"))
	}
	return out
}

func TestCSVForExcel(t *testing.T) {
	tpl := []byte(`{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":50,"h":14,"text":"{{a}}"},{"type":"text","x":60,"w":50,"h":14,"text":"{{b}}"}]}}}`)
	rows := []any{map[string]any{"a": "Téh", "b": "1,5"}}
	if out, _ := Render(tpl, rows, "csv"); string(out) != "\xef\xbb\xbfTéh,\"1,5\"\n" {
		t.Errorf("csv = %q", out)
	}
	semi := []byte(strings.Replace(string(tpl), `{"bands"`, `{"csvSeparator":";","bands"`, 1))
	if out, _ := Render(semi, rows, "csv"); string(out) != "\xef\xbb\xbfTéh;1,5\n" {
		t.Errorf("csv with ; = %q", out)
	}
}

func sheet(t *testing.T, xlsx []byte) string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(xlsx), int64(len(xlsx)))
	if err != nil {
		t.Fatalf("xlsx: %v", err)
	}
	f, err := z.Open("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatalf("xlsx: %v", err)
	}
	var b bytes.Buffer
	b.ReadFrom(f)
	return b.String()
}

func TestRender(t *testing.T) {
	tpl, err := os.ReadFile("testdata/invoice.json")
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]map[string]any, 60) // 39 rows fit on a page, so this is 2 pages
	for i := range rows {
		rows[i] = map[string]any{"name": fmt.Sprintf("Item <%d>", i+1), "qty": i + 1, "price": 1500000.5}
	}
	data := map[string]any{
		"number":   "INV-001",
		"customer": map[string]any{"name": "PT Maju & Jaya"},
		"total":    "90.000.030",
		"rows":     rows,
	}

	html := string(render(t, tpl, data, "html"))
	for _, want := range []string{"INVOICE INV-001", "PT Maju &amp; Jaya", "Item &lt;60&gt;", "Page 2 of 2", "Total: 90.000.030", "1500000.5"} {
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if n := strings.Count(html, `<div class="p">`); n != 2 {
		t.Errorf("html: %d pages, want 2", n)
	}

	pdf := render(t, tpl, data, "pdf")
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || bytes.Count(pdf, []byte("/Type /Page\n")) != 2 {
		t.Errorf("pdf: not a 2 page PDF")
	}

	// header, 60 rows, then the summary under the column it is aligned with
	csv := strings.Split(strings.TrimSpace(string(render(t, tpl, data, "csv"))), "\n")
	if len(csv) != 62 || csv[0] != "Item,Qty,Price" || csv[60] != "Item <60>,60,1500000.5" || csv[61] != ",,Total: 90.000.030" {
		t.Errorf("csv: %d lines, first %q, last %q", len(csv), csv[0], csv[len(csv)-1])
	}

	s := sheet(t, render(t, tpl, data, "xlsx"))
	if strings.Count(s, "<row>") != 62 || !strings.Contains(s, `<c s="0"><v>1500000.5</v></c>`) || !strings.Contains(s, `<c s="1" t="inlineStr"><is><t xml:space="preserve">Item<`) {
		t.Errorf("xlsx: want 62 rows, numeric price cells and a bold header")
	}

	if _, err := Render(tpl, data, "docx"); err == nil {
		t.Error("unknown format: want error")
	}
	if _, err := Render([]byte(`{"page":{"width":100,"height":100},"bands":{"detail":{"height":200}}}`), nil, "pdf"); err == nil {
		t.Error("oversized band: want error")
	}
}

const grouped = `{
  "page": {"width": 400, "height": 400, "margin": 20},
  "groupBy": "cat", "thousands": ".", "decimal": ",",
  "bands": {
    "groupHeader": {"height": 16, "elements": [{"type": "text", "x": 0, "y": 0, "w": 200, "h": 14, "text": "{{cat}} ({{count()}})", "bold": true}]},
    "detail": {"height": 14, "elements": [
      {"type": "rect", "x": 0, "y": 0, "w": 300, "h": 14, "border": 1},
      {"type": "text", "x": 0, "y": 0, "w": 100, "h": 12, "text": "{{note}}", "grow": true},
      {"type": "text", "x": 200, "y": 0, "w": 100, "h": 12, "text": "{{amt:2}}", "align": "right"}]},
    "groupFooter": {"height": 16, "elements": [{"type": "text", "x": 200, "y": 0, "w": 100, "h": 14, "text": "{{sum(amt):2}}", "align": "right", "bold": true}]},
    "summary": {"height": 130, "elements": [
      {"type": "text", "x": 0, "y": 0, "w": 300, "h": 14, "text": "Avg {{avg(amt):1}} max {{max(amt)}} of {{count()}}"},
      {"type": "chart", "x": 0, "y": 20, "w": 150, "h": 80, "chart": "pie", "label": "cat", "value": "amt"},
      {"type": "chart", "x": 160, "y": 20, "w": 100, "h": 80, "chart": "bar", "label": "cat", "value": "amt"},
      {"type": "chart", "x": 270, "y": 20, "w": 90, "h": 80, "chart": "line", "label": "cat", "value": "amt"},
      {"type": "image", "x": 0, "y": 105, "w": 10, "h": 10, "src": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="}]}
  }
}`

func TestGroupsAggregatesGrowCharts(t *testing.T) {
	long := "a note long enough that it has to wrap over several lines"
	data := []map[string]any{
		{"cat": "A", "note": long, "amt": 1000.5},
		{"cat": "B", "note": "b", "amt": 2500},
		{"cat": "A", "note": "a2", "amt": 3000}, // joins the first group
	}
	tpl := []byte(grouped)

	html := string(render(t, tpl, data, "html"))
	for _, want := range []string{"A (2)", "B (1)", "1.000,50", ">4.000,50<", ">2.500,00<", "Avg 2.166,8 max 3000 of 3", "<polygon", "<polyline", "A: 4.000,50", "<img"} {
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if strings.Index(html, "A (2)") > strings.Index(html, "B (1)") || strings.Index(html, "a2") > strings.Index(html, "B (1)") {
		t.Error("html: groups out of order")
	}
	// the wrapped note grows, and the box around it stretches by the same amount
	h := func(re string) float64 {
		m := regexp.MustCompile(re).FindStringSubmatch(html)
		if m == nil {
			t.Fatalf("html: no match for %s", re)
		}
		f, _ := strconv.ParseFloat(m[1], 64)
		return f
	}
	note, box := h(`height:([\d.]+)pt;[^>]*>a note long`), h(`height:([\d.]+)pt;border`)
	if note <= 12 || box-14 != note-12 {
		t.Errorf("grow: note is %gpt high, its box %gpt", note, box)
	}

	if pdf := render(t, tpl, data, "pdf"); !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Error("pdf: not a PDF")
	}

	csv := string(render(t, tpl, data, "csv"))
	if want := "A (2),\n" + long + ",1000.5\na2,3000\n,4000.5\nB (1),\nb,2500\n,2500\n\"Avg 2.166,8 max 3000 of 3\",\n"; csv != want {
		t.Errorf("csv:\n%s\nwant:\n%s", csv, want)
	}

	// style 6 = two decimals, 7 = two decimals bold
	s := sheet(t, render(t, tpl, data, "xlsx"))
	if !strings.Contains(s, `<c s="6"><v>1000.5</v></c>`) || !strings.Contains(s, `<c s="7"><v>4000.5</v></c>`) || !strings.Contains(s, `<col min="2" max="2" width="20.0"`) {
		t.Errorf("xlsx: missing styled cells or column widths:\n%s", s)
	}
}

func TestUnicodeFont(t *testing.T) {
	ttf, err := os.ReadFile(`C:\Windows\Fonts\arial.ttf`)
	if err != nil {
		t.Skip("no TrueType font to test with")
	}
	Font = ttf
	defer func() { Font = nil }()
	tpl := []byte(`{"bands":{"detail":{"height":30,"elements":[{"type":"text","x":0,"y":0,"w":60,"h":12,"text":"{{s}}","grow":true,"bold":true}]}}}`)
	pdf := render(t, tpl, []map[string]any{{"s": "Привет <u>мир</u>, <i>Ελληνικά</i>, žluťoučký kůň"}}, "pdf")
	if !bytes.Contains(pdf, []byte("/FontFile2")) {
		t.Error("pdf: TrueType font not embedded")
	}
}

func mustJSON(t *testing.T, path string) any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v
}
