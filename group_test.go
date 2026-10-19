package ddreport

import (
	"fmt"
	"strings"
	"testing"
)

var sales = func() []any {
	var rows []any
	for _, s := range []struct {
		region, city string
		n            int
	}{{"Jawa", "Bandung", 3}, {"Jawa", "Surabaya", 2}, {"Bali", "Denpasar", 2}} {
		for i := range s.n {
			rows = append(rows, map[string]any{"region": s.region, "city": s.city, "amt": float64(10 * (i + 1))})
		}
	}
	return rows
}()

const twoLevels = `{"groupBy":"region, city","page":{"width":300,"height":%d,"margin":10},"bands":{
  "groupHeader":{"height":14,"elements":[{"type":"text","w":200,"h":12,"text":"R {{region}}"}]},
  "groupHeader2":{"height":14,"elements":[{"type":"text","w":200,"h":12,"text":"C {{city}}"}]},
  "detail":{"height":14,"elements":[{"type":"text","w":100,"h":12,"text":"{{amt}}"}]},
  "groupFooter2":{"height":14,"elements":[{"type":"text","w":200,"h":12,"text":"city {{sum(amt)}}"}]},
  "groupFooter":{"height":14,"elements":[{"type":"text","w":200,"h":12,"text":"region {{sum(amt)}}"}]},
  "summary":{"height":14,"elements":[{"type":"text","w":200,"h":12,"text":"all {{sum(amt)}}"}]}}}`

func TestTwoGroupLevels(t *testing.T) {
	got := texts(t, fmt.Sprintf(twoLevels, 800), sales)
	want := "R Jawa | C Bandung | 10 | 20 | 30 | city 60 | C Surabaya | 10 | 20 | city 30 | region 90 | R Bali | C Denpasar | 10 | 20 | city 30 | region 30 | all 120"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	// on a short page both headers of the groups under way come again at the top
	r := load(t, fmt.Sprintf(twoLevels, 110), sales)
	r.initPDF()
	for i, page := range r.paginate(0)[1:] {
		var first []string
		for _, e := range page {
			if e.Type == "text" && len(first) < 2 {
				first = append(first, e.Text)
			}
		}
		// every page inside a region starts with its header; one with only the grand total has none
		if !strings.HasPrefix(first[0], "R ") && first[0] != "all 120" {
			t.Errorf("page %d starts with %q", i+2, first)
		}
		if strings.HasPrefix(first[0], "R ") && len(first) > 1 && strings.HasPrefix(first[1], "C ") == strings.HasPrefix(first[1], "region") {
			t.Errorf("page %d: the city header is missing in %q", i+2, first)
		}
	}
	csv := string(render(t, []byte(fmt.Sprintf(twoLevels, 800)), sales, "csv"))
	if !strings.HasPrefix(csv, "R Jawa\nC Bandung\n10\n20\n30\ncity 60\nC Surabaya\n") || !strings.HasSuffix(csv, "region 30\nall 120\n") {
		t.Errorf("csv:\n%s", csv)
	}
}

func TestSheetPerGroup(t *testing.T) {
	tpl := []byte(`{"groupBy":"region","sheetPerGroup":true,"bands":{
	  "columnHeader":{"height":14,"elements":[{"type":"text","w":100,"h":12,"text":"Kota"},{"type":"text","x":110,"w":60,"h":12,"text":"Jumlah"}]},
	  "detail":{"height":14,"elements":[{"type":"text","w":100,"h":12,"text":"{{city}}"},{"type":"text","x":110,"w":60,"h":12,"text":"{{amt}}"}]},
	  "summary":{"height":14,"elements":[{"type":"text","x":110,"w":60,"h":12,"text":"{{sum(amt)}}"}]}}}`)
	rows := append(append([]any{}, sales...),
		map[string]any{"region": "Jawa/Bali: [lama]", "city": "x", "amt": 1.0},
		map[string]any{"region": "jawa", "city": "y", "amt": 2.0}) // Excel names ignore case: "jawa" clashes with "Jawa"
	x := render(t, tpl, rows, "xlsx")
	book := string(unzip(t, x, "xl/workbook.xml"))
	for _, want := range []string{`<sheet name="Jawa" sheetId="1"`, `<sheet name="Bali" sheetId="2"`, `<sheet name="Jawa Bali   lama" sheetId="3"`, `<sheet name="jawa (2)" sheetId="4"`} {
		if !strings.Contains(book, want) {
			t.Errorf("workbook: missing %q in %s", want, book)
		}
	}
	bali := string(unzip(t, x, "xl/worksheets/sheet2.xml"))
	if !strings.Contains(bali, ">Kota<") || !strings.Contains(bali, ">Denpasar<") || strings.Contains(bali, "Bandung") || !strings.Contains(bali, "<v>30</v>") {
		t.Errorf("the Bali sheet: %s", bali)
	}
	xls := string(render(t, tpl, rows, "xls"))
	if strings.Count(xls, "<Worksheet ") != 4 || !strings.Contains(xls, `<Worksheet ss:Name="Bali">`) {
		t.Errorf("xls has %d sheets", strings.Count(xls, "<Worksheet "))
	}
	// csv and pdf are one report as before
	if !strings.Contains(string(render(t, tpl, rows, "csv")), "Bandung") {
		t.Error("csv lost rows")
	}
}
