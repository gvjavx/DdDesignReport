package ddreport

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// A placeholder is {{expression}} or {{expression:N}}, N being the number of
// decimals. An expression is a field path (name, customer.city), a number, an
// aggregate over the rows (sum, avg, min, max, count), or any of those
// combined with + - * / and parentheses: {{qty * price:2}}, {{sum(qty * price)}}.
// A field whose name is not a plain word goes in brackets: {{[unit-price] * qty}}.
var placeholder = regexp.MustCompile(`\{\{\s*([^{}:]+?)\s*(?::\s*(\d{1,2}))?\s*\}\}`)

const (
	maxDecimals = 12
	maxDepth    = 64 // nesting of parentheses, minus signs and function calls
)

// decimals reads the N of {{expression:N}}; -1 when there is none.
func decimals(s string) int {
	if s == "" {
		return -1
	}
	n, _ := strconv.Atoi(s)
	return min(n, maxDecimals)
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

var operand = regexp.MustCompile(`^[\w.]+`)

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
	switch p.peek() {
	case '(':
		p.pos++
		n := p.nested(p.sum)
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
		return func(s scope) any { return lookup(word, s.ctx...) }
	}
	p.pos++
	var arg node
	if p.peek() != ')' {
		arg = p.nested(p.sum)
	}
	p.closing()
	switch word {
	case "sum", "avg", "min", "max", "count":
		return aggregate(word, arg)
	}
	p.bad = true
	return none
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
		x, ok1 := a(s).(float64)
		y, ok2 := b(s).(float64)
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
			f, ok := arg(scope{s.t, nil, []map[string]any{row}}).(float64)
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
	n := p.sum()
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

// resolve fills in the placeholders of a text. Something in braces that is
// not an expression is left as it is.
func (s scope) resolve(text string) string {
	return placeholder.ReplaceAllStringFunc(text, func(p string) string {
		m := placeholder.FindStringSubmatch(p)
		v, ok := s.eval(m[1])
		if !ok {
			return p
		}
		if f, ok := v.(float64); ok && m[2] != "" {
			return s.t.number(f, decimals(m[2]))
		}
		return str(v)
	})
}

// cell is resolve for spreadsheets: a text that is exactly one numeric
// placeholder stays a number so it can be summed.
func (s scope) cell(e Element) cell {
	text := e.plain()
	bold := e.Bold || strings.HasPrefix(e.Text, "<b>") && strings.HasSuffix(e.Text, "</b>")
	if m := placeholder.FindStringSubmatch(text); m != nil && m[0] == text {
		if v, _ := s.eval(m[1]); v != nil {
			if f, ok := v.(float64); ok {
				return cell{f, bold, decimals(m[2])}
			}
		}
	}
	return cell{s.resolve(text), bold, -1}
}
