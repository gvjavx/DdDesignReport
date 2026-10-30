package ddreport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ToJRXML writes a template as a JasperReports 6 report (.jrxml), for teams
// that print with JasperReports or edit in Jaspersoft Studio. Expressions are
// Groovy (language="groovy"). Fields are java.lang.String (Groovy turns "12" into a number where one is needed) with their path as
// field description, so a JsonDataSource over the rows fills them; top-level
// fields used outside the row bands ({{title}}) become parameters. Aggregates
// become variables reset per group, {{page}} and {{pages}} page counts.
//
// Texts, tables (cell by cell), pictures, boxes, lines, barcodes (the jr:QRCode
// and jr:Code128 components), groups, sorting and the band options carry over;
// terbilang becomes a Groovy closure. A data table's repeating rows and a
// sub-report's detail band become a jr:list over the items (a parameter holding a
// java.util.List of Maps, or a row field holding a JSON list), with what is under
// them moving down as it grows. Charts are Jasper charts (they need JFreeChart);
// a donut prints as a pie. A crosstab is kept as the table it prints with no data.
// What has no counterpart is listed in the comment at the top of the file.
//
// Properties named ddreport.* keep what Jasper has no place for, so FromJRXML
// reads the template back as it was.
//
// It is also the "jrxml" format of Render and of the ddreport binary.
func ToJRXML(template []byte) ([]byte, error) {
	r, err := newReport(template, nil)
	if err != nil {
		return nil, err
	}
	x := &jasperOut{r: r, t: r.t, fields: map[string]string{}, params: map[string]string{}}
	// each element as written, so a data table or sub-report can be read back whole,
	// and the settings beside the page and bands
	var raw struct {
		Bands map[string]struct{ Elements []json.RawMessage }
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(template, &top) == nil {
		for k := range top {
			if k := strings.ToLower(k); k == "page" || k == "bands" {
				delete(top, k)
			}
		}
		if b, err := json.Marshal(top); err == nil && len(top) > 0 {
			x.settings = string(b)
		}
	}
	if json.Unmarshal(template, &raw) == nil {
		x.raw = map[string][]json.RawMessage{}
		for name, b := range raw.Bands {
			x.raw[name] = b.Elements
		}
	}
	return x.report(), nil
}

type jasperOut struct {
	r      *report
	t      *Template
	fields map[string]string // Jasper name: path in the row
	params map[string]string // Jasper name: path in the data
	vars   strings.Builder
	nvars  int
	warn   []string
	page   bool // the text being written has a page number: it is evaluated once the pages are known

	// the band being written
	level  int  // its group level, 1 outermost; 0 outside groups
	rowed  bool // its fields are those of a row: detail and group bands
	bottom int  // the lowest edge of what is written in it, as Jasper reads it
	right  int  // and the rightmost

	// data tables and sub-reports: their rows are a jr:list over a sub-dataset
	subsets   strings.Builder
	nlists    int
	list      map[string]string // writing a list's contents: the fields of its sub-dataset
	coll      string            // writing a data table's header or total row: Groovy for its items, which aggregates run over
	floatFrom float64           // in the band: what starts at or below this moves down as the list above it grows

	raw      map[string][]json.RawMessage // the elements of each band, as the template has them
	rawEl    json.RawMessage              // the one being written
	settings string                       // the template beside its page and bands, as JSON
	props    string                       // properties for the next reportElement
	part     bool                         // what is written is part of an element read back whole
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
	switch {
	case x.coll != "" && inAggregate: // an item of the collection an aggregate runs over
		return "r?.get(" + groovyString(path) + ")"
	case x.list != nil: // a list's contents: a field of its item, by its key, or else of the row around it, as ours
		x.list[path] = path
		return "($F{" + path + "} != null ? $F{" + path + "} : $P{parent_" + name + "})"
	}
	if x.rowed || inAggregate {
		x.fields[name] = path
		return "$F{" + name + "}"
	}
	x.params[name] = path
	return "$P{" + name + "}"
}

// variable declares an aggregate and returns its reference.
func (x *jasperOut) variable(fn, arg string) string {
	if x.coll != "" { // over the items of a data table, in Groovy: no variable sees them
		each := "(" + x.coll + " ?: []).collect{ r -> ((" + cmpOr(arg, "1") + ") as Double) }.findAll{ it != null }"
		switch fn {
		case "sum":
			return "(" + each + ".sum() ?: 0d)"
		case "avg":
			return "({ l -> l ? l.sum() / l.size() : null }.call(" + each + "))"
		case "min", "max":
			return "(" + each + "." + fn + "())"
		}
		return "((" + x.coll + " ?: []).size())"
	}
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

// groovyTerbilang is terbilang (expr.go) as a Groovy closure, so a .jrxml spells
// numbers out with nothing added to JasperReports: 2750000 is "dua juta tujuh ratus lima puluh ribu".
const groovyTerbilang = `{ Object v -> if (v == null || v == '') return null; double f = (v as Double); double n = Math.round(Math.abs(f)); ` +
	`if (n >= 1e18d) return String.valueOf(v); if (n == 0) return 'nol'; ` +
	`def small = ['', 'satu', 'dua', 'tiga', 'empat', 'lima', 'enam', 'tujuh', 'delapan', 'sembilan', 'sepuluh', 'sebelas']; ` +
	`def join = { String a, String b -> (a + ' ' + b).trim() }; def spell; spell = { long m -> ` +
	`if (m < 12) return small[(int) m]; if (m < 20) return small[(int) (m - 10)] + ' belas'; ` +
	`if (m < 100) return join(small[(int) m.intdiv(10)] + ' puluh', small[(int) (m % 10)]); ` +
	`if (m < 200) return join('seratus', spell(m - 100)); if (m < 1000) return join(small[(int) m.intdiv(100)] + ' ratus', spell(m % 100)); ` +
	`if (m < 2000) return join('seribu', spell(m - 1000)); ` +
	`for (u in [[1000000000000000L, 'kuadriliun'], [1000000000000L, 'triliun'], [1000000000L, 'miliar'], [1000000L, 'juta'], [1000L, 'ribu']]) { if (m >= u[0]) return join(spell(m.intdiv(u[0])) + ' ' + u[1], spell(m % u[0])) }; ` +
	`return '' }; (f < 0 ? 'minus ' : '') + spell((long) n) }`

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
			return "$V{MASTER_CURRENT_PAGE}", true
		case "pages":
			p.x.page = true
			return "$V{MASTER_TOTAL_PAGES}", true
		case "index":
			if p.x.list != nil { // the item's number in its list
				return "$V{REPORT_COUNT}", true
			}
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
	case word == "terbilang" && len(args) == 1:
		return "(" + groovyTerbilang + ").call(" + args[0] + ")", false
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

func property(name, value string) string {
	return `<property name="` + name + `" value="` + attr(value) + `"/>`
}

// whole is a property holding the element being written as the template has it,
// so FromJRXML reads a data table or sub-report back whole, not as its parts.
func (x *jasperOut) whole() string {
	var b bytes.Buffer
	if x.rawEl == nil || json.Compact(&b, x.rawEl) != nil {
		return ""
	}
	x.rawEl = nil // nothing inside it is the element
	return property("ddreport.element", b.String())
}

// corePDFFonts are the PDF names of the built-in fonts: regular, bold, italic, bold italic.
var corePDFFonts = map[string][4]string{
	"": {"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique"}, "Helvetica": {"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique"},
	"Times":   {"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic"},
	"Courier": {"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique"},
}

var jasperFont = map[string]string{"Helvetica": "SansSerif", "": "SansSerif", "Times": "Serif", "Courier": "Monospaced"}

// element writes one element of a band.
func (x *jasperOut) element(b *strings.Builder, e Element) {
	common := func(extra string) string {
		x.bottom = max(x.bottom, px(e.Y)+max(px(e.H), 1)) // a line is at least 1pt tall, so it may reach past the band
		x.right = max(x.right, px(e.X)+max(px(e.W), 1))
		if x.floatFrom > 0 && e.Y >= x.floatFrom-0.5 {
			extra = ` positionType="Float"` + extra
		}
		s := fmt.Sprintf(`<reportElement x="%d" y="%d" width="%d" height="%d"%s`, px(e.X), px(e.Y), max(px(e.W), 1), max(px(e.H), 1), extra)
		// properties FromJRXML reads back: a whole element, or that this is part of one
		inner := x.props
		x.props = ""
		if x.part {
			inner += `<property name="ddreport.part" value="1"/>`
		}
		if e.ShowIf != "" {
			if g, ok := x.groovy(e.ShowIf); ok {
				inner += `<printWhenExpression><![CDATA[` + cdata("(("+g+") ? Boolean.TRUE : Boolean.FALSE)") + `]]></printWhenExpression>`
			} else {
				x.warnOnce("a show-when condition Jasper cannot express is left out: " + e.ShowIf)
			}
		}
		if inner != "" {
			return s + ">" + inner + "</reportElement>"
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
		// Jasper prints nothing of a text whose first line does not fit, and measures a line
		// at about 1.3 times the font size: a box that cannot hold one line gets that much
		if h := math.Ceil(e.size() * 1.35); !e.Grow && h > e.H {
			x.props += property("ddreport.h", str(e.H)) // the height as designed, for FromJRXML
			e.H = h
		}
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
		// the built-in PDF fonts print bold and italic only by their PDF name
		faces, core := corePDFFonts[e.Font]
		pdfName := func(bold, italic bool) string {
			if !core {
				return ""
			}
			i := 0
			if bold {
				i++
			}
			if italic {
				i += 2
			}
			return ` pdfFontName="` + faces[i] + `"`
		}
		text := e.Text
		markup := ""
		if styleTag.MatchString(text) {
			markup = ` markup="styled"`
			text = strings.NewReplacer("<b>", `<style isBold="true"`+pdfName(true, e.Italic)+`>`, "<i>", `<style isItalic="true"`+pdfName(e.Bold, true)+`>`, "<u>", `<style isUnderline="true">`, "</b>", "</style>", "</i>", "</style>", "</u>", "</style>").Replace(text)
		}
		box := ""
		if e.Border > 0 {
			box = fmt.Sprintf(`<box><pen lineWidth="%g"/></box>`, e.Border)
		}
		// line spacing in points, as ours: Jasper's proportional spacing is taller, and it
		// prints nothing of a text whose first line does not fit its box
		textElement := fmt.Sprintf(`<textElement textAlignment="%s" verticalAlignment="%s"%s><font fontName="%s" size="%g" isBold="%t" isItalic="%t" isUnderline="%t"%s/><paragraph lineSpacing="Fixed" lineSpacingSize="%g"/></textElement>`,
			align, valign, markup, attr(font), e.size(), e.Bold, e.Italic, e.Underline, pdfName(e.Bold, e.Italic), e.size()*e.leading())
		el := common(colors(e.Color))
		if !strings.Contains(e.Text, "{{") {
			fmt.Fprintf(b, "\t\t\t<staticText>%s%s%s<text><![CDATA[%s]]></text></staticText>\n", el, box, textElement, cdata(text))
			return
		}
		x.page = false
		g, vars, _ := x.text(text)
		when := ""
		switch {
		case x.list != nil: // in a list each item's values are there as it prints: Jasper allows no delay
		case x.page: // "Page 2 of 5": both known when the report is done
			when = ` evaluationTime="Master"`
		case vars:
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
			x.dataTable(b, e)
			return
		}
		// cell by cell; the first one carries the whole table, so it reads back as one
		whole := x.whole()
		cells, _ := x.r.grid(e, scope{x.t, nil, nil}, false, true)
		if whole != "" {
			defer func() { x.part = false }()
		}
		for i, c := range cells {
			if whole != "" && i == 0 {
				x.props += whole
			}
			if whole != "" {
				x.part = i > 0
			}
			c.ShowIf = e.ShowIf
			if c.Type == "text" {
				c.Border = 0 // the table's lines are elements of their own
				c.Text = strings.Join(slices.Collect(func(yield func(string) bool) {
					for _, r := range c.parse() {
						yield(r.text)
					}
				}), "")
			}
			x.element(b, c)
		}
	case "list": // the box of a data table's jr:list, which dataTable writes the rest of
		fmt.Fprintf(b, "\t\t\t<componentElement>%s\n", common(""))
	case "chart":
		x.props = property("ddreport.label", e.Label) + property("ddreport.value", e.Value) // what FromJRXML reads back
		x.chart(b, e, common(""))
	case "subreport":
		x.subreport(b, e)
	}
}

// rows splits a table into the table of its rows from to to, where they are on the page.
func tableRows(e Element, from, to int, header, footer bool) Element {
	h := make([]float64, len(e.Cells))
	sum := 0.0
	for i := range h {
		h[i] = 1
		if i < len(e.Heights) && e.Heights[i] > 0 {
			h[i] = e.Heights[i]
		}
		sum += h[i]
	}
	for i := range h {
		h[i] *= e.H / sum
	}
	part := e
	part.Data, part.Header, part.Footer = "", header, footer
	part.Cells, part.Heights = e.Cells[from:to], h[from:to]
	part.Y, part.H = e.Y, 0
	for i := range from {
		part.Y += h[i]
	}
	for i := from; i < to; i++ {
		part.H += h[i]
	}
	part.CellStyle = map[string]CellStyle{}
	for k, s := range e.CellStyle { // "row,column", counted from the part's first row
		var r, c int
		if _, err := fmt.Sscanf(k, "%d,%d", &r, &c); err == nil && r >= from && r < to {
			part.CellStyle[fmt.Sprintf("%d,%d", r-from, c)] = s
		}
	}
	if !header && !footer {
		part.Frame = 0 // the frame is drawn around each item
	}
	return part
}

// dataTable writes a table whose rows repeat for each item of a list in the data:
// its header row, a jr:list repeating the rows between, and its total row under
// the list, moving down as it grows. The items are a parameter of the report (a
// java.util.List of Maps), or in a detail band a field of the row holding a JSON list.
func (x *jasperOut) dataTable(b *strings.Builder, e Element) {
	nr := len(e.Cells)
	first, last := 0, nr
	if e.Header {
		first = 1
	}
	if e.Footer && nr-1 >= first && (nr > 1 || !e.Header) {
		last = nr - 1
	}
	coll, whole := x.items(e.Data), x.whole()
	// header and total row: their aggregates count the items. With the list
	// carrying the whole table, they are its parts
	edge := func(from, to int, header, footer bool) {
		if from < to {
			x.coll, x.part = coll, whole != "" && first < last
			x.element(b, tableRows(e, from, to, header, footer))
			x.coll, x.part = "", false
		}
	}
	edge(0, first, true, false)
	if first < last {
		items := tableRows(e, first, last, false, false)
		top, height := items.Y, items.H
		items.X, items.Y = 0, 0
		x.repeat(b, e, coll, whole, top, height, func(inner *strings.Builder) { x.element(inner, items) })
	}
	edge(last, nr, false, true)
}

// items is the Groovy for a list in the data: a parameter of the report (a
// java.util.List of Maps), or in a band of rows a field of the row holding a JSON list.
func (x *jasperOut) items(path string) string {
	coll := x.field(path, false)
	if x.rowed {
		// Jackson comes with JasperReports, which reads JSON with it
		coll = "(new com.fasterxml.jackson.databind.ObjectMapper().readValue((" + coll + " ?: '[]') as String, java.util.List))"
	}
	return coll
}

// repeat writes a jr:list at top in e's box, printing what write writes once for
// each of the items, over a sub-dataset with a field for every key they use.
// whole is the property that reads the element back.
func (x *jasperOut) repeat(b *strings.Builder, e Element, coll, whole string, top, height float64, write func(*strings.Builder)) {
	x.nlists++
	name := fmt.Sprintf("list%d", x.nlists)
	outer, float, bottom, right := x.list, x.floatFrom, x.bottom, x.right
	x.list, x.floatFrom, x.bottom, x.right = map[string]string{}, 0, 0, 0
	var inner strings.Builder
	write(&inner)
	fields, contents, wide := x.list, max(px(math.Ceil(height)), x.bottom, 1), max(px(e.W), x.right)
	x.list, x.floatFrom, x.bottom, x.right = outer, float, bottom, right
	keys := slices.Sorted(maps.Keys(fields))
	var parents strings.Builder // what an item has not comes from around the list
	for _, f := range keys {
		fmt.Fprintf(&parents, `<datasetParameter name="parent_%s"><datasetParameterExpression><![CDATA[%s]]></datasetParameterExpression></datasetParameter>`,
			jasperName.ReplaceAllString(f, "_"), cdata(x.field(f, false)))
	}
	place := e
	place.Y, place.H, place.W = top, float64(contents), float64(wide)
	place.Type, place.ShowIf = "list", e.ShowIf
	fmt.Fprintf(&x.subsets, "\t<subDataset name=\"%s\">\n", name)
	for _, f := range keys {
		fmt.Fprintf(&x.subsets, "\t\t<parameter name=\"parent_%s\" class=\"java.lang.Object\"/>\n", jasperName.ReplaceAllString(f, "_"))
	}
	for _, f := range keys {
		fmt.Fprintf(&x.subsets, "\t\t<field name=\"%s\" class=\"java.lang.Object\"/>\n", attr(f))
	}
	x.subsets.WriteString("\t</subDataset>\n")
	x.props = whole
	x.element(b, place) // the list's box
	fmt.Fprintf(b, "\t\t\t\t<jr:list xmlns:jr=\"http://jasperreports.sourceforge.net/jasperreports/components\" xsi:schemaLocation=\"http://jasperreports.sourceforge.net/jasperreports/components http://jasperreports.sourceforge.net/xsd/components.xsd\" printOrder=\"Vertical\">"+
		"<datasetRun subDataset=\"%s\">%s<dataSourceExpression><![CDATA[%s]]></dataSourceExpression></datasetRun>"+
		"<jr:listContents height=\"%d\" width=\"%d\">\n%s\t\t\t\t</jr:listContents></jr:list></componentElement>\n",
		name, parents.String(), cdata("new net.sf.jasperreports.engine.data.JRMapCollectionDataSource((java.util.Collection) ("+coll+" ?: []))"), contents, wide, inner.String())
}

// subreport writes a sub-report in its box: its header and column header, its
// detail band repeated in a jr:list for each item, and its summary under that,
// moving down as the list grows. Aggregates in header and summary count the items.
func (x *jasperOut) subreport(b *strings.Builder, e Element) {
	rep := e.Report
	if rep == nil || e.Data == "" {
		x.warnOnce("a sub-report without a list (data) or a report is left as an empty box")
		e.Type, e.Border = "rect", 0.5
		x.element(b, e)
		return
	}
	if strings.TrimSpace(rep.GroupBy) != "" || strings.TrimSpace(rep.SortBy) != "" {
		x.warnOnce("a sub-report's groups and sorting are left out: its items print in the order of the data")
	}
	d := rep.Bands.Detail
	listed := d.Height > 0 || len(d.Elements) > 0
	coll, whole, y := x.items(e.Data), x.whole(), e.Y
	// a band of the sub-report, at y in the box; around the list, a part of the element it carries
	at := func(w *strings.Builder, band Band, dx, dy float64, part bool) {
		x.part = part && whole != "" && listed
		for _, el := range band.Elements {
			el.X += dx
			el.Y += dy
			x.element(w, el)
		}
		x.part = false
	}
	for _, band := range []Band{rep.Bands.Header, rep.Bands.ColumnHeader} {
		x.coll = coll
		at(b, band, e.X, y, true)
		x.coll = ""
		y += band.Height
	}
	if listed {
		x.repeat(b, e, coll, whole, y, d.Height, func(inner *strings.Builder) { at(inner, d, 0, 0, false) })
		y += d.Height
	}
	x.coll = coll
	at(b, rep.Bands.Summary, e.X, y, true)
	x.coll = ""
}

// chart writes a chart over the rows of its band: all of them, or its group's.
// Jasper's datasets keep the last value of a label where ours sum them, so a
// variable sums each label's values row by row, in the order the labels come,
// and the chart draws that once the rows are done. Charts need JFreeChart, as
// every JasperReports chart does.
func (x *jasperOut) chart(b *strings.Builder, e Element, box string) {
	if strings.TrimSpace(e.Label) == "" || strings.TrimSpace(e.Value) == "" { // nothing to draw yet: its box
		x.warnOnce("a chart without a label and value field is left as an empty box")
		fmt.Fprintf(b, "\t\t\t<rectangle>%s<graphicElement><pen lineWidth=\"0.5\" lineStyle=\"Dashed\"/></graphicElement></rectangle>\n", box)
		return
	}
	x.nvars++
	sums := fmt.Sprintf("chart%d", x.nvars)
	rowed := x.rowed
	x.rowed = true // label and value are fields of the rows
	label := x.field(e.Label, true)
	value, ok := x.groovy(e.Value)
	x.rowed = rowed
	if !ok {
		x.warnOnce("a chart value Jasper cannot express is drawn as nothing: " + e.Value)
		value = "null"
	}
	reset, when := ` resetType="Report"`, ` evaluationTime="Report"`
	if lv := x.level; lv > 0 || x.rowed && len(x.t.levels()) > 0 {
		if lv == 0 {
			lv = len(x.t.levels())
		}
		reset = fmt.Sprintf(` resetType="Group" resetGroup="g%d"`, lv)
		when = fmt.Sprintf(` evaluationTime="Group" evaluationGroup="g%d"`, lv)
	}
	fmt.Fprintf(&x.vars, "\t<variable name=\"%s\" class=\"java.util.Map\"%s calculation=\"Nothing\"><variableExpression><![CDATA[%s]]></variableExpression><initialValueExpression><![CDATA[new java.util.LinkedHashMap()]]></initialValueExpression></variable>\n",
		sums, reset, cdata("({ m, k, v -> m.put(k, ((m.get(k) ?: 0d) as Double) + v); m }).call($V{"+sums+"} ?: new java.util.LinkedHashMap(), "+
			label+" == null ? '' : String.valueOf("+label+"), (("+value+") as Double) ?: 0d)"))
	x.nlists++
	set := fmt.Sprintf("list%d", x.nlists)
	fmt.Fprintf(&x.subsets, "\t<subDataset name=\"%s\">\n\t\t<field name=\"label\" class=\"java.lang.Object\"/>\n\t\t<field name=\"value\" class=\"java.lang.Object\"/>\n\t</subDataset>\n", set)
	dataset := fmt.Sprintf(`<dataset%s><datasetRun subDataset="%s"><dataSourceExpression><![CDATA[%s]]></dataSourceExpression></datasetRun></dataset>`,
		reset, set, cdata("new net.sf.jasperreports.engine.data.JRMapCollectionDataSource($V{"+sums+"}.collect{ k, v -> ['label': k, 'value': v] })"))
	color := `<seriesColor seriesOrder="0" color="` + palette[0] + `"/>`
	slices := "" // a pie's slices, in the colours ours has
	for i, c := range palette {
		slices += fmt.Sprintf(`<seriesColor seriesOrder="%d" color="%s"/>`, i, c)
	}
	head := func(legend bool) string {
		return fmt.Sprintf(`<chart isShowLegend="%t"%s>%s</chart>`, legend, when, box)
	}
	series := `<categorySeries><seriesExpression><![CDATA["` + attr(e.Value) + `"]]></seriesExpression><categoryExpression><![CDATA[String.valueOf($F{label})]]></categoryExpression>` +
		`<valueExpression><![CDATA[($F{value} as Double)]]></valueExpression></categorySeries>`
	axes := `<categoryAxisFormat><axisFormat/></categoryAxisFormat><valueAxisFormat><axisFormat/></valueAxisFormat>`
	switch e.Chart {
	case "pie", "donut":
		if e.Chart == "donut" {
			x.warnOnce("a donut chart prints as a pie: JasperReports 6 has no donut")
		}
		fmt.Fprintf(b, "\t\t\t<pieChart>%s<pieDataset>%s<keyExpression><![CDATA[String.valueOf($F{label})]]></keyExpression>"+
			"<valueExpression><![CDATA[Math.max(($F{value} as Double), 0d)]]></valueExpression></pieDataset><piePlot><plot>%s</plot><itemLabel/></piePlot></pieChart>\n", head(true), dataset, slices)
	case "line":
		fmt.Fprintf(b, "\t\t\t<lineChart>%s<categoryDataset>%s%s</categoryDataset><linePlot><plot>%s</plot>%s</linePlot></lineChart>\n", head(false), dataset, series, color, axes)
	default:
		orientation := ""
		if e.Chart == "hbar" {
			orientation = ` orientation="Horizontal"`
		}
		fmt.Fprintf(b, "\t\t\t<barChart>%s<categoryDataset>%s%s</categoryDataset><barPlot><plot%s>%s</plot><itemLabel/>%s</barPlot></barChart>\n", head(false), dataset, series, orientation, color, axes)
	}
}

// band writes a band, as tall as what is in it.
func (x *jasperOut) band(b *strings.Builder, tag, key string, band Band, level int, rowed bool) {
	x.level, x.rowed, x.bottom, x.floatFrom = level, rowed, 0, 0
	// under a data table's first repeated row, things move down as its list grows
	for _, e := range band.Elements {
		if e.Type == "table" && e.Data != "" {
			first, last := 0, len(e.Cells)
			if e.Header {
				first = 1
			}
			if e.Footer && last-1 >= first {
				last--
			}
			if rows := tableRows(e, first, last, false, false); first < last && (x.floatFrom == 0 || rows.Y+rows.H < x.floatFrom) {
				x.floatFrom = rows.Y + rows.H
			}
		}
		if e.Type == "subreport" && e.Report != nil && e.Data != "" { // under its repeated detail band
			r := e.Report.Bands
			if end := e.Y + r.Header.Height + r.ColumnHeader.Height + r.Detail.Height; x.floatFrom == 0 || end < x.floatFrom {
				x.floatFrom = end
			}
		}
	}
	var els strings.Builder
	for i, e := range band.Elements {
		x.rawEl = nil
		if r := x.raw[key]; i < len(r) {
			x.rawEl = r[i]
		}
		x.element(&els, e)
	}
	x.rawEl = nil
	// as tall as designed, and as what is in it: Jasper refuses an element reaching outside its band
	h := max(px(math.Ceil(band.Height)), x.bottom)
	fmt.Fprintf(b, "\t<%s>\n\t\t<band height=\"%d\" splitType=\"Stretch\">\n%s\t\t</band>\n\t</%s>\n", tag, h, els.String(), tag)
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
		suffix := ""
		if i > 0 {
			suffix = strconv.Itoa(i + 1)
		}
		x.band(&gh, "groupHeader", "groupHeader"+suffix, l.header, i+1, true)
		x.band(&gf, "groupFooter", "groupFooter"+suffix, l.footer, i+1, true)
		groups.WriteString(gh.String() + gf.String() + "\t</group>\n")
	}

	// bands, in Jasper's order
	if bs.Header.Pages == "first" {
		x.band(&body, "title", "header", bs.Header, 0, false)
	} else if len(bs.Header.Elements) > 0 || bs.Header.Height > 0 {
		x.band(&body, "pageHeader", "header", bs.Header, 0, false)
	}
	x.band(&body, "columnHeader", "columnHeader", bs.ColumnHeader, 0, false)
	x.band(&body, "detail", "detail", bs.Detail, 0, true)
	if bs.Footer.Pages == "last" {
		x.band(&body, "lastPageFooter", "footer", bs.Footer, 0, false)
	} else {
		x.band(&body, "pageFooter", "footer", bs.Footer, 0, false)
	}
	x.band(&body, "summary", "summary", bs.Summary, 0, false)

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
	fmt.Fprintf(&out, `<jasperReport xmlns="http://jasperreports.sourceforge.net/jasperreports" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://jasperreports.sourceforge.net/jasperreports http://jasperreports.sourceforge.net/xsd/jasperreport.xsd" name="report" language="groovy" pageWidth="%d" pageHeight="%d" columnWidth="%d" leftMargin="%d" rightMargin="%d" topMargin="%d" bottomMargin="%d" whenNoDataType="AllSectionsNoDetail" isSummaryWithPageHeaderAndFooter="true"%s>`+"\n",
		px(t.Page.Width), px(t.Page.Height), px(t.Page.Width-left-right), px(left), px(right), px(top), px(bottom), summaryNewPage)
	// the template's settings that Jasper has no place for, such as the sorting before
	// the group fields were put first, and number separators: FromJRXML reads them back
	if x.settings != "" {
		out.WriteString("\t" + property("ddreport.settings", x.settings) + "\n")
	}
	out.WriteString(x.subsets.String())
	for _, name := range slices.Sorted(maps.Keys(x.params)) {
		// its path in the data, such as customer.name, for FromJRXML
		fmt.Fprintf(&out, "\t<parameter name=\"%s\" class=\"java.lang.Object\" isForPrompting=\"false\"><parameterDescription><![CDATA[%s]]></parameterDescription></parameter>\n", name, cdata(x.params[name]))
	}
	// the sort fields must be fields too. Jasper groups only rows that are next to each
	// other, where ours gathers the rows of a group wherever they are: sort by the group fields first
	var sorts strings.Builder
	var by []string
	for _, l := range levels {
		if l.by != "" {
			by = append(by, l.by)
		}
	}
	if len(by) > 0 {
		x.warnOnce("the rows are sorted by the group fields (" + strings.Join(by, ", ") + "), as Jasper groups only rows next to each other")
	}
	sorted := map[string]bool{}
	for _, s := range append(by, strings.Split(t.SortBy, ",")...) {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		order := ""
		if strings.HasPrefix(s, "-") {
			s, order = strings.TrimSpace(s[1:]), ` order="Descending"`
		}
		name := jasperName.ReplaceAllString(s, "_")
		if sorted[name] { // a group field also in sortBy: sorted once, in the group's place
			continue
		}
		sorted[name] = true
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
		fmt.Fprintf(&out, "\t<field name=\"%s\" class=\"java.lang.String\"><fieldDescription><![CDATA[%s]]></fieldDescription></field>\n", name, cdata(x.fields[name]))
	}
	out.WriteString(sorts.String())
	out.WriteString(x.vars.String())
	out.WriteString(groups.String())
	out.WriteString(body.String())
	out.WriteString("</jasperReport>\n")
	return []byte(out.String())
}
