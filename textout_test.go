package ddreport

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func readyMade(t *testing.T, id string) ([]byte, any) {
	t.Helper()
	raw, err := os.ReadFile("designer/templates/" + id + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Template json.RawMessage
		Data     any
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f.Template, f.Data
}

func TestReceiptAsText(t *testing.T) {
	tpl, data := readyMade(t, "receipt")
	out, err := Render(tpl, data, "txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(out), "\n")
	// 80 mm paper: 48 characters; the shop's name centred, totals right-aligned on the edge
	want := []string{
		"                 WARUNG BU SARI",
		strings.Repeat("-", 48),
		"TOTAL                                     78.000",
		"Kembali                                   22.000",
		"        Terima kasih atas kunjungan Anda",
	}
	for _, w := range want {
		if !slices.Contains(lines, w) {
			t.Errorf("no line %q in:\n%s", w, out)
		}
	}
	if lines[0] != want[0] {
		t.Errorf("first line %q, want the shop's name", lines[0])
	}
	for _, l := range lines {
		if len([]rune(l)) > 48 {
			t.Errorf("line longer than 48: %q", l)
		}
	}
}

func TestTextColumns(t *testing.T) {
	tpl := func(width float64, cols int) []byte {
		b, _ := json.Marshal(map[string]any{"page": map[string]any{"width": width, "height": 400}, "textColumns": cols,
			"bands": map[string]any{"detail": map[string]any{"height": 20, "elements": []any{
				map[string]any{"type": "text", "x": 0, "y": 0, "w": width, "h": 10, "text": "{{name}}", "align": "right"}}}}})
		return b
	}
	for _, c := range []struct {
		width float64
		cols  int
		want  int
	}{{164, 0, 32}, {226, 0, 48}, {595, 0, 80}, {226, 42, 42}} {
		out, err := Render(tpl(c.width, c.cols), []any{map[string]any{"name": "Kopi"}}, "txt")
		if err != nil {
			t.Fatal(err)
		}
		if got := len(strings.TrimSuffix(string(out), "\n")); got != c.want {
			t.Errorf("width %v, textColumns %d: line of %d, want %d: %q", c.width, c.cols, got, c.want, out)
		}
	}
}

func TestReceiptAsEscpos(t *testing.T) {
	tpl, data := readyMade(t, "receipt")
	out, err := Render(tpl, data, "escpos")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{
		"reset and code page":    {0x1b, '@', 0x1b, 't', 16},
		"double height on TOTAL": append([]byte{0x1d, '!', 1, 0x1b, 'E', 1}, "TOTAL"...),
		"Code 128 of the number": append([]byte{0x1d, 'k', 73, 19}, "{BTRX-20261014-0031"...),
	} {
		if !bytes.Contains(out, want) {
			t.Errorf("%s: no % x", name, want)
		}
	}
	if !bytes.HasSuffix(out, []byte{0x1d, 'V', 66, 3}) {
		t.Error("does not end with feed and cut")
	}
	if !bytes.HasPrefix(out, []byte{0x1b, '@'}) {
		t.Error("does not start with a reset")
	}
}

func TestEscposPicturesAndCharacters(t *testing.T) {
	black := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNgAAAAAgABSK+kcQAAAABJRU5ErkJggg=="
	tpl := []byte(`{"page": {"width": 164, "height": 400, "margin": 0}, "bands": {"summary": {"height": 120, "elements": [
		{"type": "text", "x": 0, "y": 0, "w": 164, "h": 10, "text": "Café € 5"},
		{"type": "image", "x": 41, "y": 12, "w": 82, "h": 40, "src": "` + black + `"},
		{"type": "barcode", "x": 50, "y": 60, "w": 64, "h": 64, "text": "{{id}}"},
		{"type": "text", "x": 0, "y": 100, "w": 164, "h": 10, "text": "hidden", "showIf": "id = 'x'"}]}}}`)
	out, err := Render(tpl, map[string]any{"id": "INV-7", "rows": []any{}}, "escpos")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("Caf\xe9 \x80 5")) {
		t.Error("é and € not in Windows-1252")
	}
	// the picture: centred, 192 dots (half of 384) = 24 bytes a row, all black
	if i := bytes.Index(out, []byte{0x1b, 'a', 1, 0x1d, 'v', '0', 0, 24, 0}); i < 0 || out[i+11] != 0xff {
		t.Errorf("no centred raster picture of 24 bytes a row, black: % x", out)
	}
	if !bytes.Contains(out, append([]byte{0x1d, '(', 'k', 8, 0, 49, 80, 48}, "INV-7"...)) {
		t.Error("no QR code holding INV-7")
	}
	if bytes.Contains(out, []byte("hidden")) {
		t.Error("a text whose showIf is false was printed")
	}
}

func TestWrap(t *testing.T) {
	for _, c := range []struct {
		text  string
		width int
		want  []string
	}{
		{"", 10, nil},
		{"satu dua tiga", 8, []string{"satu dua", "tiga"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"a\nb", 5, []string{"a", "b"}},
	} {
		if got := wrap(c.text, c.width); !slices.Equal(got, c.want) {
			t.Errorf("wrap(%q, %d) = %q, want %q", c.text, c.width, got, c.want)
		}
	}
}

// every ready-made template prints as text and for receipt printers
func TestTemplatesAsText(t *testing.T) {
	entries, _ := os.ReadDir("designer/templates")
	for _, f := range entries {
		id, ok := strings.CutSuffix(f.Name(), ".json")
		if !ok || id == "index" {
			continue
		}
		tpl, data := readyMade(t, id)
		for _, format := range []string{"txt", "escpos"} {
			if out, err := Render(tpl, data, format); err != nil || len(out) < 20 {
				t.Errorf("%s as %s: %d bytes, %v", id, format, len(out), err)
			}
		}
		if out, _ := Render(tpl, data, "txt"); bytes.Contains(out, []byte("{{")) {
			t.Errorf("%s as txt: placeholder left: %s", id, out)
		}
	}
}

// charts go to XLSX as charts Excel draws, with their data, beside each other as designed
func TestXLSXCharts(t *testing.T) {
	tpl, data := readyMade(t, "sales-report")
	out, err := Render(tpl, data, "xlsx")
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range z.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		files[f.Name] = string(b)
	}
	if c := files["xl/charts/chart1.xml"]; !strings.Contains(c, "<c:doughnutChart>") || !strings.Contains(c, "<c:v>Minuman</c:v>") || !strings.Contains(c, "<c:v>1296000</c:v>") {
		t.Errorf("chart1 is not the donut of sales per category: %.300s", c)
	}
	if c := files["xl/charts/chart2.xml"]; !strings.Contains(c, `<c:barDir val="col"/>`) {
		t.Errorf("chart2 is not a column chart: %.300s", c)
	}
	if !strings.Contains(files["[Content_Types].xml"], `/xl/charts/chart2.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"`) {
		t.Error("no content type for the charts")
	}
	// every chart the drawing names is one of its relationships
	drawing, rels := files["xl/drawings/drawing1.xml"], files["xl/drawings/_rels/drawing1.xml.rels"]
	for _, m := range regexp.MustCompile(`c:chart [^>]*r:id="(rId\d+)"`).FindAllStringSubmatch(drawing, -1) {
		if !strings.Contains(rels, `Id="`+m[1]+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"`) {
			t.Errorf("drawing names %s, which is not a chart relationship: %s", m[1], rels)
		}
	}
	// the second chart is 270pt in, inside the first column: it keeps its distance
	offs := regexp.MustCompile(`<xdr:colOff>(\d+)</xdr:colOff>`).FindAllStringSubmatch(drawing, -1)
	if len(offs) != 2 || offs[0][1] != "0" || offs[1][1] == "0" {
		t.Errorf("chart offsets %v, want 0 and the second one's", offs)
	}
}

// a chart under another one in a band without texts gets a row of its own, not the same spot
func TestXLSXChartsStacked(t *testing.T) {
	tpl := []byte(`{"bands": {"detail": {"height": 14, "elements": [{"type": "text", "x": 0, "y": 0, "w": 100, "h": 12, "text": "{{k}}"}]},
	  "summary": {"height": 300, "elements": [
	    {"type": "chart", "chart": "line", "label": "k", "value": "v", "x": 0, "y": 10, "w": 250, "h": 120},
	    {"type": "chart", "chart": "hbar", "label": "k", "value": "v", "x": 270, "y": 10, "w": 250, "h": 120},
	    {"type": "chart", "chart": "pie", "label": "k", "value": "v", "x": 0, "y": 150, "w": 250, "h": 120}]}}}`)
	out, err := Render(tpl, []any{map[string]any{"k": "A", "v": 1}, map[string]any{"k": "B", "v": 2}}, "xlsx")
	if err != nil {
		t.Fatal(err)
	}
	z, _ := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	var drawing string
	for _, f := range z.File {
		if f.Name == "xl/drawings/drawing1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			drawing = string(b)
		}
	}
	var rows []string
	for _, m := range regexp.MustCompile(`<xdr:row>(\d+)</xdr:row>`).FindAllStringSubmatch(drawing, -1) {
		rows = append(rows, m[1])
	}
	// two rows of data, then the summary: line and hbar side by side on row 2, the pie under them
	if strings.Join(rows, " ") != "2 2 3" {
		t.Errorf("chart rows %v, want 2 2 3", rows)
	}
}

// a table under a data table that grew moves down with it, as other elements do:
// the report card's attendance comes after the last subject, in text and in CSV alike
func TestTableUnderGrownTable(t *testing.T) {
	tpl, data := readyMade(t, "rapor")
	for _, format := range []string{"txt", "csv"} {
		out, err := Render(tpl, data, format)
		if err != nil {
			t.Fatal(err)
		}
		last, attendance := bytes.Index(out, []byte("Bahasa Inggris")), bytes.Index(out, []byte("Ketidakhadiran"))
		if last < 0 || attendance < last {
			t.Errorf("%s: attendance at %d, before the last subject at %d:\n%s", format, attendance, last, out)
		}
	}
}
