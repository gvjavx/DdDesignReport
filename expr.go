package ddreport

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A placeholder is {{expression}} or {{expression:format}}. The format is a
// number of decimals ({{price:2}}) or a date pattern ({{date:dd/MM/yyyy}}).
// An expression is a field path (name, customer.city), a number, a 'text', an
// aggregate over the rows (sum, avg, min, max, count), a function (if, upper,
// lower, round), or any of those combined with + - * /, comparisons
// (= != < > <= >=), and, or, not, and parentheses: {{qty * price:2}},
// {{if(paid, 'LUNAS', 'BELUM')}}. A field whose name is not a plain word goes
// in brackets: {{[unit-price] * qty}}. {{now}} is the time the report is made.
var placeholder = regexp.MustCompile(`\{\{\s*([^{}:]+?)\s*(?::([^{}]*?))?\s*\}\}`)

const (
	maxDecimals = 12
	maxDepth    = 64 // nesting of parentheses, minus signs and function calls
)

var (
	digits    = regexp.MustCompile(`^\d{1,2}$`)
	dateToken = regexp.MustCompile(`[yMdHms]|EEE`)
	numeric   = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
)

// decimals reads the N of {{expression:N}}; -1 when there is none.
func decimals(s string) int {
	if !digits.MatchString(s) {
		return -1
	}
	n, _ := strconv.Atoi(s)
	return min(n, maxDecimals)
}

// num reads a value as a number. Numbers sent as text ("18000", "1250.50", as
// many databases and JSON encoders send decimals) count as numbers too.
func num(v any) (float64, bool) {
	switch v := v.(type) {
	case float64:
		return v, true
	case string:
		s := strings.TrimSpace(v)
		if !numeric.MatchString(s) {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil && !math.IsInf(f, 0)
	}
	return 0, false
}

// truthy is how a condition reads a value: nothing, false, 0, "", "0" and "false" are false.
func truthy(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		s := strings.TrimSpace(v)
		return s != "" && s != "0" && !strings.EqualFold(s, "false")
	}
	return true
}

// scope is what a placeholder is evaluated against.
type scope struct {
	t    *Template
	rows []any            // rows aggregates run over
	ctx  []map[string]any // where fields are looked up, first match wins
}

// node is a parsed expression.
type node func(scope) any

func none(scope) any { return nil }

var operand = regexp.MustCompile(`^[\p{L}\p{N}_.]+`)

type parser struct {
	src   string
	pos   int
	depth int
	bad   bool
}

func (p *parser) peek() byte {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

// keyword consumes a word such as "and" when it stands on its own.
func (p *parser) keyword(w string) bool {
	p.peek()
	rest := p.src[p.pos:]
	if !strings.HasPrefix(rest, w) || len(operand.FindString(rest)) != len(w) {
		return false
	}
	p.pos += len(w)
	return true
}

// chain parses operands joined by the given operators, left to right.
func (p *parser) chain(ops string, next func() node) node {
	left := next()
	for !p.bad {
		op := p.peek()
		if op == 0 || !strings.ContainsRune(ops, rune(op)) {
			break
		}
		p.pos++
		left = binary(op, left, next())
	}
	return left
}

func (p *parser) expr() node {
	left := p.and()
	for !p.bad && p.keyword("or") {
		a, b := left, p.and()
		left = func(s scope) any { return truthy(a(s)) || truthy(b(s)) }
	}
	return left
}

func (p *parser) and() node {
	left := p.compare()
	for !p.bad && p.keyword("and") {
		a, b := left, p.compare()
		left = func(s scope) any { return truthy(a(s)) && truthy(b(s)) }
	}
	return left
}

// compare is a = b, a != b, a < b and the like: numbers when both sides are, text otherwise.
func (p *parser) compare() node {
	left := p.sum()
	if p.peek() == 0 || p.bad { // the end, or past it after an unclosed bracket
		return left
	}
	op := ""
	for _, o := range []string{"==", "!=", "<>", "<=", ">=", "=", "<", ">"} {
		if strings.HasPrefix(p.src[p.pos:], o) {
			op = o
			break
		}
	}
	if op == "" || p.bad {
		return left
	}
	p.pos += len(op)
	a, b := left, p.sum()
	return func(s scope) any {
		x, y := a(s), b(s)
		c := strings.Compare(str(x), str(y))
		f, okx := num(x)
		g, oky := num(y)
		if okx && oky {
			c = compareFloat(f, g)
		}
		switch op {
		case "=", "==":
			return c == 0
		case "!=", "<>":
			return c != 0
		case "<":
			return c < 0
		case ">":
			return c > 0
		case "<=":
			return c <= 0
		}
		return c >= 0
	}
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (p *parser) sum() node     { return p.chain("+-", p.product) }
func (p *parser) product() node { return p.chain("*/", p.factor) }

func (p *parser) closing() {
	if p.peek() != ')' {
		p.bad = true
	}
	p.pos++
}

// nested parses something that recurses, refusing expressions nested so deep
// that they could exhaust the stack.
func (p *parser) nested(parse func() node) node {
	if p.depth++; p.depth > maxDepth {
		p.bad = true
		return none
	}
	n := parse()
	p.depth--
	return n
}

func (p *parser) factor() node {
	switch c := p.peek(); c {
	case '(':
		p.pos++
		n := p.nested(p.expr)
		p.closing()
		return n
	case '-':
		p.pos++
		return binary('-', func(scope) any { return 0.0 }, p.nested(p.factor))
	case '[': // [any field name]
		end := strings.IndexByte(p.src[p.pos:], ']')
		if end < 0 {
			p.bad = true
			return none
		}
		key := p.src[p.pos+1 : p.pos+end]
		p.pos += end + 1
		return func(s scope) any { return field(key, s.ctx) }
	case '\'', '"': // 'text'
		end := strings.IndexByte(p.src[p.pos+1:], c)
		if end < 0 {
			p.bad = true
			return none
		}
		text := p.src[p.pos+1 : p.pos+1+end]
		p.pos += end + 2
		return func(scope) any { return text }
	}
	if p.keyword("not") {
		n := p.nested(p.factor)
		return func(s scope) any { return !truthy(n(s)) }
	}
	word := operand.FindString(p.src[p.pos:])
	p.pos += len(word)
	if word == "" {
		p.bad = true
		return none
	}
	if c := word[0]; c >= '0' && c <= '9' || c == '.' {
		if f, err := strconv.ParseFloat(word, 64); err == nil {
			return func(scope) any { return f }
		}
	}
	if p.peek() != '(' {
		return func(s scope) any {
			if v := lookup(word, s.ctx...); v != nil || word != "now" {
				return v
			}
			return s.t.now // {{now}}, unless the data has a field called now
		}
	}
	p.pos++
	var args []node
	for p.peek() != ')' && !p.bad {
		args = append(args, p.nested(p.expr))
		if p.peek() != ',' {
			break
		}
		p.pos++
	}
	p.closing()
	if p.bad {
		return none
	}
	if n := function(word, args); n != nil {
		return n
	}
	p.bad = true
	return none
}

// function builds a call; nil when there is no such function or the arguments do not fit it.
func function(name string, args []node) node {
	arg := func(i int) node {
		if i < len(args) {
			return args[i]
		}
		return none
	}
	switch {
	case len(args) <= 1 && (name == "sum" || name == "avg" || name == "min" || name == "max" || name == "count"):
		var a node
		if len(args) == 1 {
			a = args[0]
		}
		return aggregate(name, a)
	case name == "if" && (len(args) == 2 || len(args) == 3):
		c, yes, no := args[0], args[1], arg(2)
		return func(s scope) any {
			if truthy(c(s)) {
				return yes(s)
			}
			return no(s)
		}
	case (name == "upper" || name == "lower") && len(args) == 1:
		change := strings.ToUpper
		if name == "lower" {
			change = strings.ToLower
		}
		return func(s scope) any {
			if v := args[0](s); v != nil {
				return change(str(v))
			}
			return nil
		}
	case name == "terbilang" && len(args) == 1:
		return func(s scope) any {
			if f, ok := num(args[0](s)); ok {
				return terbilang(f)
			}
			return nil
		}
	case name == "round" && (len(args) == 1 || len(args) == 2):
		return func(s scope) any {
			f, ok := num(args[0](s))
			if !ok {
				return nil
			}
			n, _ := num(arg(1)(s))
			pow := math.Pow(10, math.Min(math.Max(math.Round(n), 0), maxDecimals))
			return math.Round(f*pow) / pow
		}
	}
	return nil
}

// field looks a key up as it is written, dots and all.
func field(key string, ctx []map[string]any) any {
	for _, c := range ctx {
		if v := c[key]; v != nil {
			return v
		}
	}
	return nil
}

// binary does arithmetic. Anything that is not a number, a division by zero
// and a result too large to be a number give no value rather than a wrong one.
func binary(op byte, a, b node) node {
	return func(s scope) any {
		x, ok1 := num(a(s))
		y, ok2 := num(b(s))
		if !ok1 || !ok2 || op == '/' && y == 0 {
			return nil
		}
		var v float64
		switch op {
		case '+':
			v = x + y
		case '-':
			v = x - y
		case '*':
			v = x * y
		default:
			v = x / y
		}
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return nil
		}
		return v
	}
}

// aggregate evaluates arg for every row of the scope and combines the numbers.
// The result only depends on the rows, so it is computed once per set of rows:
// a total used in every detail line does not make the report quadratic.
func aggregate(name string, arg node) node {
	type rowsKey struct {
		first *any
		n     int
	}
	memo := map[rowsKey]any{}
	return func(s scope) any {
		if arg == nil {
			if name == "count" {
				return float64(len(s.rows))
			}
			return nil
		}
		var key rowsKey
		if len(s.rows) > 0 {
			key = rowsKey{&s.rows[0], len(s.rows)}
		}
		if v, ok := memo[key]; ok {
			return v
		}
		var sum, lo, hi float64
		n := 0
		for _, r := range s.rows {
			row, _ := r.(map[string]any)
			f, ok := num(arg(scope{s.t, nil, []map[string]any{row}}))
			if !ok {
				continue
			}
			if n == 0 || f < lo {
				lo = f
			}
			if n == 0 || f > hi {
				hi = f
			}
			sum += f
			n++
		}
		var v any
		switch {
		case name == "count":
			v = float64(n)
		case name == "sum":
			v = sum
		case n == 0:
		case name == "min":
			v = lo
		case name == "max":
			v = hi
		default:
			v = sum / float64(n)
		}
		if f, ok := v.(float64); ok && (math.IsInf(f, 0) || math.IsNaN(f)) {
			v = nil
		}
		memo[key] = v
		return v
	}
}

// parse compiles an expression once per template; nil when it is not one.
func (t *Template) parse(expr string) node {
	if n, ok := t.nodes[expr]; ok {
		return n
	}
	p := parser{src: expr}
	n := p.expr()
	if p.bad || p.peek() != 0 {
		n = nil
	}
	if t.nodes == nil {
		t.nodes = map[string]node{}
	}
	t.nodes[expr] = n
	return n
}

// eval computes an expression. ok is false when it is not a valid expression.
func (s scope) eval(expr string) (v any, ok bool) {
	// A placeholder that is exactly a field name is that field, whatever the
	// name looks like: {{unit-price}} is not "unit minus price" when the data
	// has a field called unit-price.
	if v := field(expr, s.ctx); v != nil {
		return v, true
	}
	n := s.t.parse(expr)
	if n == nil {
		return nil, false
	}
	return n(s), true
}

// shows says whether an element prints: always, unless its ShowIf condition
// is false. A condition that is not an expression (a typo) shows the element,
// so the mistake is seen rather than the element quietly missing.
func (s scope) shows(e Element) bool {
	cond := strings.TrimSpace(e.ShowIf)
	cond = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(cond, "{{"), "}}"))
	if cond == "" {
		return true
	}
	v, ok := s.eval(cond)
	return !ok || truthy(v)
}

// format writes a value with the format of its placeholder. ok is false when
// the format is neither decimals nor a date pattern.
func (s scope) format(v any, spec string) (string, bool) {
	spec = strings.TrimSpace(spec)
	switch {
	case spec == "":
		return str(v), true
	case digits.MatchString(spec):
		if f, ok := num(v); ok {
			return s.t.number(f, decimals(spec)), true
		}
		return str(v), true
	case dateToken.MatchString(spec):
		if tm, ok := asTime(v); ok {
			return s.t.date(tm, spec), true
		}
		return str(v), true
	}
	return "", false
}

// resolve fills in the placeholders of a text. Something in braces that is
// not an expression is left as it is.
func (s scope) resolve(text string) string {
	return placeholder.ReplaceAllStringFunc(text, func(p string) string {
		m := placeholder.FindStringSubmatch(p)
		v, ok := s.eval(m[1])
		if !ok {
			return p
		}
		out, ok := s.format(v, m[2])
		if !ok {
			return p
		}
		return out
	})
}

// paint fills in a colour written as a placeholder, such as
// {{if(total < 0, '#dc2626', '#000000')}}. What does not come out as #rrggbb
// is ignored where the colour is used, as a mistyped colour is.
func (s scope) paint(c string) string {
	if !strings.Contains(c, "{{") {
		return c
	}
	return strings.TrimSpace(s.resolve(c))
}

// cell is resolve for spreadsheets: a text that is exactly one numeric
// placeholder stays a number so it can be summed. A field such as a code
// ("007") stays text unless its placeholder asks for decimals.
func (s scope) cell(e Element) cell {
	if e.own != nil { // a cell of a data table: filled in from its item
		s = *e.own
	}
	text := e.plain()
	bold := e.Bold || strings.HasPrefix(e.Text, "<b>") && strings.HasSuffix(e.Text, "</b>")
	fill, color := e.bg, s.paint(e.Color)
	if f := s.paint(e.Fill); fill == "" && hexColor.MatchString(f) {
		fill = f
	}
	ink := ""
	if hexColor.MatchString(color) && color != "#000000" {
		ink = color
	}
	align := ""
	if e.Align == "center" || e.Align == "right" {
		align = e.Align
	}
	c := cell{dec: -1, bold: bold, fill: fill, color: ink, align: align, sides: e.sides}
	if m := placeholder.FindStringSubmatch(text); m != nil && m[0] == text {
		v, _ := s.eval(m[1])
		spec := strings.TrimSpace(m[2])
		_, isNumber := v.(float64)
		if f, ok := num(v); ok && (isNumber || digits.MatchString(spec)) {
			c.v, c.dec = f, decimals(spec)
			if c.align == "right" { // where a spreadsheet puts a number anyway
				c.align = ""
			}
			return c
		}
	}
	c.v = s.resolve(text)
	return c
}

// asTime reads a date: a time, an ISO date text, or a Unix time in seconds or milliseconds.
func asTime(v any) (time.Time, bool) {
	switch v := v.(type) {
	case time.Time:
		return v, true
	case float64:
		if v > 1e11 {
			return time.UnixMilli(int64(v)), true
		}
		return time.Unix(int64(v), 0), true
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
			if tm, err := time.Parse(layout, strings.TrimSpace(v)); err == nil {
				return tm, true
			}
		}
	}
	return time.Time{}, false
}

var (
	monthsID = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	daysID   = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	pattern  = regexp.MustCompile(`'[^']*'|yyyy|yy|MMMM|MMM|MM|M|dd|d|HH|H|mm|ss|EEEE|EEE`)
)

// date writes a time with a pattern such as "dd MMMM yyyy HH:mm"; text in
// 'quotes' is kept. Month and day names follow the template's Locale.
func (t *Template) date(tm time.Time, spec string) string {
	month, day := tm.Month().String(), tm.Weekday().String()
	if t.Locale == "id" {
		month, day = monthsID[tm.Month()-1], daysID[tm.Weekday()]
	}
	two := func(n int) string { return strconv.Itoa(n/10) + strconv.Itoa(n%10) }
	return pattern.ReplaceAllStringFunc(spec, func(tok string) string {
		switch tok {
		case "yyyy":
			return strconv.Itoa(tm.Year())
		case "yy":
			return two(tm.Year() % 100)
		case "MMMM":
			return month
		case "MMM":
			return string([]rune(month)[:3])
		case "MM":
			return two(int(tm.Month()))
		case "M":
			return strconv.Itoa(int(tm.Month()))
		case "dd":
			return two(tm.Day())
		case "d":
			return strconv.Itoa(tm.Day())
		case "HH":
			return two(tm.Hour())
		case "H":
			return strconv.Itoa(tm.Hour())
		case "mm":
			return two(tm.Minute())
		case "ss":
			return two(tm.Second())
		case "EEEE":
			return day
		case "EEE":
			return string([]rune(day)[:3])
		}
		return strings.Trim(tok, "'")
	})
}

// terbilang spells a number in Indonesian words, as a receipt does: 2750000 is
// "dua juta tujuh ratus lima puluh ribu". Decimals are rounded off; numbers
// from 10^18 on, past what is ever written out, stay digits.
func terbilang(f float64) string {
	n := math.Round(math.Abs(f))
	if n >= 1e18 || math.IsNaN(n) {
		return str(f)
	}
	if n == 0 {
		return "nol"
	}
	small := []string{"", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh", "sebelas"}
	join := func(a, b string) string { return strings.TrimSpace(a + " " + b) }
	var spell func(n int64) string
	spell = func(n int64) string {
		switch {
		case n < 12:
			return small[n]
		case n < 20:
			return small[n-10] + " belas"
		case n < 100:
			return join(small[n/10]+" puluh", small[n%10])
		case n < 200:
			return join("seratus", spell(n-100))
		case n < 1000:
			return join(small[n/100]+" ratus", spell(n%100))
		case n < 2000:
			return join("seribu", spell(n-1000))
		}
		for _, u := range []struct {
			v    int64
			name string
		}{{1e15, "kuadriliun"}, {1e12, "triliun"}, {1e9, "miliar"}, {1e6, "juta"}, {1e3, "ribu"}} {
			if n >= u.v {
				return join(spell(n/u.v)+" "+u.name, spell(n%u.v))
			}
		}
		return ""
	}
	if f < 0 {
		return "minus " + spell(int64(n))
	}
	return spell(int64(n))
}
