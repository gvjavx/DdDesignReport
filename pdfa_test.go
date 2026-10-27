package ddreport

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"
)

// A PDF/A file has every font in it, the metadata archives ask for, an sRGB
// output intent and a document ID; the same report without it has none of that.
func TestPDFA(t *testing.T) {
	tpl := `{"pdfA":true,"bands":{"detail":{"height":40,"elements":[
	  {"type":"text","w":200,"h":14,"text":"Helvetica {{name}}"},
	  {"type":"text","y":14,"w":200,"h":14,"font":"Times","bold":true,"text":"Times"},
	  {"type":"text","y":28,"w":200,"h":14,"font":"Courier","text":"Courier"}]}}}`
	pdf := render(t, []byte(tpl), []map[string]any{{"name": "Kopi"}}, "pdf")
	for _, want := range []string{
		"<pdfaid:part>2</pdfaid:part>", "<pdfaid:conformance>B</pdfaid:conformance>",
		"/OutputIntents", "/S /GTS_PDFA1", "/Producer (DdDesignReport)", "<pdf:Producer>DdDesignReport</pdf:Producer>",
		"/FontFile2", // the fonts are in the file
	} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Errorf("missing %s", want)
		}
	}
	for _, core := range []string{"/BaseFont /Helvetica", "/BaseFont /Times", "/BaseFont /Courier"} {
		if bytes.Contains(pdf, []byte(core)) {
			t.Errorf("a font that is not in the file: %s", core)
		}
	}
	// the dates of the metadata and of the document info are the same
	info := regexp.MustCompile(`/CreationDate \(D:(\d{4})(\d\d)(\d\d)(\d\d)(\d\d)(\d\d)\)`).FindSubmatch(pdf)
	xmp := regexp.MustCompile(`<xmp:CreateDate>(\d{4})-(\d\d)-(\d\d)T(\d\d):(\d\d):(\d\d)</xmp:CreateDate>`).FindSubmatch(pdf)
	if info == nil || xmp == nil || !bytes.Equal(bytes.Join(info[1:], nil), bytes.Join(xmp[1:], nil)) {
		t.Errorf("dates differ: %q and %q", info, xmp)
	}
	// the ID is in the trailer, and the cross-reference table still points at its objects
	if !regexp.MustCompile(`trailer\n<<\n/ID \[<[0-9A-F]{32}><[0-9A-F]{32}>\]\n/Size`).Match(pdf) {
		t.Error("no document ID in the trailer")
	}
	start := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(pdf)
	if n, _ := strconv.Atoi(string(start[1])); !bytes.HasPrefix(pdf[n:], []byte("xref")) {
		t.Error("startxref does not point at the xref table")
	}
	// without pdfA: the built-in fonts, no metadata
	plain := render(t, []byte(`{"bands":{"detail":{"height":14,"elements":[{"type":"text","w":200,"h":14,"text":"x"}]}}}`), []any{1}, "pdf")
	if bytes.Contains(plain, []byte("pdfaid")) || !bytes.Contains(plain, []byte("/BaseFont /Helvetica")) {
		t.Error("PDF/A without asking for it")
	}
}
