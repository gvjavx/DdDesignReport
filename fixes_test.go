package ddreport

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// A band for the last page prints when the template names "Pages" in capitals,
// as json.Marshal of a Template writes it.
func TestLastPageBandCapitalised(t *testing.T) {
	tpl := []byte(`{"Bands":{"Detail":{"Height":14,"Elements":[{"Type":"text","W":100,"H":14,"Text":"row"}]},
	  "Footer":{"Height":14,"Pages":"last","Elements":[{"Type":"text","W":100,"H":14,"Text":"END"}]}}}`)
	if html := string(render(t, tpl, []any{1}, "html")); !strings.Contains(html, ">END<") {
		t.Error("the last-page footer is gone")
	}
}

// PDF/A: a font that is not there prints with an embedded one, not Helvetica.
func TestPDFAUnknownFont(t *testing.T) {
	tpl := []byte(`{"pdfA":true,"bands":{"detail":{"height":14,"elements":[{"type":"text","w":100,"h":14,"font":"No Such Font","text":"x"}]}}}`)
	if pdf := render(t, tpl, []any{1}, "pdf"); bytes.Contains(pdf, []byte("/BaseFont /Helvetica")) {
		t.Error("PDF/A with a font that is not in the file")
	}
}

// Right to left: a run of left-to-right words inside keeps its order.
func TestBidiOrder(t *testing.T) {
	var spans []span
	for _, s := range []string{"فاتورة", " ", "Order", " ", "123", " ", "رقم"} {
		spans = append(spans, span{run: run{text: s}})
	}
	var got []string
	for _, s := range bidiOrder(spans) {
		got = append(got, s.text)
	}
	if want := "رقم| |Order| |123| |فاتورة"; strings.Join(got, "|") != want {
		t.Errorf("got %q, want %q", strings.Join(got, "|"), want)
	}
}

// a folder whose font files cannot be opened
type unreadable struct{ fstest.MapFS }

func (u unreadable) Open(name string) (fs.File, error) {
	if strings.HasSuffix(name, ".ttf") {
		return nil, fs.ErrPermission
	}
	return u.MapFS.Open(name)
}

func TestAddFontsUnreadable(t *testing.T) {
	if err := AddFonts(unreadable{fstest.MapFS{"Broken-Regular.ttf": {Data: []byte("x")}}}); err == nil {
		t.Error("a font file that cannot be read is not said")
	}
	delete(Families, "Broken")
}

func TestFromJRXMLWhatHasNoCounterpart(t *testing.T) {
	groups := `<group name="x"><groupExpression><![CDATA[$F{a} + 1]]></groupExpression></group>`
	for _, f := range []string{"f1", "f2", "f3", "f4", "f5", "f6"} {
		groups += `<group name="` + f + `"><groupExpression><![CDATA[$F{` + f + `}]]></groupExpression></group>`
	}
	jr := `<jasperReport xmlns="http://jasperreports.sourceforge.net/jasperreports" pageWidth="595" pageHeight="842">` + groups + `
	  <detail><band height="40">
	    <textField><reportElement x="0" y="0" width="100" height="20"><printWhenExpression><![CDATA[$V{PAGE_NUMBER} == 1]]></printWhenExpression></reportElement><textFieldExpression><![CDATA[$F{name}.toUpperCase()]]></textFieldExpression></textField>
	  </band></detail></jasperReport>`
	b, notes, err := FromJRXML([]byte(jr))
	if err != nil {
		t.Fatal(err)
	}
	s, all := string(b), strings.Join(notes, "\n")
	if strings.Contains(s, "showIf") || !strings.Contains(all, "$V{PAGE_NUMBER}") {
		t.Errorf("a print-when on a variable: %s / %s", s, all)
	}
	if !strings.Contains(s, `"text": "[$F{name}.toUpperCase()]"`) {
		t.Errorf("a Java method call is kept as written: %s", s)
	}
	if !strings.Contains(s, `"groupBy": "f1, f2, f3, f4, f5, f6"`) {
		t.Errorf("six groups on fields after one that is not: %s", s)
	}
}
