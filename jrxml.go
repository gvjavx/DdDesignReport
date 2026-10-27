package ddreport

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ToJRXML writes a template as a JasperReports 6 report (.jrxml), for teams
// that print with JasperReports or edit in Jaspersoft Studio. Expressions are
// Groovy (language="groovy"). Fields are java.lang.Object with their path as
// field description, so a JsonDataSource over the rows fills them; top-level
// fields used outside the row bands ({{title}}) become parameters. Aggregates
// become variables reset per group, {{page}} and {{pages}} page counts.
//
// Texts, tables (cell by cell), pictures, boxes, lines, barcodes (the jr:QRCode
// and jr:Code128 components), groups, sorting and the band options carry over.
// Charts, sub-reports, crosstabs, data tables' repeating rows and terbilang have
// no direct counterpart: they are left out or kept as they print once, and the
// comment at the top of the file lists them.
//
// It is also the "jrxml" format of Render and of the ddreport binary.
func ToJRXML(template []byte) ([]byte, error) {
	r, err := newReport(template, nil)
	if err != nil {
		return nil, err
	}
	x := &jasperOut{r: r, t: r.t, fields: map[string]string{}, params: map[string]bool{}}
	return x.report(), nil
}

type jasperOut struct {
	r      *report
	t      *Template
	fields map[string]string // Jasper name: path in the row
	params map[string]bool
	vars   strings.Builder
	nvars  int
	warn   []string
	page   bool // CURRENT_PAGE is needed

	// the band being written
	level int  // its group level, 1 outermost; 0 outside groups
	rowed bool // its fields are those of a row: detail and group bands
}

func (x *jasperOut) warnOnce(s string) {
	if !slices.Contains(x.warn, s) {
		x.warn = append(x.warn, s)
	}
}

var jasperName = regexp.MustCompile(`[^\p{L}\p{N}_]`)

// field is a reference to a value of the data: a field of the row, or a parameter.
func (x *jasperOut) field(path string, inAggregate bool) string {
	name := jasperName.ReplaceAllString(path, "_")
	if x.rowed || inAggregate {
		x.fields[name] = path
		return "$F{" + name + "}"
	}
	x.params[name] = true
	return "$P{" + name + "}"
}

// variable declares an aggregate and returns its reference.
func (x *jasperOut) variable(fn, arg string) string {
	x.nvars++
	name := fmt.Sprintf("v%d", x.nvars)
	calc := map[string]string{"sum": "Sum", "avg": "Average", "min": "Lowest", "max": "Highest", "count": "Count"}[fn]
	class, expr := "java.lang.Double", "(("+arg+") as Double)"
	if fn == "count" {
		class, expr = "java.lang.Integer", "1"
		if arg != "" {
			expr = arg
		}
	}
	reset := ` resetType="Report"`
	switch {
	case x.level > 0:
		reset = fmt.Sprintf(` resetType="Group" resetGroup="g%d"`, x.level)
	case x.rowed && len(x.t.levels()) > 0: // the detail band: the innermost group
		reset = fmt.Sprintf(` resetType="Group" resetGroup="g%d"`, len(x.t.levels()))
	}
	fmt.Fprintf(&x.vars, `	<variable name="%s" class="%s"%s calculation="%s"><variableExpression><![CDATA[%s]]></variableExpression></variable>`+"\n",
		name, class, reset, calc, cdata(expr))
	return "$V{" + name + "}"
}

// groovy is the Groovy for one of our expressions; ok is false for what has no counterpart.
func (x *jasperOut) groovy(src string) (string, bool) {
	p := &jasperParser{x: x, src: src}
	g, _ := p.expr()
	if p.peek() != 0 || p.bad {
		return "", false
	}
	return g, true
}

// jasperParser reads our expression grammar (see expr.go) and writes Groovy.
// Each part says whether it is a number, so arithmetic and comparisons can
// turn the other side into one, as ours do.
type jasperParser struct {
	x     *jasperOut
	src   string
	pos   int
	bad   bool
	inAgg bool
	depth int
}

func (p *jasperParser) peek() byte {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *jasperParser) keyword(w string) bool {
	p.peek()
	rest := p.src[p.pos:]
	if !strings.HasPrefix(rest, w) || len(operand.FindString(rest)) != len(w) {
		return false
	}
	p.pos += len(w)
	return true
}

func asNumber(g string, isNum bool) string {
	if isNum {
		return g
	}
	return "((" + g + ") as Double)"
}

func (p *jasperParser) expr() (string, bool) {
	g, n := p.and()
	for !p.bad && p.keyword("or") {
		h, _ := p.and()
		g, n = "("+g+" || "+h+")", false
	}
	return g, n
}

func (p *jasperParser) and() (string, bool) {
	g, n := p.compare()
	for !p.bad && p.keyword("and") {
		h, _ := p.compare()
		g, n = "("+g+" && "+h+")", false
	}
	return g, n
}

func (p *jasperParser) compare() (string, bool) {
	g, n := p.sum()
	if p.peek() == 0 || p.bad {
		return g, n
	}
	for _, o := range []string{"==", "!=", "<>", "<=", ">=", "=", "<", ">"} {
		if strings.HasPrefix(p.src[p.pos:], o) {
			p.pos += len(o)
			h, m := p.sum()
			if n || m { // a number on one side: compare as numbers
				g, h = asNumber(g, n), asNumber(h, m)
			}
			op := map[string]string{"=": "==", "<>": "!="}[o]
			if op == "" {
				op = o
			}
			return "(" + g + " " + op + " " + h + ")", false
		}
	}
	return g, n
}

func (p *jasperParser) chain(ops string, next func() (string, bool)) (string, bool) {
	g, n := next()
	for !p.bad {
		op := p.peek()
		if op == 0 || !strings.ContainsRune(ops, rune(op)) {
			break
		}
		p.pos++
		h, m := next()
		g, n = "("+asNumber(g, n)+" "+string(op)+" "+asNumber(h, m)+")", true
	}
	return g, n
}

func (p *jasperParser) sum() (string, bool)     { return p.chain("+-", p.product) }
func (p *jasperParser) product() (string, bool) { return p.chain("*/", p.factor) }

func groovyString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`).Replace(s) + "'"
}

func (p *jasperParser) factor() (string, bool) {
	if p.depth++; p.depth > maxDepth {
		p.bad = true
		return "", false
	}
	defer func() { p.depth-- }()
	switch c := p.peek(); c {
	case '(':
		p.pos++
		g, n := p.expr()
		if p.peek() != ')' {
			p.bad = true
		}
		p.pos++
		return "(" + g + ")", n
	case '-':
		p.pos++
		g, n := p.factor()
		return "(-" + asNumber(g, n) + ")", true
	case '[':
		end := strings.IndexByte(p.src[p.pos:], ']')
		if end < 0 {
			p.bad = true
			return "", false
		}
		key := p.src[p.pos+1 : p.pos+end]
		p.pos += end + 1
		return p.x.field(key, p.inAgg), false
	case '\'', '"':
		end := strings.IndexByte(p.src[p.pos+1:], c)
		if end < 0 {
			p.bad = true
			return "", false
		}
		text := p.src[p.pos+1 : p.pos+1+end]
		p.pos += end + 2
		return groovyString(text), false
	}
	if p.keyword("not") {
		g, _ := p.factor()
		return "!(" + g + ")", false
	}
	word := operand.FindString(p.src[p.pos:])
	p.pos += len(word)
	if word == "" {
		p.bad = true
		return "", false
	}
	if c := word[0]; c >= '0' && c <= '9' || c == '.' {
		if _, err := strconv.ParseFloat(word, 64); err == nil {
			return word + "d", true
		}
	}
	if p.peek() != '(' {
		switch word {
		case "page":
			p.x.page = true
			return "$V{CURRENT_PAGE}", true
		case "pages":
			return "$V{PAGE_NUMBER}", true
		case "index":
			if n := len(p.x.t.levels()); n > 0 && p.x.rowed {
				return fmt.Sprintf("$V{g%d_COUNT}", n), true
			}
			return "$V{REPORT_COUNT}", true
		case "now":
			return "new java.util.Date()", false
		}
		return p.x.field(word, p.inAgg), false
	}
	p.pos++
	var args []string
	var nums []bool
	agg := slices.Contains([]string{"sum", "avg", "min", "max", "count"}, word)
	before := p.inAgg
	p.inAgg = p.inAgg || agg
	for p.peek() != ')' && !p.bad {
		g, n := p.expr()
		args, nums = append(args, g), append(nums, n)
		if p.peek() != ',' {
			break
		}
		p.pos++
	}
	p.inAgg = before
	if p.peek() != ')' {
		p.bad = true
		return "", false
	}
	p.pos++
	switch {
	case agg && len(args) <= 1:
		arg := ""
		if len(args) == 1 {
			arg = args[0]
		}
		return p.x.variable(word, arg), word != "min" && word != "max"
	case word == "if" && (len(args) == 2 || len(args) == 3):
		no := "null"
		if len(args) == 3 {
			no = args[2]
		}
		return "((" + args[0] + ") ? " + args[1] + " : " + no + ")", false
	case (word == "upper" || word == "lower") && len(args) == 1:
		m := map[string]string{"upper": "toUpperCase", "lower": "toLowerCase"}[word]
		return "(" + args[0] + " == null ? null : String.valueOf(" + args[0] + ")." + m + "())", false
	case word == "round" && (len(args) == 1 || len(args) == 2):
		places := "0"
		if len(args) == 2 {
			places = "((" + args[1] + ") as Integer)"
		}
		return "(" + asNumber(args[0], nums[0]) + " == null ? null : new java.math.BigDecimal(" + asNumber(args[0], nums[0]) + ".toString()).setScale(" + places + ", java.math.RoundingMode.HALF_UP).doubleValue())", true
	case word == "terbilang":
		p.x.warnOnce("terbilang(...) has no JasperReports counterpart: the number is printed in digits")
		return args[0], nums[0]
	}
	p.bad = true
	return "", false
}

// text is the Groovy for a text with placeholders: its parts joined. ok is
// false when a placeholder has no counterpart, and vars when one uses a
// variable (aggregates, pages), so the text is evaluated when they are known.
func (x *jasperOut) text(s string) (g string, vars, ok bool) {
	var parts []string
	last := 0
	for _, m := range placeholder.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > last {
			parts = append(parts, groovyString(s[last:m[0]]))
		}
		last = m[1]
		expr, format := s[m[2]:m[3]], ""
		if m[4] >= 0 {
			format = s[m[4]:m[5]]
		}
		v, good := x.groovy(expr)
		if !good {
			x.warnOnce("a placeholder Jasper cannot express is printed as written: {{" + expr + "}}")
			parts = append(parts, groovyString(s[m[0]:m[1]]))
			continue
		}
		vars = vars || strings.Contains(v, "$V{")
		parts = append(parts, x.format(v, format))
	}
	if last < len(s) || len(parts) == 0 {
		parts = append(parts, groovyString(s[last:]))
	}
	return strings.Join(parts, " + "), vars, true
}

// format prints a value as our placeholders do: numbers with the template's
// separators and decimals, dates with a pattern, nothing for no value.
func (x *jasperOut) format(v, format string) string {
	t := x.t
	thousands, decimal := t.Thousands, t.Decimal
	if thousands == "" && t.Thousands != "none" {
		thousands = ","
	}
	if decimal == "" {
		decimal = "."
	}
	symbols := fmt.Sprintf("new java.text.DecimalFormatSymbols(java.util.Locale.US).with { groupingSeparator = %s as char; decimalSeparator = %s as char; it }",
		groovyString(cmpOr(thousands, ",")), groovyString(decimal))
	if n := decimals(format); n >= 0 {
		pattern := "#,##0"
		if thousands == "" {
			pattern = "0"
		}
		if n > 0 {
			pattern += "." + strings.Repeat("0", n)
		}
		return fmt.Sprintf("({ v -> v == null || !(v.toString().isNumber()) ? (v == null ? '' : String.valueOf(v)) : new java.text.DecimalFormat(%s, %s).format(v.toString() as Double) })(%s)", groovyString(pattern), symbols, v)
	}
	if format != "" && dateToken.MatchString(format) {
		locale := "java.util.Locale.ENGLISH"
		if t.Locale == "id" {
			locale = "new java.util.Locale('id')"
		}
		return fmt.Sprintf("({ v -> v == null ? '' : v instanceof java.util.Date ? new java.text.SimpleDateFormat(%[1]s, %[2]s).format(v) : java.time.LocalDate.parse(String.valueOf(v).take(10)).format(java.time.format.DateTimeFormatter.ofPattern(%[1]s, %[2]s)) })(%[3]s)", groovyString(format), locale, v)
	}
	return fmt.Sprintf("({ v -> v == null ? '' : v instanceof Number ? new java.math.BigDecimal(v.toString()).stripTrailingZeros().toPlainString() : String.valueOf(v) })(%s)", v)
}

func cmpOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func cdata(s string) string { return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>") }

func attr(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\n", "&#10;").Replace(s)
}

func px(f float64) int { return int(math.Round(f)) }

var jasperFont = map[string]string{"Helvetica": "SansSerif", "": "SansSerif", "Times": "Serif", "Courier": "Monospaced"}

// element writes one element of a band.
func (x *jasperOut) element(b *strings.Builder, e Element) {
	common := func(extra string) string {
		s := fmt.Sprintf(`<reportElement x="%d" y="%d" width="%d" height="%d"%s`, px(e.X), px(e.Y), max(px(e.W), 1), max(px(e.H), 1), extra)
		if e.ShowIf != "" {
			if g, ok := x.groovy(e.ShowIf); ok {
				return s + `><printWhenExpression><![CDATA[` + cdata("(("+g+") ? Boolean.TRUE : Boolean.FALSE)") + `]]></printWhenExpression></reportElement>`
			}
			x.warnOnce("a show-when condition Jasper cannot express is left out: " + e.ShowIf)
		}
		return s + `/>`
	}
	colors := func(stroke string) string {
		s := ""
		if hexColor.MatchString(stroke) {
			s += ` forecolor="` + stroke + `"`
		}
		if hexColor.MatchString(e.Fill) {
			s += ` mode="Opaque" backcolor="` + e.Fill + `"`
		}
		return s
	}
	switch e.Type {
	case "text":
		font := jasperFont[e.Font]
		if font == "" {
			font = e.Font
			x.warnOnce("the font " + e.Font + " needs a JasperReports font extension to print")
		}
		align := map[string]string{"center": "Center", "right": "Right", "justify": "Justified"}[e.Align]
		if align == "" {
			align = "Left"
		}
		valign := map[string]string{"middle": "Middle", "bottom": "Bottom"}[e.VAlign]
		if valign == "" {
			valign = "Top"
		}
		text := e.Text
		markup := ""
		if styleTag.MatchString(text) {
			markup = ` markup="styled"`
			text = strings.NewReplacer("<b>", `<style isBold="true">`, "<i>", `<style isItalic="true">`, "<u>", `<style isUnderline="true">`, "</b>", "</style>", "</i>", "</style>", "</u>", "</style>").Replace(text)
		}
		box := ""
		if e.Border > 0 {
			box = fmt.Sprintf(`<box><pen lineWidth="%g"/></box>`, e.Border)
		}
		textElement := fmt.Sprintf(`<textElement textAlignment="%s" verticalAlignment="%s"%s><font fontName="%s" size="%g" isBold="%t" isItalic="%t" isUnderline="%t"/><paragraph lineSpacing="Proportional" lineSpacingSize="%g"/></textElement>`,
			align, valign, markup, attr(font), e.size(), e.Bold, e.Italic, e.Underline, e.leading())
		el := common(colors(e.Color))
		if !strings.Contains(e.Text, "{{") {
			fmt.Fprintf(b, "\t\t\t<staticText>%s%s%s<text><![CDATA[%s]]></text></staticText>\n", el, box, textElement, cdata(text))
			return
		}
		g, vars, _ := x.text(text)
		when := ""
		if vars {
			when = ` evaluationTime="Auto"`
		}
		grow := ""
		if e.Grow {
			grow = ` textAdjust="StretchHeight"`
		}
		fmt.Fprintf(b, "\t\t\t<textField%s%s isBlankWhenNull=\"true\">%s%s%s<textFieldExpression><![CDATA[%s]]></textFieldExpression></textField>\n", grow, when, el, box, textElement, cdata(g))
	case "rect":
		fmt.Fprintf(b, "\t\t\t<rectangle>%s<graphicElement><pen lineWidth=\"%g\"/></graphicElement></rectangle>\n", common(colors(e.Color)), max(e.Border, 0))
	case "line":
		e.Fill = ""
		dir := ""
		if e.Slope == "up" {
			dir = ` direction="BottomUp"`
		}
		if e.Slope == "" { // flat or upright: one point thick the other way
			if e.H > e.W {
				e.W = 0
			} else {
				e.H = 0
			}
		}
		fmt.Fprintf(b, "\t\t\t<line%s>%s<graphicElement><pen lineWidth=\"%g\"/></graphicElement></line>\n", dir, common(colors(e.Color)), max(e.Border, 0.5))
	case "image":
		var src string
		switch {
		case strings.Contains(e.Src, "{{"):
			g, _, _ := x.text(strings.TrimSpace(e.Src))
			src = "new java.io.ByteArrayInputStream(java.util.Base64.getDecoder().decode((" + g + ").replaceFirst('^data:[^,]*,', '')))"
		case e.Src != "":
			_, b64, _ := strings.Cut(e.Src, ";base64,")
			src = "new java.io.ByteArrayInputStream(java.util.Base64.getDecoder().decode('" + b64 + "'))"
		default:
			return
		}
		fmt.Fprintf(b, "\t\t\t<image scaleImage=\"RetainShape\" hAlign=\"Center\" vAlign=\"Middle\" isUsingCache=\"false\" onErrorType=\"Blank\">%s<imageExpression><![CDATA[%s]]></imageExpression></image>\n", common(""), cdata(src))
	case "barcode":
		g, _, _ := x.text(e.Text)
		kind := "QRCode"
		if e.Format == "code128" {
			kind = "Code128"
		}
		fmt.Fprintf(b, "\t\t\t<componentElement>%s<jr:%s xmlns:jr=\"http://jasperreports.sourceforge.net/jasperreports/components\" xsi:schemaLocation=\"http://jasperreports.sourceforge.net/jasperreports/components http://jasperreports.sourceforge.net/xsd/components.xsd\"><jr:codeExpression><![CDATA[%s]]></jr:codeExpression></jr:%s></componentElement>\n",
			common(colors(e.Color)), kind, cdata(g), kind)
	case "table", "crosstab":
		if e.Type == "crosstab" {
			x.warnOnce("a crosstab is written as the table it prints with no data; use a Jasper crosstab for one that follows the data")
			e = x.r.crosstab(e, scope{x.t, nil, nil})
		}
		if e.Data != "" {
			x.warnOnce("a table with rows from data (" + e.Data + ") prints its rows once; use a Jasper list or sub-report to repeat them per item")
		}
		cells, _ := x.r.grid(e, scope{x.t, nil, nil}, false, true)
		for _, c := range cells {
			c.ShowIf = e.ShowIf
			if c.Type == "text" {
				c.Text = strings.Join(slices.Collect(func(yield func(string) bool) {
					for _, r := range c.parse() {
						yield(r.text)
					}
				}), "")
			}
			x.element(b, c)
		}
	case "chart":
		x.warnOnce("charts are left out: add a Jasper chart where the box was")
		fmt.Fprintf(b, "\t\t\t<rectangle>%s<graphicElement><pen lineWidth=\"0.5\" lineStyle=\"Dashed\"/></graphicElement></rectangle>\n", common(""))
	case "subreport":
		x.warnOnce("sub-reports are left out: add a Jasper sub-report where the box was")
		fmt.Fprintf(b, "\t\t\t<rectangle>%s<graphicElement><pen lineWidth=\"0.5\" lineStyle=\"Dashed\"/></graphicElement></rectangle>\n", common(""))
	}
}

// band writes a band, as tall as what is in it.
func (x *jasperOut) band(b *strings.Builder, tag string, band Band, level int, rowed bool) {
	x.level, x.rowed = level, rowed
	h := band.Height
	for _, e := range band.Elements {
		h = max(h, e.Y+e.H)
	}
	var els strings.Builder
	for _, e := range band.Elements {
		x.element(&els, e)
	}
	fmt.Fprintf(b, "\t<%s>\n\t\t<band height=\"%d\" splitType=\"Stretch\">\n%s\t\t</band>\n\t</%s>\n", tag, px(math.Ceil(h)), els.String(), tag)
}

func (x *jasperOut) report() []byte {
	t, bs := x.t, x.t.Bands
	top, right, bottom, left := t.Page.margins()
	var body strings.Builder

	// groups, outer first
	levels := t.levels()
	var groups strings.Builder
	for i, l := range levels {
		x.level, x.rowed = i+1, true
		by := "''"
		if l.by != "" {
			by = x.field(l.by, false)
		}
		newPage := ""
		if t.GroupPageBreak && i == 0 || l.header.NewPage {
			newPage = ` isStartNewPage="true"`
		}
		fmt.Fprintf(&groups, "\t<group name=\"g%d\"%s>\n\t\t<groupExpression><![CDATA[%s]]></groupExpression>\n", i+1, newPage, cdata(by))
		var gh, gf strings.Builder
		x.band(&gh, "groupHeader", l.header, i+1, true)
		x.band(&gf, "groupFooter", l.footer, i+1, true)
		groups.WriteString(gh.String() + gf.String() + "\t</group>\n")
	}

	// bands, in Jasper's order
	if bs.Header.Pages == "first" {
		x.band(&body, "title", bs.Header, 0, false)
	} else if len(bs.Header.Elements) > 0 || bs.Header.Height > 0 {
		x.band(&body, "pageHeader", bs.Header, 0, false)
	}
	x.band(&body, "columnHeader", bs.ColumnHeader, 0, false)
	x.band(&body, "detail", bs.Detail, 0, true)
	if bs.Footer.Pages == "last" {
		x.band(&body, "lastPageFooter", bs.Footer, 0, false)
	} else {
		x.band(&body, "pageFooter", bs.Footer, 0, false)
	}
	x.band(&body, "summary", bs.Summary, 0, false)

	var out strings.Builder
	out.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	out.WriteString("<!-- Made by DdDesignReport from a template. Fill it with a JsonDataSource over the rows" +
		" (each field's description is its path in a row); the other values of the data are parameters.\n")
	slices.Sort(x.warn)
	for _, w := range x.warn {
		out.WriteString("     Note: " + strings.ReplaceAll(w, "--", "- -") + "\n")
	}
	out.WriteString("-->\n")
	summaryNewPage := ""
	if bs.Summary.NewPage {
		summaryNewPage = ` isSummaryNewPage="true"`
	}
	fmt.Fprintf(&out, `<jasperReport xmlns="http://jasperreports.sourceforge.net/jasperreports" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://jasperreports.sourceforge.net/jasperreports http://jasperreports.sourceforge.net/xsd/jasperreport.xsd" name="report" language="groovy" pageWidth="%d" pageHeight="%d" columnWidth="%d" leftMargin="%d" rightMargin="%d" topMargin="%d" bottomMargin="%d" whenNoDataType="AllSectionsNoDetail"%s>`+"\n",
		px(t.Page.Width), px(t.Page.Height), px(t.Page.Width-left-right), px(left), px(right), px(top), px(bottom), summaryNewPage)
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range x.params {
			if !yield(k) {
				return
			}
		}
	}) {
		fmt.Fprintf(&out, "\t<parameter name=\"%s\" class=\"java.lang.Object\" isForPrompting=\"false\"/>\n", name)
	}
	// the sort fields must be fields too
	var sorts strings.Builder
	for _, s := range strings.Split(t.SortBy, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		order := ""
		if strings.HasPrefix(s, "-") {
			s, order = strings.TrimSpace(s[1:]), ` order="Descending"`
		}
		name := jasperName.ReplaceAllString(s, "_")
		x.fields[name] = s
		fmt.Fprintf(&sorts, "\t<sortField name=\"%s\"%s/>\n", name, order)
	}
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range x.fields {
			if !yield(k) {
				return
			}
		}
	}) {
		fmt.Fprintf(&out, "\t<field name=\"%s\" class=\"java.lang.Object\"><fieldDescription><![CDATA[%s]]></fieldDescription></field>\n", name, cdata(x.fields[name]))
	}
	out.WriteString(sorts.String())
	if x.page {
		out.WriteString("\t<variable name=\"CURRENT_PAGE\" class=\"java.lang.Integer\" resetType=\"Page\"><variableExpression><![CDATA[$V{PAGE_NUMBER}]]></variableExpression></variable>\n")
	}
	out.WriteString(x.vars.String())
	out.WriteString(groups.String())
	out.WriteString(body.String())
	out.WriteString("</jasperReport>\n")
	return []byte(out.String())
}
