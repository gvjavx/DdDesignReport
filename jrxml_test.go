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
		`<field name="price" class="java.lang.String"><fieldDescription><![CDATA[price]]>`, // a field of the rows
		`<sortField name="total" order="Descending"/>`,
		`<group name="g1">`, `<groupExpression><![CDATA[$F{region}]]>`,
		`resetType="Group" resetGroup="g1" calculation="Sum"><variableExpression><![CDATA[((((($F{qty}) as Double) * (($F{price}) as Double))) as Double)]]>`,
		`evaluationTime="Auto"`, `evaluationTime="Master"`, `$V{MASTER_CURRENT_PAGE}`, `$V{MASTER_TOTAL_PAGES}`, `isSummaryWithPageHeaderAndFooter="true"`,
		`markup="styled"`, `<style isBold="true" pdfFontName="Helvetica-Bold">`,
		`<sortField name="region"/>
	<sortField name="total" order="Descending"/>`, // the group first: Jasper groups rows next to each other
		`lineSpacing="Fixed" lineSpacingSize="12"`, // as ours: 10pt × 1.2
		`groupingSeparator = '.' as char; decimalSeparator = ',' as char`,
		`<printWhenExpression><![CDATA[((((($F{qty}) as Double) > 0d)) ? Boolean.TRUE : Boolean.FALSE)]]>`,
		`<jr:QRCode `, `textAlignment="Right"`, `fontName="SansSerif" size="14" isBold="true" isItalic="false" isUnderline="false" pdfFontName="Helvetica-Bold"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	// what has no counterpart is said at the top
	note, _ := ToJRXML([]byte(`{"bands":{"detail":{"height":40,"elements":[{"type":"chart","w":100,"h":40,"label":"name","value":"total"},{"type":"text","w":100,"h":14,"text":"{{terbilang(total)}}"}]}}}`))
	if bytes.Contains(note, []byte("charts are left out")) || !bytes.Contains(note, []byte("<barChart>")) || bytes.Contains(note, []byte("Note: terbilang")) || !bytes.Contains(note, []byte(".call($F{total})")) {
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
	for _, want := range []string{"a chart whose category or value", "pictures from files", "substring"} {
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

// a table with rows from data repeats them in a jr:list over the items, its total row
// under the list (moving down with it) summing the items in Groovy
func TestToJRXMLDataTable(t *testing.T) {
	jr, err := ToJRXML([]byte(`{"bands": {"summary": {"height": 80, "elements": [
	  {"type": "table", "data": "items", "x": 0, "y": 0, "w": 300, "h": 42, "header": true, "footer": true, "border": 1,
	   "cells": [["No", "Name", "Qty"], ["{{index}}", "{{name}}", "{{qty}}"], ["Total", "", "{{sum(qty)}}"]]},
	  {"type": "text", "x": 0, "y": 50, "w": 300, "h": 12, "text": "under the table"}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, "jrxml", jr)
	s := string(jr)
	for _, want := range []string{
		`<subDataset name="list1">`, `<field name="name" class="java.lang.Object"/>`,
		`<datasetRun subDataset="list1">`, `<dataSourceExpression><![CDATA[new net.sf.jasperreports.engine.data.JRMapCollectionDataSource((java.util.Collection) ($P{items} ?: []))]]>`,
		`<jr:listContents height="16" width="301">`, `$V{REPORT_COUNT}`, // the item's number
		`(($P{items} ?: []).collect{ r -> ((r?.get('qty')) as Double) }.findAll{ it != null }.sum() ?: 0d)`,
		`<reportElement x="0" y="50" width="300" height="14" positionType="Float">`, // under the list: moves down with it
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(s, "prints its rows once") {
		t.Error("still notes the rows as printed once")
	}
}

// a chart sums each label's values in a variable, as ours does (Jasper's datasets
// keep the last), and draws them once its rows are done; per group in a group band
func TestToJRXMLChart(t *testing.T) {
	jr, err := ToJRXML([]byte(`{"groupBy": "region", "bands": {
	  "groupFooter": {"height": 120, "elements": [{"type": "chart", "chart": "pie", "label": "city", "value": "qty * price", "w": 200, "h": 100}]},
	  "summary": {"height": 120, "elements": [{"type": "chart", "chart": "hbar", "label": "city", "value": "total", "w": 200, "h": 100}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, "jrxml", jr)
	s := string(jr)
	for _, want := range []string{
		`class="java.util.Map" resetType="Group" resetGroup="g1" calculation="Nothing"`, // the pie: per region
		`m.put(k, ((m.get(k) ?: 0d) as Double) + v)`,
		`$F{city} == null ? '' : String.valueOf($F{city})`,
		`<pieChart><chart isShowLegend="true" evaluationTime="Group" evaluationGroup="g1">`,
		`<seriesColor seriesOrder="1" color="#f28e2b"/>`,
		`<barChart><chart isShowLegend="false" evaluationTime="Report">`, `<plot orientation="Horizontal">`,
		`JRMapCollectionDataSource($V{chart`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
}

// a sub-report prints its column header, its detail band in a jr:list over the
// items (a field the item has not comes from the row around it), and its summary
func TestToJRXMLSubreport(t *testing.T) {
	jr, err := ToJRXML([]byte(`{"bands": {"detail": {"height": 80, "elements": [
	  {"type": "subreport", "data": "lines", "x": 20, "y": 10, "w": 300, "h": 60, "report": {"bands": {
	    "columnHeader": {"height": 14, "elements": [{"type": "text", "w": 100, "h": 12, "text": "Item"}]},
	    "detail": {"height": 14, "elements": [{"type": "text", "w": 200, "h": 12, "text": "{{name}} of {{number}}"}]},
	    "summary": {"height": 14, "elements": [{"type": "text", "w": 100, "h": 12, "text": "{{count()}} items"}]}}}},
	  {"type": "text", "x": 0, "y": 70, "w": 100, "h": 10, "text": "after"}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	wellFormed(t, "jrxml", jr)
	s := string(jr)
	for _, want := range []string{
		`<staticText><reportElement x="20" y="10"`, // the column header, in the box
		`<parameter name="parent_number" class="java.lang.Object"/>`,
		`<datasetParameter name="parent_number"><datasetParameterExpression><![CDATA[$F{number}]]>`,
		`($F{name} != null ? $F{name} : $P{parent_name})`,
		`readValue(($F{lines} ?: '[]') as String, java.util.List)`, // the row's list, read from its JSON
		`(((new com.fasterxml.jackson.databind.ObjectMapper()`,     // the summary counts the items
		`<reportElement x="0" y="70" width="100" height="14" positionType="Float">`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(s, "sub-reports are left out") {
		t.Error("still notes sub-reports as left out")
	}
}

// every ready-made template comes back from a trip through .jrxml printing as it did
func TestJRXMLRoundTripPrintsTheSame(t *testing.T) {
	entries, _ := os.ReadDir("designer/templates")
	for _, f := range entries {
		id, ok := strings.CutSuffix(f.Name(), ".json")
		if !ok || id == "index" {
			continue
		}
		tpl, data := readyMade(t, id)
		jr, err := ToJRXML(tpl)
		if err != nil {
			t.Fatal(err)
		}
		back, notes, err := FromJRXML(jr)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(notes) > 0 {
			t.Errorf("%s: notes on the way back: %q", id, notes)
		}
		before, _ := Render(tpl, data, "txt")
		after, err := Render(back, data, "txt")
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("%s prints differently after the trip (%v):\n%s\n---\n%s", id, err, before, after)
		}
	}
}

// what Jasper's own reports have: charts on a field, lists, crosstabs, and Java's c ? a : b
func TestFromJRXMLJasperParts(t *testing.T) {
	b, notes, err := FromJRXML([]byte(`<jasperReport xmlns="http://jasperreports.sourceforge.net/jasperreports" name="r" pageWidth="595" pageHeight="842">
	<field name="region" class="java.lang.String"/><field name="amount" class="java.math.BigDecimal"/><field name="month" class="java.lang.String"/>
	<summary><band height="300">
	  <barChart><chart><reportElement x="0" y="0" width="200" height="100"/></chart>
	    <categoryDataset><categorySeries><seriesExpression><![CDATA["s"]]></seriesExpression><categoryExpression><![CDATA[$F{region}]]></categoryExpression><valueExpression><![CDATA[$F{amount}]]></valueExpression></categorySeries></categoryDataset>
	    <barPlot><plot orientation="Horizontal"/></barPlot></barChart>
	  <pie3DChart><chart><reportElement x="200" y="0" width="200" height="100"/></chart>
	    <pieDataset><keyExpression><![CDATA[$F{region}]]></keyExpression><valueExpression><![CDATA[$F{amount} * 2]]></valueExpression></pieDataset><pie3DPlot/></pie3DChart>
	  <componentElement><reportElement x="0" y="110" width="300" height="20"/>
	    <jr:list xmlns:jr="http://jasperreports.sourceforge.net/jasperreports/components"><datasetRun subDataset="items"><dataSourceExpression><![CDATA[new JRBeanCollectionDataSource($F{lines})]]></dataSourceExpression></datasetRun>
	      <jr:listContents height="20" width="300"><textField><reportElement x="0" y="0" width="100" height="20"/><textFieldExpression><![CDATA[$F{name}]]></textFieldExpression></textField></jr:listContents></jr:list></componentElement>
	  <crosstab><reportElement x="0" y="140" width="300" height="60"/>
	    <rowGroup name="r"><bucket><bucketExpression><![CDATA[$F{region}]]></bucketExpression></bucket></rowGroup>
	    <columnGroup name="c"><bucket><bucketExpression><![CDATA[$F{month}]]></bucketExpression></bucket></columnGroup>
	    <measure name="m" calculation="Sum"><measureExpression><![CDATA[$F{amount}]]></measureExpression></measure></crosstab>
	  <textField><reportElement x="0" y="210" width="200" height="20"/><textFieldExpression><![CDATA[$F{amount} >= 0 ? "PROFIT" : "LOSS"]]></textFieldExpression></textField>
	</band></summary></jasperReport>`))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"chart": "hbar"`, `"label": "region"`, `"value": "amount"`, `"chart": "pie"`, `"value": "amount * 2"`,
		`"type": "subreport"`, `"data": "lines"`, `"text": "{{name}}"`,
		`"type": "crosstab"`, `"rowField": "region"`, `"columnField": "month"`, `"measure": "sum(amount)"`,
		`"text": "{{if(amount >= 0, 'PROFIT', 'LOSS')}}"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in\n%s", want, s)
		}
	}
	if len(notes) > 0 {
		t.Errorf("notes: %q", notes)
	}
	// in the summary, amount and lines are values of the data beside the rows
	data := map[string]any{"amount": 5, "lines": []any{map[string]any{"name": "Kopi"}}, "rows": []map[string]any{{"region": "Jawa", "month": "Jan", "amount": 100}}}
	if html := string(render(t, b, data, "html")); !strings.Contains(html, "PROFIT") || !strings.Contains(html, "Kopi") {
		t.Errorf("render: %s", html)
	}
}
