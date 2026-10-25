package ddreport

import (
	"fmt"
	"strings"
	"testing"
)

// an order with its items, as an invoice or a sales order has them
var order = map[string]any{
	"number": "SO-1",
	"items": []any{
		map[string]any{"name": "Kopi", "qty": 2.0, "price": "18000"},
		map[string]any{"name": "Teh", "qty": 1.0, "price": 12000.0},
		map[string]any{"name": "Gula", "qty": 3.0, "price": 5000.0},
	},
}

const itemsTable = `{"type":"table","w":300,"h":60,"border":1,"header":true,"footer":true,"data":"items","stripe":"#eeeeee",
  "cells":[["No","Item","Total"],["{{index}}","{{name}}","{{qty * price:0}}"],["","{{count()}} items","{{sum(qty * price):0}}"]]}`

func TestDataTable(t *testing.T) {
	tpl := `{"bands":{"summary":{"height":80,"elements":[` + itemsTable + `,
	  {"type":"text","y":62,"w":200,"h":12,"text":"after the table"}]}}}`
	got := texts(t, tpl, map[string]any{"rows": []any{}, "items": order["items"]})
	want := "No | Item | Total | 1 | Kopi | 36,000 | 2 | Teh | 12,000 | 3 | Gula | 15,000 |  | 3 items | 63,000 | after the table"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	// the table has a row per item, and what is under it moves down by the extra rows
	r := load(t, tpl, map[string]any{"rows": []any{}, "items": order["items"]})
	r.initPDF()
	els, h := r.place(r.t.Bands.Summary, scope{r.t, r.rows, []map[string]any{r.root}})
	last := els[len(els)-1]
	if last.Text != "after the table" || last.Y != 62+40 || h != 80+40 {
		t.Errorf("text under the table at %v, band %v", last.Y, h)
	}
	// banded rows go by item
	stripes := 0
	for _, e := range els {
		if e.Type == "rect" && e.Fill == "#eeeeee" {
			stripes++
		}
	}
	if stripes != 3 { // the second item's three cells
		t.Errorf("%d striped cells", stripes)
	}
}

func TestDataTableInEveryRow(t *testing.T) {
	// a list inside each row: every order with its own items
	tpl := `{"bands":{"detail":{"height":80,"elements":[{"type":"text","w":200,"h":12,"text":"Order {{number}}"},
	  ` + strings.Replace(itemsTable, `"w":300`, `"y":14,"w":300`, 1) + `]}}}`
	other := map[string]any{"number": "SO-2", "items": []any{map[string]any{"name": "Susu", "qty": 1.0, "price": 9000.0}}}
	got := texts(t, tpl, []any{order, other})
	if !strings.Contains(got, "Order SO-1 | No | Item | Total | 1 | Kopi") || !strings.Contains(got, "63,000 | Order SO-2 | No | Item | Total | 1 | Susu | 9,000 |  | 1 items | 9,000") {
		t.Errorf("got %q", got)
	}
	// in a spreadsheet every item is a line of its own
	csv := string(render(t, []byte(tpl), []any{order, other}, "csv"))
	if !strings.Contains(csv, "Order SO-1,,,\n,No,Item,Total\n,1,Kopi,36000\n,2,Teh,12000\n,3,Gula,15000\n,,3 items,63000\n") || !strings.Contains(csv, ",1,Susu,9000\n") {
		t.Errorf("csv:\n%s", csv)
	}
	// an order without items still prints its header and total
	if got := texts(t, tpl, []any{map[string]any{"number": "SO-3"}}); got != "Order SO-3 | No | Item | Total |  | 0 items | 0" {
		t.Errorf("empty list: %q", got)
	}
}

func TestMergedCellsAndCellStyles(t *testing.T) {
	yes := true
	table := Element{Type: "table", W: 300, H: 60, Border: 1, Cells: [][]string{{"Title", "", ""}, {"a", "b", "c"}, {"d", "e", "f"}},
		CellStyle: map[string]CellStyle{
			"0,0": {ColSpan: 3, Fill: "#ff0000", Color: "#ffffff", Align: "center", Bold: &yes},
			"1,0": {RowSpan: 2},
		}}
	b := expand(Band{Elements: []Element{table}})
	cells := map[string]Element{}
	for _, e := range b.Elements {
		if e.Type == "text" {
			cells[e.Text] = e
		}
	}
	if len(cells) != 6 || cells[""].Type != "" { // the covered cells are gone
		t.Errorf("cells %v", len(cells))
	}
	title, a, b2 := cells["Title"], cells["a"], cells["b"]
	if title.W != 294 || title.Align != "center" || !title.Bold || title.Color != "#ffffff" || title.bg != "#ff0000" {
		t.Errorf("title %+v", title)
	}
	if a.H != 36 || a.Y != 22 || b2.X != 103 { // two rows of 20, 2pt padding
		t.Errorf("a %v@%v, b at x %v", a.H, a.Y, b2.X)
	}
	// no line crosses a merged cell: nothing vertical inside the title row, no line between a's rows
	for _, e := range b.Elements {
		if e.Type == "line" && e.H > e.W && e.Y == 0 && e.X > 0 && e.X < 300 {
			t.Errorf("a line crosses the title at x %v", e.X)
		}
		if e.Type == "line" && e.H <= e.W && e.Y == 40 && e.X == 0 {
			t.Errorf("a line crosses the merged cell a")
		}
	}
	// a span past the edge of the table is kept inside it
	table.CellStyle = map[string]CellStyle{"1,2": {ColSpan: 5, RowSpan: 9}}
	for _, e := range expand(Band{Elements: []Element{table}}).Elements {
		if e.Type == "text" && e.Text == "c" && (e.X+e.W > 300 || e.Y+e.H > 60) {
			t.Errorf("c reaches %v, %v", e.X+e.W, e.Y+e.H)
		}
	}
}

func TestRowHeightsPaddingAndGrowingRows(t *testing.T) {
	pad := 0.0
	b := expand(Band{Elements: []Element{{Type: "table", W: 100, H: 60, Heights: []float64{2, 1}, Padding: &pad, Cells: [][]string{{"a"}, {"b"}}}}})
	if a, bb := b.Elements[0], b.Elements[1]; a.H != 40 || bb.Y != 40 || bb.H != 20 || a.X != 0 {
		t.Errorf("rows %v and %v@%v, x %v", a.H, bb.H, bb.Y, a.X)
	}
	// a row that grows grows as one: its short cell gets the same height, so it can be centred
	r := load(t, `{"bands":{"detail":{"height":40,"elements":[{"type":"table","w":200,"h":20,"grow":true,"valign":"middle",
	  "cells":[["short","a long text that wraps over several lines in this cell"]]}]}}}`, []any{1})
	r.initPDF()
	els, _ := r.place(r.t.Bands.Detail, scope{r.t, r.rows, nil})
	if els[0].H != els[1].H || els[0].H <= 16 {
		t.Errorf("cells %v and %v high", els[0].H, els[1].H)
	}
}

func TestLongTableOverPages(t *testing.T) {
	var items []any
	for i := range 40 {
		items = append(items, map[string]any{"name": fmt.Sprint("item ", i), "qty": float64(i)})
	}
	tpl := `{"page":{"width":300,"height":300,"margin":20},"bands":{"summary":{"height":60,"elements":[
	  {"type":"table","w":200,"h":60,"border":1,"header":true,"footer":true,"data":"items",
	   "cells":[["Item","Qty"],["{{name}}","{{qty}}"],["Total","{{sum(qty)}}"]]}]}}}`
	r := load(t, tpl, map[string]any{"items": items})
	r.initPDF()
	pages := r.paginate(0)
	if len(pages) < 3 {
		t.Fatalf("%d pages", len(pages))
	}
	seen := map[string]int{}
	for i, page := range pages {
		var first string
		for _, e := range page {
			if e.Type == "text" {
				seen[e.Text]++
				if first == "" {
					first = e.Text
				}
			}
			// no row is cut: every cell is wholly on the page
			if e.Type == "text" && e.Y+e.H > 280.01 {
				t.Errorf("page %d: %q ends at %v, below the margin", i+1, e.Text, e.Y+e.H)
			}
		}
		if first != "Item" {
			t.Errorf("page %d starts with %q, not the header", i+1, first)
		}
	}
	if seen["Item"] != len(pages) || seen["Total"] != 1 || seen["item 39"] != 1 || seen["780"] != 1 {
		t.Errorf("header %d times on %d pages, total %d, last item %d, sum %d", seen["Item"], len(pages), seen["Total"], seen["item 39"], seen["780"])
	}
	// a static table taller than its band is not cut off either
	rows := `["H"]`
	for i := range 40 {
		rows += fmt.Sprintf(`,["r%d"]`, i)
	}
	big := `{"page":{"width":300,"height":300,"margin":20},"bands":{"summary":{"height":20,"elements":[{"type":"table","w":200,"h":820,"header":true,"cells":[` + rows + `]}]}}}`
	html := string(render(t, []byte(big), nil, "html"))
	if !strings.Contains(html, ">r39<") || strings.Count(html, `<div class="p">`) < 3 || strings.Count(html, ">H<") != strings.Count(html, `<div class="p">`) {
		t.Errorf("static long table: %d pages, header %d times, last row %v", strings.Count(html, `<div class="p">`), strings.Count(html, ">H<"), strings.Contains(html, ">r39<"))
	}
}

func TestTableColoursInSpreadsheets(t *testing.T) {
	tpl := []byte(`{"bands":{"detail":{"height":30,"elements":[{"type":"table","w":200,"h":20,"header":true,"headerFill":"#1e3a8a","headerColor":"#ffffff",
	  "cells":[["H1","H2"],["{{a}}","{{b:2}}"]],"cellStyle":{"1,1":{"fill":"#ffff00"}}}]}}}`)
	rows := []any{map[string]any{"a": "x", "b": 2.0}}
	x := sheet(t, render(t, tpl, rows, "xlsx"))
	if !strings.Contains(x, `<c s="28" t="inlineStr"><is><t xml:space="preserve">H1<`) || !strings.Contains(x, `<c s="29"><v>2</v></c>`) {
		t.Errorf("xlsx sheet: %s", x)
	}
	styles := string(unzip(t, render(t, tpl, rows, "xlsx"), "xl/styles.xml"))
	for _, want := range []string{`<color rgb="FFFFFFFF"/>`, `<fgColor rgb="FF1E3A8A"/>`, `<fgColor rgb="FFFFFF00"/>`} {
		if !strings.Contains(styles, want) {
			t.Errorf("xlsx styles: missing %q", want)
		}
	}
	xls := string(render(t, tpl, rows, "xls"))
	for _, want := range []string{`<Style ss:ID="s28"><Font ss:Bold="1" ss:Color="#ffffff"/><Interior ss:Color="#1e3a8a" ss:Pattern="Solid"/></Style>`, `<Cell ss:StyleID="s28"><Data ss:Type="String">H1</Data>`} {
		if !strings.Contains(xls, want) {
			t.Errorf("xls: missing %q", want)
		}
	}
}

func TestHeavierTableLines(t *testing.T) {
	var items []any
	for i := range 40 {
		items = append(items, map[string]any{"n": float64(i)})
	}
	tpl := `{"page":{"width":300,"height":300,"margin":20},"bands":{"summary":{"height":60,"elements":[
	  {"type":"table","w":200,"h":60,"border":0.5,"headerLine":2,"footerLine":1.5,"frame":1,"header":true,"footer":true,"data":"items",
	   "cells":[["N"],["{{n}}"],["{{sum(n)}}"]]}]}}}`
	r := load(t, tpl, map[string]any{"items": items})
	r.initPDF()
	pages := r.paginate(0)
	for i, page := range pages {
		heavy, frames := 0, 0
		for _, e := range page {
			if e.Type == "line" && e.Border == 2 {
				heavy++
			}
			if e.Type == "rect" && e.Border == 1 {
				frames++
			}
		}
		if heavy != 1 || frames != 1 { // the line under the header comes with it, the frame is closed on every page
			t.Errorf("page %d: %d heavy header lines, %d frames", i+1, heavy, frames)
		}
	}
	last, total := pages[len(pages)-1], 0
	for _, e := range last {
		if e.Type == "line" && e.Border == 1.5 {
			total++
		}
	}
	if total != 1 {
		t.Errorf("%d lines over the total row", total)
	}
}

// A colour may be an expression: worked out per row, per item of a data table,
// and in the spreadsheets too. One that does not come out as #rrggbb is not drawn.
func TestColoursFromExpressions(t *testing.T) {
	red := `{{if(amt < 0, '#dc2626', '#000000')}}`
	tpl := []byte(`{"bands":{"detail":{"height":20,"elements":[
	  {"type":"text","w":100,"h":14,"text":"{{amt}}","color":"` + red + `","fill":"{{if(amt < 0, '#fee2e2', '')}}"},
	  {"type":"text","x":120,"w":100,"h":14,"text":"bad","color":"{{'red;}</style><script>'}}"}]}}}`)
	h := string(render(t, tpl, rowsOf(5, -3), "html"))
	if strings.Count(h, "color:#dc2626") != 1 || strings.Count(h, "background:#fee2e2") != 1 || strings.Contains(h, "<script>") {
		t.Errorf("html: %s", h)
	}
	x := string(unzip(t, render(t, tpl, rowsOf(5, -3), "xlsx"), "xl/styles.xml"))
	if !strings.Contains(x, `<color rgb="FFDC2626"/>`) || !strings.Contains(x, `<fgColor rgb="FFFEE2E2"/>`) {
		t.Errorf("xlsx styles: %s", x)
	}

	items := []byte(`{"bands":{"summary":{"height":60,"elements":[{"type":"table","w":300,"h":40,"header":true,"data":"items",
	  "textColor":"{{if(qty > 2, '#16a34a', '#000000')}}","cellStyle":{"1,0":{"fill":"{{if(name = 'Teh', '#fef9c3', '')}}"}},
	  "cells":[["Item","Qty"],["{{name}}","{{qty}}"]]}]}}}`)
	h = string(render(t, items, order, "html"))
	if strings.Count(h, "color:#16a34a") != 2 || strings.Count(h, "#fef9c3") != 1 { // Gula's two cells; Teh's first cell
		t.Errorf("data table html: %s", h)
	}
}
