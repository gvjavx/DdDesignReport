package ddreport

import (
	"fmt"
	"strings"
	"testing"
)

// a sub-report of an order's lines, grouped by kind, with its own header, column header and total
const linesReport = `{"type":"subreport","x":10,"y":14,"w":280,"h":40,"data":"lines","report":{"groupBy":"kind","sortBy":"kind","bands":{
  "header":{"height":14,"elements":[{"type":"text","w":200,"h":12,"text":"Lines of {{number}}"}]},
  "columnHeader":{"height":14,"elements":[{"type":"text","w":100,"h":12,"text":"Item"},{"type":"text","x":150,"w":80,"h":12,"text":"Total"}]},
  "groupHeader":{"height":14,"elements":[{"type":"text","w":100,"h":12,"text":"== {{kind}}"}]},
  "detail":{"height":14,"elements":[{"type":"text","w":100,"h":12,"text":"{{index}}. {{name}}"},{"type":"text","x":150,"w":80,"h":12,"text":"{{qty * price:0}}"}]},
  "groupFooter":{"height":14,"elements":[{"type":"text","x":150,"w":80,"h":12,"text":"{{sum(qty * price):0}}"}]},
  "summary":{"height":14,"elements":[{"type":"text","x":150,"w":80,"h":12,"text":"total {{sum(qty * price):0}}"}]}}}}`

func orders(n int) []any {
	var out []any
	for o := 1; o <= 2; o++ {
		var lines []any
		for i := range n {
			kind := "food"
			if i%2 == 1 {
				kind = "drink"
			}
			lines = append(lines, map[string]any{"kind": kind, "name": fmt.Sprint("item ", o, ".", i+1), "qty": 1.0, "price": float64(1000 * (i + 1))})
		}
		out = append(out, map[string]any{"number": fmt.Sprint("SO-", o), "lines": lines})
	}
	return out
}

func TestSubreport(t *testing.T) {
	tpl := `{"bands":{"detail":{"height":70,"elements":[{"type":"text","w":200,"h":12,"text":"Order {{number}}"},` + linesReport + `,
	  {"type":"text","y":56,"w":200,"h":12,"text":"end of {{number}}"}]}}}`
	got := texts(t, tpl, orders(3))
	want := "Order SO-1 | Lines of SO-1 | Item | Total | == drink | 1. item 1.2 | 2,000 | 2,000 | == food | 1. item 1.1 | 1,000 | 2. item 1.3 | 3,000 | 4,000 | total 6,000 | end of SO-1 | " +
		"Order SO-2 | Lines of SO-2 | Item | Total | == drink | 1. item 2.2 | 2,000 | 2,000 | == food | 1. item 2.1 | 1,000 | 2. item 2.3 | 3,000 | 4,000 | total 6,000 | end of SO-2"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	// the band grows with the sub-report: what is under it moves down
	r := load(t, tpl, orders(3))
	r.initPDF()
	els, h := r.place(r.t.Bands.Detail, scope{r.t, r.rows, []map[string]any{r.rows[0].(map[string]any)}})
	last := els[len(els)-1]
	if last.Text != "end of SO-1" || last.Y != 56+10*14-40 || h != 70+10*14-40 {
		t.Errorf("end at %v, band %v", last.Y, h)
	}
	// in a spreadsheet every line of the sub-report is a line of the sheet
	csv := string(render(t, []byte(tpl), orders(3), "csv"))
	// "total 6,000" is text with a number in it, so it stays as written; the text under the sub-report comes after it
	if !strings.Contains(csv, ",1. item 1.2,2000\n") || !strings.Contains(csv, ",,\"total 6,000\"\nend of SO-1,,\n") || !strings.Contains(csv, "Lines of SO-2") {
		t.Errorf("csv:\n%s", csv)
	}
	// a picture inside a sub-report is checked like the others
	bad := strings.Replace(tpl, `"text":"Lines of {{number}}"}`, `"text":"x"},{"type":"image","w":10,"h":10,"src":"https://x/a.png"}`, 1)
	if _, err := Render([]byte(bad), orders(1), "pdf"); err == nil {
		t.Error("an image link inside a sub-report was accepted")
	}
}

func TestLongSubreportOverPages(t *testing.T) {
	tpl := `{"page":{"width":300,"height":200,"margin":10},"bands":{"detail":{"height":60,"elements":[` + linesReport + `]}}}`
	r := load(t, tpl, orders(30)[:1])
	r.initPDF()
	pages := r.paginate(0)
	if len(pages) < 3 {
		t.Fatalf("%d pages", len(pages))
	}
	count := map[string]int{}
	for i, page := range pages {
		var first string
		for _, e := range page {
			if e.Type != "text" {
				continue
			}
			count[e.Text]++
			if first == "" {
				first = e.Text
			}
			if e.Y+e.H > 190.01 {
				t.Errorf("page %d: %q below the margin", i+1, e.Text)
			}
		}
		if first != "Lines of SO-1" { // its header and column header come again on every page
			t.Errorf("page %d starts with %q", i+1, first)
		}
	}
	if count["Item"] != len(pages) || count["total 465,000"] != 1 || count["15. item 1.29"] != 1 {
		t.Errorf("column header %d times on %d pages, total %d, last line %d", count["Item"], len(pages), count["total 465,000"], count["15. item 1.29"])
	}
}
