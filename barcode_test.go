package ddreport

import (
	"strings"
	"testing"
)

func TestBars(t *testing.T) {
	out := bars(Element{X: 10, Y: 20, W: 58, H: 100, Text: "INV-001"})
	if len(out) != 1 || out[0].Type != "code" || out[0].X != 10 || out[0].Y != 20 || out[0].W != 58 || out[0].H != 100 || out[0].Color != "#000000" {
		t.Fatalf("QR code is %d elements, want one \"code\" element in the barcode's box", len(out))
	}
	runs := out[0].Points
	if len(runs) == 0 || len(runs)%4 != 0 {
		t.Fatalf("%d numbers, want x,y,w,h per dark run", len(runs))
	}
	// version 1 is 21 modules wide (no quiet zone is added), fitted into the smaller side
	const module = 58.0 / 21
	// the top edge of the top-left finder pattern: 7 dark modules
	if runs[0] != 0 || runs[1] != 0 || runs[2] != 7*module || runs[3] != module {
		t.Errorf("first run %v", runs[:4])
	}
	for i := 0; i < len(runs); i += 4 {
		if runs[i] < 0 || runs[i]+runs[i+2] > 58+1e-9 || runs[i+1] < 0 || runs[i+1]+runs[i+3] > 58+1e-9 {
			t.Fatalf("run outside the square: %v", runs[i:i+4])
		}
	}
	other := bars(Element{W: 58, H: 58, Text: "INV-002"})[0].Points
	if len(other) == len(runs) && other[len(other)/2] == runs[len(runs)/2] && other[len(other)/2+1] == runs[len(runs)/2+1] {
		t.Error("different text gave the same code")
	}

	lines := bars(Element{X: 5, Y: 5, W: 200, H: 30, Text: "TRX-0031", Format: "code128", Color: "#112233"})
	if len(lines) != 1 || lines[0].Color != "#112233" || len(lines[0].Points) < 80 {
		t.Fatalf("code 128: %d elements", len(lines))
	}
	for p, i := lines[0].Points, 0; i < len(p); i += 4 {
		if p[i+1] != 0 || p[i+3] != 30 || p[i] < 0 || p[i]+p[i+2] > 200+1e-9 {
			t.Fatalf("bar %v", p[i:i+4])
		}
	}

	for name, e := range map[string]Element{
		"empty":             {W: 50, H: 50},
		"empty code 128":    {W: 50, H: 50, Format: "code128"},
		"non-ASCII code128": {W: 50, H: 50, Format: "code128", Text: "kopi ☕"},
	} {
		if got := bars(e); got != nil {
			t.Errorf("%s: drew %d elements", name, len(got))
		}
	}
}

func TestBarcodeInAReport(t *testing.T) {
	tpl := []byte(`{"bands":{"detail":{"height":80,"elements":[
	  {"type":"barcode","x":0,"y":0,"w":60,"h":60,"text":"https://example.com/{{sku}}"},
	  {"type":"barcode","x":80,"y":0,"w":120,"h":30,"text":"{{sku}}","format":"code128","color":"#aa0000"},
	  {"type":"text","x":80,"y":40,"w":120,"h":12,"text":"{{sku}}"}]}}}`)
	data := []map[string]any{{"sku": "A-1"}, {"sku": "B-2"}}
	html := string(render(t, tpl, data, "html"))
	// one compact drawing per code, not one element per module
	if n := strings.Count(html, "<svg"); n != 4 {
		t.Errorf("html has %d drawings, want 4", n)
	}
	if !strings.Contains(html, `<path fill="#000000" d="M0 0h`) || !strings.Contains(html, `<path fill="#aa0000" d="M`) {
		t.Error("html: barcode paths missing")
	}
	if strings.Contains(html, "{{") {
		t.Error("barcode text was not resolved")
	}
	if pdf := render(t, tpl, data, "pdf"); !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Error("not a PDF")
	}
	// codes are pictures: spreadsheets only get the text column
	if csv := string(render(t, tpl, data, "csv")); csv != "A-1\nB-2\n" {
		t.Errorf("csv = %q", csv)
	}
}
