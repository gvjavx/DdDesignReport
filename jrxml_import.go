package ddreport

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// FromJRXML reads a JasperReports report (.jrxml) as a template, for moving
// reports over from JasperReports. It returns the template JSON and notes on
// what did not come over as it was.
//
// Bands, groups (up to six, on a field), sorting, static texts, text fields,
// boxes, ellipses (as boxes), lines, frames (their contents, and their border
// as a box), barcodes and pictures embedded as Base64 come over, with fonts,
// colours, alignment and borders. A text field's expression comes over when it
// is made of text, fields ($F{} and $P{} become {{field}}), page numbers, row
// counts, Sum / Average / Lowest / Highest / Count variables of a field, and
// arithmetic on fields; its pattern becomes a number or date format. What is
// not, such as other Java, is kept as written in [brackets] and noted, as are
// charts, sub-reports, crosstabs, lists and pictures from files.
func FromJRXML(jrxml []byte) (template []byte, notes []string, err error) {
	var root xnode
	if err := xml.Unmarshal(jrxml, &root); err != nil {
		return nil, nil, fmt.Errorf("ddreport: jrxml: %w", err)
	}
	if root.XMLName.Local != "jasperReport" {
		return nil, nil, fmt.Errorf("ddreport: jrxml: not a JasperReports report (it starts with <%s>)", root.XMLName.Local)
	}
	in := &jasperIn{root: &root, fields: map[string]string{}, vars: map[string][2]string{}}
	for _, f := range root.all("field") {
		path := strings.TrimSpace(f.child("fieldDescription").Text)
		if path == "" || strings.ContainsAny(path, "$[]()") {
			path = f.attr("name")
		}
		in.fields[f.attr("name")] = path
	}
	for _, v := range root.all("variable") {
		in.vars[v.attr("name")] = [2]string{v.attr("calculation"), strings.TrimSpace(v.child("variableExpression").Text)}
	}
	// written as people read it: < and > stay as they are (showIf, <b>)
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err = enc.Encode(in.template())
	return b.Bytes(), in.notes, err
}

// xnode is any element of the XML, kept whole.
type xnode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Nodes   []xnode    `xml:",any"`
	Text    string     `xml:",chardata"`
}

func (n *xnode) attr(name string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (n *xnode) num(name string) float64 {
	f, _ := strconv.ParseFloat(n.attr(name), 64)
	return f
}

func (n *xnode) child(name string) *xnode {
	for i := range n.Nodes {
		if n.Nodes[i].XMLName.Local == name {
			return &n.Nodes[i]
		}
	}
	return &xnode{}
}

func (n *xnode) all(name string) []*xnode {
	var out []*xnode
	for i := range n.Nodes {
		if n.Nodes[i].XMLName.Local == name {
			out = append(out, &n.Nodes[i])
		}
	}
	return out
}

type jasperIn struct {
	root   *xnode
	fields map[string]string    // Jasper field: its path in a row
	vars   map[string][2]string // variable: calculation, expression
	notes  []string
	evalAt string // the evaluationTime of the text field being read
}

func (in *jasperIn) note(s string) {
	if !slices.Contains(in.notes, s) {
		in.notes = append(in.notes, s)
	}
}

type jsonMap = map[string]any

func (in *jasperIn) template() jsonMap {
	r := in.root
	page := jsonMap{
		"width": r.num("pageWidth"), "height": r.num("pageHeight"),
		"marginTop": r.num("topMargin"), "marginRight": r.num("rightMargin"), "marginBottom": r.num("bottomMargin"), "marginLeft": r.num("leftMargin"),
	}
	if page["width"] == 0.0 {
		page["width"], page["height"] = 595.0, 842.0
	}
	bands := jsonMap{}
	put := func(name string, tag string) *xnode {
		b := r.child(tag).child("band")
		if b.XMLName.Local == "" {
			return nil
		}
		bands[name] = in.band(b)
		return b
	}
	// title prints once on the first page, pageHeader on every page: our header is one band,
	// so with both, the title goes on top of it and shows on the first page only
	if r.child("pageHeader").child("band").XMLName.Local != "" {
		put("header", "pageHeader")
		if b := r.child("title").child("band"); b.XMLName.Local != "" {
			in.note("the title band is at the top of the header, shown on the first page only; on the next pages its space stays empty")
			title := in.band(b)
			for _, e := range title["elements"].([]any) {
				e.(jsonMap)["showIf"] = "page = 1"
			}
			bands["header"] = stack(title, bands["header"].(jsonMap))
		}
	} else if put("header", "title") != nil {
		bands["header"].(jsonMap)["pages"] = "first"
	}
	put("columnHeader", "columnHeader")
	// detail may be several bands: one after the other
	var detail jsonMap
	for _, b := range r.child("detail").all("band") {
		detail = stack(detail, in.band(b))
	}
	if detail != nil {
		bands["detail"] = detail
	}
	if put("footer", "pageFooter") == nil && put("footer", "lastPageFooter") != nil {
		bands["footer"].(jsonMap)["pages"] = "last"
	} else if r.child("lastPageFooter").child("band").XMLName.Local != "" {
		in.note("the last page footer is left out: the page footer prints on every page")
	}
	if r.child("columnFooter").child("band").XMLName.Local != "" {
		in.note("the column footer is left out: put what it shows in the summary or the footer")
	}
	summary := put("summary", "summary")
	if summary != nil && r.attr("isSummaryNewPage") == "true" {
		bands["summary"].(jsonMap)["newPage"] = true
	}
	for _, tag := range []string{"background", "noData"} {
		if r.child(tag).child("band").XMLName.Local != "" {
			in.note("the " + tag + " band is left out")
		}
	}

	out := jsonMap{"page": page, "dataKey": "rows", "bands": bands}
	var by []string
	for _, g := range r.all("group") {
		expr := strings.TrimSpace(g.child("groupExpression").Text)
		field, ok := in.fieldOf(expr)
		if !ok || len(by) >= 6 {
			in.note("a group on " + expr + " is left out: only groups on a field, up to six, come over")
			continue
		}
		by = append(by, field)
		suffix := ""
		if len(by) > 1 {
			suffix = strconv.Itoa(len(by))
		}
		if b := g.child("groupHeader").child("band"); b.XMLName.Local != "" {
			bands["groupHeader"+suffix] = in.band(b)
		}
		if b := g.child("groupFooter").child("band"); b.XMLName.Local != "" {
			bands["groupFooter"+suffix] = in.band(b)
		}
		if g.attr("isStartNewPage") == "true" && len(by) == 1 {
			out["groupPageBreak"] = true
		}
	}
	if len(by) > 0 {
		out["groupBy"] = strings.Join(by, ", ")
	}
	var sorts []string
	for _, s := range r.all("sortField") {
		name := in.path(s.attr("name"))
		if s.attr("order") == "Descending" {
			name = "-" + name
		}
		sorts = append(sorts, name)
	}
	if len(sorts) > 0 {
		out["sortBy"] = strings.Join(sorts, ", ")
	}
	return out
}

// stack puts band b under band a.
func stack(a, b jsonMap) jsonMap {
	if a == nil {
		return b
	}
	h := a["height"].(float64)
	els := a["elements"].([]any)
	for _, e := range b["elements"].([]any) {
		e.(jsonMap)["y"] = e.(jsonMap)["y"].(float64) + h
		els = append(els, e)
	}
	return jsonMap{"height": h + b["height"].(float64), "elements": els}
}

func (in *jasperIn) band(b *xnode) jsonMap {
	var els []any
	in.elements(b, 0, 0, &els)
	if els == nil {
		els = []any{}
	}
	return jsonMap{"height": b.num("height"), "elements": els}
}

// elements reads the elements of a band or frame, at an offset.
func (in *jasperIn) elements(parent *xnode, dx, dy float64, out *[]any) {
	for i := range parent.Nodes {
		n := &parent.Nodes[i]
		re := n.child("reportElement")
		x, y, w, h := re.num("x")+dx, re.num("y")+dy, re.num("width"), re.num("height")
		e := jsonMap{"x": x, "y": y, "w": w, "h": h}
		if c := re.attr("forecolor"); hexColor.MatchString(c) {
			e["color"] = strings.ToLower(c)
		}
		if c := re.attr("backcolor"); hexColor.MatchString(c) && re.attr("mode") == "Opaque" {
			e["fill"] = strings.ToLower(c)
		}
		if cond := strings.TrimSpace(re.child("printWhenExpression").Text); cond != "" {
			if s, ok := in.condition(cond); ok {
				e["showIf"] = s
			} else {
				in.note("a print-when expression is left out: " + cond)
			}
		}
		if pen := in.pen(n); pen > 0 {
			e["border"] = pen
		}
		switch n.XMLName.Local {
		case "staticText", "textField":
			e["type"] = "text"
			in.textStyle(n.child("textElement"), e)
			if n.XMLName.Local == "staticText" {
				e["text"] = in.markup(n.child("text").Text, n.child("textElement").attr("markup"))
			} else {
				in.evalAt = n.attr("evaluationTime")
				e["text"] = in.markup(in.expression(strings.TrimSpace(n.child("textFieldExpression").Text), n.attr("pattern")), n.child("textElement").attr("markup"))
				if n.attr("textAdjust") == "StretchHeight" || n.attr("isStretchWithOverflow") == "true" {
					e["grow"] = true
				}
			}
		case "rectangle", "ellipse":
			if n.XMLName.Local == "ellipse" {
				in.note("ellipses become boxes")
			}
			e["type"] = "rect"
			if _, ok := e["border"]; !ok {
				e["border"] = 1.0
				if pen := n.child("graphicElement").child("pen"); pen.attr("lineWidth") != "" {
					e["border"] = pen.num("lineWidth")
				}
			}
		case "line":
			e["type"] = "line"
			if pen := n.child("graphicElement").child("pen"); pen.attr("lineWidth") != "" {
				e["border"] = pen.num("lineWidth")
			} else if _, ok := e["border"]; !ok {
				e["border"] = 1.0
			}
			switch {
			case w > 1 && h > 1 && n.attr("direction") == "BottomUp":
				e["slope"] = "up"
			case w > 1 && h > 1:
				e["slope"] = "down"
			}
		case "image":
			e["type"] = "image"
			src := strings.TrimSpace(n.child("imageExpression").Text)
			if m := base64Literal.FindStringSubmatch(src); m != nil {
				e["src"] = "data:image/png;base64," + m[1]
				if strings.HasPrefix(m[1], "/9j/") {
					e["src"] = "data:image/jpeg;base64," + m[1]
				}
			} else if field, ok := in.fieldOf(base64Field.ReplaceAllString(src, "$1")); ok && base64Field.MatchString(src) {
				e["src"] = "{{" + field + "}}"
			} else {
				e["src"] = ""
				in.note("pictures from files or other expressions are left empty: pick them again in the designer (" + src + ")")
			}
		case "frame":
			// its contents, then its border as a box behind them
			if b, ok := e["border"]; ok || e["fill"] != nil {
				box := jsonMap{"type": "rect", "x": x, "y": y, "w": w, "h": h, "border": 0.0}
				if ok {
					box["border"] = b
				}
				if f := e["fill"]; f != nil {
					box["fill"] = f
				}
				*out = append(*out, box)
			}
			in.elements(n, x, y, out)
			continue
		case "componentElement":
			code := n.child("QRCode")
			e["format"] = "qr"
			if code.XMLName.Local == "" {
				code = n.child("Code128")
				e["format"] = "code128"
			}
			if code.XMLName.Local == "" {
				for _, c := range n.Nodes {
					if c.XMLName.Local != "reportElement" {
						in.note("the component " + c.XMLName.Local + " is left out")
					}
				}
				continue
			}
			e["type"] = "barcode"
			in.evalAt = ""
			e["text"] = in.expression(strings.TrimSpace(code.child("codeExpression").Text), "")
		case "reportElement", "property", "":
			continue
		default:
			in.note("the " + n.XMLName.Local + " elements are left out (charts, sub-reports, crosstabs, lists): add them again in the designer")
			continue
		}
		*out = append(*out, e)
	}
}

var (
	jasperStyle   = regexp.MustCompile(`<style([^>]*)>|</style>`)
	base64Literal = regexp.MustCompile(`decode\(\s*["']([A-Za-z0-9+/=\s]+)["']\s*\)`)
	base64Field   = regexp.MustCompile(`^.*decode\(\s*\(?\s*(\$[FP]\{[^}]+\})[\s\S]*$`)
	jasperRef     = regexp.MustCompile(`\$([FPV])\{([^}]+)\}`)
)

// pen is an element's border: its box's pen, as wide as the widest of its sides.
func (in *jasperIn) pen(n *xnode) float64 {
	box := n.child("box")
	w := box.child("pen").num("lineWidth")
	for _, side := range []string{"topPen", "leftPen", "bottomPen", "rightPen"} {
		w = max(w, box.child(side).num("lineWidth"))
	}
	return w
}

func (in *jasperIn) textStyle(te *xnode, e jsonMap) {
	if a := map[string]string{"Center": "center", "Right": "right", "Justified": "justify"}[te.attr("textAlignment")]; a != "" {
		e["align"] = a
	}
	if a := map[string]string{"Middle": "middle", "Bottom": "bottom"}[te.attr("verticalAlignment")]; a != "" {
		e["valign"] = a
	}
	font := te.child("font")
	if f := font.attr("fontName"); f != "" {
		switch f {
		case "SansSerif", "Arial", "Helvetica", "DejaVu Sans":
		case "Serif", "Times New Roman", "DejaVu Serif":
			e["font"] = "Times"
		case "Monospaced", "Courier New", "DejaVu Sans Mono":
			e["font"] = "Courier"
		default:
			e["font"] = f
		}
	}
	e["fontSize"] = 10.0
	if s := font.num("size"); s > 0 {
		e["fontSize"] = s
	}
	for attr, key := range map[string]string{"isBold": "bold", "isItalic": "italic", "isUnderline": "underline"} {
		if font.attr(attr) == "true" {
			e[key] = true
		}
	}
}

// markup turns Jasper's styled text into ours: bold, italic and underline.
func (in *jasperIn) markup(s, markup string) string {
	if markup != "styled" {
		return s
	}
	var out strings.Builder
	var open []string
	last := 0
	for _, m := range jasperStyle.FindAllStringSubmatchIndex(s, -1) {
		out.WriteString(s[last:m[0]])
		last = m[1]
		if s[m[0]+1] == '/' {
			if len(open) > 0 {
				out.WriteString(open[len(open)-1])
				open = open[:len(open)-1]
			}
			continue
		}
		attrs, tag := s[m[2]:m[3]], ""
		switch {
		case strings.Contains(attrs, `isBold="true"`):
			tag = "b"
		case strings.Contains(attrs, `isItalic="true"`):
			tag = "i"
		case strings.Contains(attrs, `isUnderline="true"`):
			tag = "u"
		}
		if tag != "" {
			out.WriteString("<" + tag + ">")
			open = append(open, "</"+tag+">")
		} else {
			open = append(open, "")
		}
	}
	out.WriteString(s[last:])
	return out.String()
}

// path is the path of a Jasper field in a row.
func (in *jasperIn) path(name string) string {
	if p, ok := in.fields[name]; ok {
		return p
	}
	return name
}

// fieldOf reads an expression that is a single field or parameter.
func (in *jasperIn) fieldOf(expr string) (string, bool) {
	m := jasperRef.FindStringSubmatch(strings.TrimSpace(expr))
	if m == nil || m[0] != strings.TrimSpace(expr) || m[1] == "V" {
		return "", false
	}
	return in.path(m[2]), true
}

// expression turns a text field's expression into a text with placeholders:
// literal strings joined with + to fields, variables and arithmetic.
func (in *jasperIn) expression(expr, pattern string) string {
	// a number pattern (#,##0.00) becomes its decimals, a date pattern stays as it is
	format := ""
	switch {
	case pattern == "":
	case strings.ContainsAny(pattern, "#0"):
		frac := ""
		if _, f, ok := strings.Cut(pattern, "."); ok {
			frac, _, _ = strings.Cut(f, ";")
		}
		format = ":" + strconv.Itoa(strings.Count(frac, "0")+strings.Count(frac, "#"))
	default:
		format = ":" + pattern
	}
	terms := splitTop(expr)
	hasString := slices.ContainsFunc(terms, func(t string) bool { return strings.HasPrefix(t, `"`) || strings.HasPrefix(t, `'`) })
	if !hasString { // no text in it: one value, perhaps a sum of fields
		return in.value(expr, format)
	}
	var out strings.Builder
	for _, t := range terms {
		if lit, ok := stringLiteral(t); ok {
			out.WriteString(lit)
			continue
		}
		out.WriteString(in.value(t, ""))
	}
	return out.String()
}

// value is one placeholder for a part of an expression that is not text.
func (in *jasperIn) value(expr, format string) string {
	expr = strings.TrimSpace(expr)
	for strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") && balanced(expr[1:len(expr)-1]) {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	if s, ok := in.ours(expr); ok {
		return "{{" + s + format + "}}"
	}
	in.note("expressions with Java that has no counterpart are kept as written, in [brackets]: " + expr)
	return "[" + expr + "]"
}

// ours is one of our expressions for a Jasper one: references, numbers, + - * / and parentheses.
func (in *jasperIn) ours(expr string) (string, bool) {
	ok := true
	s := jasperRef.ReplaceAllStringFunc(expr, func(ref string) string {
		m := jasperRef.FindStringSubmatch(ref)
		if m[1] != "V" {
			p := in.path(m[2])
			if !operandOnly.MatchString(p) {
				return "[" + p + "]"
			}
			return p
		}
		switch name := m[2]; {
		case name == "PAGE_NUMBER" && (in.evalAt == "Report"):
			return "pages"
		case name == "PAGE_NUMBER", name == "CURRENT_PAGE":
			return "page"
		case name == "REPORT_COUNT", strings.HasSuffix(name, "_COUNT"):
			return "index"
		default:
			v, known := in.vars[name]
			fn := map[string]string{"Sum": "sum", "Average": "avg", "Lowest": "min", "Highest": "max", "Count": "count"}[v[0]]
			if !known || fn == "" {
				ok = false
				return ref
			}
			arg, good := in.ours(v[1])
			if fn == "count" {
				return "count()"
			}
			if !good {
				ok = false
			}
			return fn + "(" + arg + ")"
		}
	})
	s = strings.ReplaceAll(s, "new java.util.Date()", "now")
	if !ok || !ourExpression.MatchString(s) {
		return "", false
	}
	for _, call := range jasperCall.FindAllStringSubmatch(s, -1) {
		if !slices.Contains([]string{"sum", "avg", "min", "max", "count"}, call[1]) {
			return "", false // a Java method such as toUpperCase()
		}
	}
	return strings.TrimSpace(s), true
}

var (
	operandOnly   = regexp.MustCompile(`^[\p{L}\p{N}_.]+$`)
	jasperCall    = regexp.MustCompile(`([\p{L}_][\p{L}\p{N}_.]*)\s*\(`)
	ourExpression = regexp.MustCompile(`^[\p{L}\p{N}_.\s()+\-*/\[\]]+$`)
)

// condition turns a print-when expression into a show-when condition, for the simple ones.
func (in *jasperIn) condition(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	for _, wrap := range []string{"new Boolean(", "Boolean.valueOf("} {
		if strings.HasPrefix(expr, wrap) && strings.HasSuffix(expr, ")") {
			expr = expr[len(wrap) : len(expr)-1]
		}
	}
	if strings.Contains(expr, "$V{") { // page numbers, counts and sums are not fields
		return "", false
	}
	s := strings.NewReplacer("==", "=", "&&", " and ", "||", " or ", `"`, "'").Replace(expr)
	s = jasperRef.ReplaceAllStringFunc(s, func(ref string) string { return in.path(jasperRef.FindStringSubmatch(ref)[2]) })
	if strings.ContainsAny(s, "$!?{};") || strings.Contains(s, ".equals") {
		return "", false
	}
	return s, true
}

// splitTop splits an expression at the + signs outside strings and parentheses.
func splitTop(expr string) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == '+' && depth == 0:
			out = append(out, strings.TrimSpace(expr[start:i]))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(expr[start:]))
}

func balanced(s string) bool {
	depth := 0
	for _, c := range s {
		if c == '(' {
			depth++
		} else if c == ')' {
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// stringLiteral reads "text" or 'text', with Java's escapes.
func stringLiteral(t string) (string, bool) {
	if len(t) < 2 || t[0] != t[len(t)-1] || t[0] != '"' && t[0] != '\'' {
		return "", false
	}
	return strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`, `\'`, "'", `\\`, `\`).Replace(t[1 : len(t)-1]), true
}
