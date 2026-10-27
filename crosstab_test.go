package ddreport

import (
	"strings"
	"testing"
)

var crossRows = []map[string]any{
	{"region": "Jawa", "month": "Jan", "qty": 2, "price": 10},
	{"region": "Jawa", "month": "Feb", "qty": 1, "price": 10},
	{"region": "Bali", "month": "Jan", "qty": 3, "price": 5},
	{"region": "Jawa", "month": "Jan", "qty": 1, "price": 4},
}

func TestCrosstab(t *testing.T) {
	tpl := `{"bands":{"summary":{"height":60,"elements":[{"type":"crosstab","w":300,"h":45,"border":1,
	  "text":"Region","rowField":"region","columnField":"month","measure":"sum(qty * price)","totalLabel":"Jumlah"}]}}}`
	// rows and columns in the order they first come, the cells summed, totals across and down
	want := "Region,Jan,Feb,Jumlah\nJawa,24,10,34\nBali,15,,15\nJumlah,39,10,49\n"
	if csv := string(render(t, []byte(tpl), crossRows, "csv")); csv != want {
		t.Errorf("csv:\n%s\nwant:\n%s", csv, want)
	}
	// in the PDF and HTML too, a row each as tall as a third of the box, and the band grows to hold them
	html := string(render(t, []byte(tpl), crossRows, "html"))
	for _, s := range []string{">Region<", ">Bali<", ">49<", ">Jumlah<"} {
		if !strings.Contains(html, s) {
			t.Errorf("html: missing %s", s)
		}
	}
	r := load(t, tpl, crossRows)
	r.initPDF()
	if _, h := r.place(r.t.Bands.Summary, scope{r.t, r.rows, []map[string]any{r.root}}); h < 4*15 {
		t.Errorf("band height %g, want room for four rows of 15pt", h)
	}
	// count() is the default, and a format applies to every cell
	count := strings.Replace(tpl, `"measure":"sum(qty * price)",`, ``, 1)
	if csv := string(render(t, []byte(count), crossRows, "csv")); !strings.HasPrefix(csv, "Region,Jan,Feb,Jumlah\nJawa,2,1,3\n") {
		t.Errorf("count: %s", csv)
	}
	decimals := strings.Replace(tpl, `sum(qty * price)`, `avg(price):1`, 1)
	if csv := string(render(t, []byte(decimals), crossRows, "csv")); !strings.Contains(csv, "Jawa,7.0,10.0,8.0") {
		t.Errorf("avg with a format: %s", csv)
	}
	// a value from the data is text: no placeholder in it is filled in, no markup applied
	evil := []map[string]any{{"region": "{{now}}<b>x</b>", "month": "Jan", "qty": 1, "price": 1}}
	if csv := string(render(t, []byte(tpl), evil, "csv")); !strings.Contains(csv, "{ {now}}‹b›x‹/b›") {
		t.Errorf("data as text: %s", csv)
	}
	// per group, in a group footer: the rows of that group only
	grouped := `{"groupBy":"region","bands":{"groupFooter":{"height":45,"elements":[{"type":"crosstab","w":300,"h":45,
	  "rowField":"month","columnField":"region","measure":"sum(qty)"}]}}}`
	if csv := string(render(t, []byte(grouped), crossRows, "csv")); !strings.Contains(csv, "Jan,3,3\nFeb,1,1\nTotal,4,4") || !strings.Contains(csv, "Jan,3,3\nTotal,3,3") {
		t.Errorf("per group: %s", csv)
	}
	// from a list in the data
	nested := `{"bands":{"detail":{"height":45,"elements":[{"type":"crosstab","data":"lines","w":300,"h":45,"rowField":"a","columnField":"b"}]}}}`
	if csv := string(render(t, []byte(nested), []map[string]any{{"lines": []map[string]any{{"a": "x", "b": "y"}, {"a": "x", "b": "y"}}}}, "csv")); !strings.Contains(csv, "x,2,2") {
		t.Errorf("from data: %s", csv)
	}
	// no rows: just the header and total row, nothing breaks
	if csv := string(render(t, []byte(tpl), []map[string]any{}, "csv")); csv != "Region,Jumlah\nJumlah,\n" {
		t.Errorf("empty: %q", csv)
	}
}
