package ddreport

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wellFormed fails when the document is not XML.
func wellFormed(t *testing.T, name string, doc []byte) {
	t.Helper()
	d := xml.NewDecoder(bytes.NewReader(doc))
	for {
		if _, err := d.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("%s: not XML: %v", name, err)
		}
	}
}

// Every ready-made template goes to JRXML and back, and still renders.
func TestJRXMLRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("designer/templates/*.json")
	for _, f := range files {
		if strings.HasSuffix(f, "index.json") {
			continue
		}
		var file struct {
			Template json.RawMessage
			Data     any
		}
		b, _ := os.ReadFile(f)
		json.Unmarshal(b, &file)
		jr, err := Render(file.Template, nil, "jrxml")
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		wellFormed(t, f, jr)
		for _, want := range []string{`<jasperReport `, `language="groovy"`, `<detail>`, `</jasperReport>`} {
			if !bytes.Contains(jr, []byte(want)) {
				t.Errorf("%s: missing %s", f, want)
			}
		}
		back, notes, err := FromJRXML(jr)
		if err != nil {
			t.Fatalf("%s: back: %v", f, err)
		}
		if _, err := Render(back, file.Data, "pdf"); err != nil {
			t.Errorf("%s: the template read back does not render: %v (notes %v)", f, err, notes)
		}
	}
}

func TestToJRXML(t *testing.T) {
	tpl := []byte(`{"groupBy":"region","sortBy":"-total","thousands":".","decimal":",","bands":{
	  "header":{"height":30,"elements":[{"type":"text","w":300,"h":20,"text":"{{title}}","bold":true,"fontSize":14}]},
	  "groupHeader":{"height":16,"elements":[{"type":"text","w":200,"h":14,"text":"Region {{region}}"}]},
	  "detail":{"height":16,"elements":[{"type":"text","w":100,"h":14,"text":"{{name}}"},
	    {"type":"text","x":100,"w":80,"h":14,"text":"{{qty * price:2}}","align":"right","showIf":"qty > 0"},
	    {"type":"barcode","x":200,"w":14,"h":14,"text":"{{code}}"}]},
	  "groupFooter":{"height":16,"elements":[{"type":"text","w":200,"h":14,"text":"Subtotal <b>{{sum(qty * price):2}}</b>"}]},
	  "footer":{"height":16,"elements":[{"type":"text","w":200,"h":14,"text":"Page {{page}} of {{pages}}"}]}}}`)
	jr, err := ToJRXML(tpl)
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, "jrxml", jr)
	s := string(jr)
	for _, want := range []string{
		`<parameter name="title" class="java.lang.Object"`,                                 // a value of the data, outside the rows
		`<field name="price" class="java.lang.Object"><fieldDescription><![CDATA[price]]>`, // a field of the rows
		`<sortField name="total" order="Descending"/>`,
		`<group name="g1">`, `<groupExpression><![CDATA[$F{region}]]>`,
		`resetType="Group" resetGroup="g1" calculation="Sum"><variableExpression><![CDATA[((((($F{qty}) as Double) * (($F{price}) as Double))) as Double)]]>`,
		`evaluationTime="Auto"`, `$V{CURRENT_PAGE}`, `$V{PAGE_NUMBER}`, `resetType="Page"`,
		`markup="styled"`, `<style isBold="true">`,
		`groupingSeparator = '.' as char; decimalSeparator = ',' as char`,
		`<printWhenExpression><![CDATA[((((($F{qty}) as Double) > 0d)) ? Boolean.TRUE : Boolean.FALSE)]]>`,
		`<jr:QRCode `, `textAlignment="Right"`, `fontName="SansSerif" size="14" isBold="true"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	// what has no counterpart is said at the top
	note, _ := ToJRXML([]byte(`{"bands":{"detail":{"height":40,"elements":[{"type":"chart","w":100,"h":40},{"type":"text","w":100,"h":14,"text":"{{terbilang(total)}}"}]}}}`))
	if !bytes.Contains(note, []byte("Note: charts are left out")) || !bytes.Contains(note, []byte("Note: terbilang")) {
		t.Errorf("notes: %s", note)
	}
	// text that closes CDATA cannot break the document
	odd, _ := ToJRXML([]byte(`{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"a ]]> b <c> & \"d\""}]}}}`))
	wellFormed(t, "odd", odd)
}

// A report as Jaspersoft Studio writes one.
const studioReport = `<?xml version="1.0" encoding="UTF-8"?>
<jasperReport xmlns="http://jasperreports.sourceforge.net/jasperreports" name="sales" pageWidth="595" pageHeight="842" columnWidth="555" leftMargin="20" rightMargin="20" topMargin="20" bottomMargin="20" uuid="1">
	<field name="region" class="java.lang.String"/>
	<field name="customerName" class="java.lang.String"><fieldDescription><![CDATA[customer.name]]></fieldDescription></field>
	<field name="amount" class="java.math.BigDecimal"/>
	<sortField name="amount" order="Descending"/>
	<variable name="regionTotal" class="java.math.BigDecimal" resetType="Group" resetGroup="byRegion" calculation="Sum"><variableExpression><![CDATA[$F{amount}]]></variableExpression></variable>
	<group name="byRegion">
		<groupExpression><![CDATA[$F{region}]]></groupExpression>
		<groupHeader><band height="20"><textField><reportElement x="0" y="0" width="200" height="20"/><textElement><font isBold="true"/></textElement><textFieldExpression><![CDATA["Region: " + $F{region}]]></textFieldExpression></textField></band></groupHeader>
		<groupFooter><band height="20"><textField pattern="#,##0.00"><reportElement x="355" y="0" width="200" height="20"/><textElement textAlignment="Right"/><textFieldExpression><![CDATA[$V{regionTotal}]]></textFieldExpression></textField></band></groupFooter>
	</group>
	<title><band height="40"><staticText><reportElement x="0" y="0" width="555" height="30" forecolor="#1E3A8A"/><textElement textAlignment="Center"><font fontName="Serif" size="18" isBold="true"/></textElement><text><![CDATA[Sales report]]></text></staticText></band></title>
	<pageHeader><band height="20"><textField pattern="dd/MM/yyyy"><reportElement x="455" y="0" width="100" height="20"/><textFieldExpression><![CDATA[new java.util.Date()]]></textFieldExpression></textField></band></pageHeader>
	<detail><band height="20">
		<frame><reportElement x="0" y="0" width="555" height="20" mode="Opaque" backcolor="#F1F5F9"/><box><pen lineWidth="0.5"/></box>
			<textField><reportElement x="10" y="0" width="200" height="20"/><textFieldExpression><![CDATA[$F{customerName}]]></textFieldExpression></textField>
			<textField pattern="#,##0"><reportElement x="355" y="0" width="190" height="20"><printWhenExpression><![CDATA[$F{amount} > 0]]></printWhenExpression></reportElement><textFieldExpression><![CDATA[$F{amount} * 1.11]]></textFieldExpression></textField>
		</frame>
		<line><reportElement x="0" y="19" width="555" height="1"/></line>
		<image><reportElement x="530" y="0" width="20" height="20"/><imageExpression><![CDATA["logo.png"]]></imageExpression></image>
	</band></detail>
	<pageFooter><band height="20"><textField evaluationTime="Report"><reportElement x="0" y="0" width="200" height="20"/><textFieldExpression><![CDATA["of " + $V{PAGE_NUMBER}]]></textFieldExpression></textField></band></pageFooter>
	<summary><band height="60"><barChart><chart><reportElement x="0" y="0" width="300" height="60"/></chart></barChart>
		<componentElement><reportElement x="400" y="0" width="50" height="50"/><jr:QRCode xmlns:jr="http://jasperreports.sourceforge.net/jasperreports/components"><jr:codeExpression><![CDATA["INV-" + $F{region}]]></jr:codeExpression></jr:QRCode></componentElement>
		<textField><reportElement x="0" y="40" width="300" height="20"/><textFieldExpression><![CDATA[$F{region}.substring(0, 2)]]></textFieldExpression></textField>
	</band></summary>
</jasperReport>`

func TestFromJRXML(t *testing.T) {
	b, notes, err := FromJRXML([]byte(studioReport))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"groupBy": "region"`, `"sortBy": "-amount"`,
		`"text": "Region: {{region}}"`,               // text joined to a field
		`"text": "{{sum(amount):2}}"`,                // a Sum variable with its pattern
		`"text": "{{customer.name}}"`,                // a field by its description
		`"text": "{{amount * 1.11:0}}"`,              // arithmetic
		`"showIf": "amount > 0"`,                     // a simple print-when
		`"text": "{{now:dd/MM/yyyy}}"`,               // the date
		`"text": "of {{pages}}"`,                     // the page count, evaluated at the end
		`"text": "INV-{{region}}"`, `"format": "qr"`, // the QR code
		`"text": "[$F{region}.substring(0, 2)]"`,  // Java kept as written
		`"showIf": "page = 1"`,                    // the title, on top of the header
		`"fill": "#f1f5f9"`, `"color": "#1e3a8a"`, // the frame's background, colours
		`"font": "Times"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in\n%s", want, s)
		}
	}
	all := strings.Join(notes, "\n")
	for _, want := range []string{"barChart", "pictures from files", "substring"} {
		if !strings.Contains(all, want) {
			t.Errorf("notes do not mention %s: %s", want, all)
		}
	}
	// and it renders
	data := map[string]any{"rows": []map[string]any{{"region": "Jawa", "customer": map[string]any{"name": "Ani"}, "amount": 100}}}
	if html := string(render(t, b, data, "html")); !strings.Contains(html, "Region: Jawa") || !strings.Contains(html, ">Ani<") {
		t.Errorf("render: %s", html)
	}
	if _, _, err := FromJRXML([]byte(`<html></html>`)); err == nil || !strings.Contains(err.Error(), "not a JasperReports report") {
		t.Errorf("not a report: %v", err)
	}
}
