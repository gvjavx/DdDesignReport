// Package ddreport renders report templates made in the drag-and-drop designer
// (JSON) to PDF, HTML, XLSX or CSV.
package ddreport

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"codeberg.org/go-pdf/fpdf"
)

// Font and FontBold replace the built-in PDF font, which only covers Latin
// (cp1252), with TrueType data. Set them once at startup when reports contain
// other scripts. FontBold falls back to Font.
var Font, FontBold []byte

// Template is the JSON document produced by the designer. All sizes are in points.
type Template struct {
	Page           Page
	DataKey        string // key of the row array in the data, default "rows"
	SortBy         string // row fields to sort by, "-" for descending: "category, -total"
	GroupBy        string // row field; rows with the same value form a group
	GroupPageBreak bool   // every group starts on a new page
	Thousands      string // separators used by {{field:2}}, default "," and "."; "none" for no thousands separator
	Decimal        string
	Locale         string // "id" for Indonesian month and day names in dates, English otherwise
	CSVSeparator   string // , (default), ; or a tab
	Bands          struct{ Header, ColumnHeader, GroupHeader, Detail, GroupFooter, Summary, Footer Band }

	nodes map[string]node // expressions, parsed once
	now   time.Time       // {{now}}, the same on every page
}

type Page struct {
	Width, Height                                    float64
	Margin                                           float64 // all four sides, unless set per side
	MarginTop, MarginRight, MarginBottom, MarginLeft float64
	Background                                       string // #rrggbb
	BackgroundImage                                  string // data URI, stretched over the page
}

func (p Page) margins() (top, right, bottom, left float64) {
	side := func(v float64) float64 {
		if v > 0 {
			return v
		}
		return p.Margin
	}
	return side(p.MarginTop), side(p.MarginRight), side(p.MarginBottom), side(p.MarginLeft)
}

// Band is a horizontal strip of the report. Header, ColumnHeader and Footer
// repeat on every page, Detail repeats per data row, GroupHeader and
// GroupFooter surround each group (and, without GroupBy, all the rows as one
// group), Summary follows the last row. A group header is repeated at the top
// of a page its group continues on.
type Band struct {
	Height   float64
	Elements []Element
	Pages    string // header, column header, footer: "first" for the first page only, "last" (footer) for the last
	NewPage  bool   // group header, detail, group footer, summary: start on a new page
}

type Element struct {
	Type       string // text | rect | line | image | chart | table | barcode
	X, Y, W, H float64
	// Text may contain {{field}}, {{a.b}}, {{page}}, {{pages}}, {{now}}, the
	// aggregates {{sum(f)}}, {{avg(f)}}, {{min(f)}}, {{max(f)}}, {{count()}},
	// which run over the group inside group and detail bands and over all rows
	// elsewhere, and the expressions described in expr.go. ":N" formats a
	// number with N decimals ({{sum(price):2}}), a pattern formats a date
	// ({{date:dd MMMM yyyy}}). Parts of it can be styled with <b>, <i> and <u>.
	Text                    string
	ShowIf                  string // condition such as paid = 'yes'; when false the element is not printed
	Font                    string // Helvetica (default) | Times | Courier | a name in Families
	FontSize                float64
	LineHeight              float64 // multiple of the font size, default 1.2
	Bold, Italic, Underline bool
	Grow                    bool   // text: the band grows with the wrapped text instead of clipping it
	Align                   string // left | center | right | justify
	VAlign                  string // text: top (default) | middle | bottom
	Color                   string // #rrggbb, text or stroke colour
	Fill                    string // #rrggbb, background
	Border                  float64
	Src                     string // image: PNG, JPEG or GIF data URI, or a placeholder such as {{photo}} whose value is one
	Slope                   string // line: "down" from top left to bottom right, "up" from bottom left to top right; otherwise flat or upright
	ShowText                bool   // barcode: print its text under it

	Chart        string // bar | hbar | line | pie | donut
	Label, Value string // chart: row fields; values are summed per label

	Format string // barcode: qr (default) | code128; its content is Text

	Cells     [][]string // table: cell texts, row by row
	Cols      []float64  // table: relative column widths, default equal
	Aligns    []string   // table: alignment per column
	TextColor string     // table: colour of the body text; Color is the colour of its lines
	// table: the first row is a header and the last a total row, both bold,
	// each with its own background and text colour
	Header, Footer          bool
	HeaderFill, HeaderColor string
	FooterFill, FooterColor string
	Stripe                  string               // table: background of every second body row
	Grid                    string               // table: which lines a border draws: all (default) | outer | rows
	Data                    string               // table: field with a list; the rows between header and total row repeat for each item
	Heights                 []float64            // table: relative row heights, default equal
	Padding                 *float64             // table: room around a cell's text, default 3pt across and 2pt down
	CellStyle               map[string]CellStyle // table: the look and merging of single cells, keyed "row,column" from 0

	Points []float64 // internal: x,y pairs of a "poly" (charts), x,y,w,h of the dark runs of a "code" (barcodes); relative to X,Y
	runs   []run     // text split into styled runs, placeholders resolved
	own    *scope    // a table cell of a data table: the scope of its item
	unit   int       // table rows with the same unit are kept on one page
	head   bool      // a table header row, repeated on the next page when its table goes on there
	tbl    int       // the table a line, background or cell belongs to
	bg     string    // a table cell's background, for the spreadsheets
}

// run is a stretch of text in one style.
type run struct {
	text                    string
	bold, italic, underline bool
}

var (
	styleTag  = regexp.MustCompile(`</?[biu]>`)
	escapedLt = regexp.MustCompile(`&lt;(/?[biu]>)`) // how a literal "<b>" is written
)

// parse splits Text at its <b>, <i> and <u> tags.
func (e Element) parse() []run {
	var out []run
	depth := map[byte]int{}
	add := func(s string) {
		if s = escapedLt.ReplaceAllString(s, "<$1"); s != "" {
			out = append(out, run{s, e.Bold || depth['b'] > 0, e.Italic || depth['i'] > 0, e.Underline || depth['u'] > 0})
		}
	}
	pos := 0
	for _, m := range styleTag.FindAllStringIndex(e.Text, -1) {
		add(e.Text[pos:m[0]])
		if tag := e.Text[m[0]:m[1]]; tag[1] == '/' {
			depth[tag[2]]--
		} else {
			depth[tag[1]]++
		}
		pos = m[1]
	}
	add(e.Text[pos:])
	return out
}

// plain is Text without the style tags.
func (e Element) plain() string {
	return escapedLt.ReplaceAllString(styleTag.ReplaceAllString(e.Text, ""), "<$1")
}

// pieces are the runs to draw. Texts generated by charts have no runs: their
// Text is data and is not parsed for tags.
func (e Element) pieces() []run {
	if e.runs != nil {
		return e.runs
	}
	return []run{{e.Text, e.Bold, e.Italic, e.Underline}}
}

// Render fills template with data and returns the report in the given format:
// "pdf", "html", "xlsx", "xls" or "csv". data is anything that marshals to a JSON
// object (fields plus a row array under DataKey) or to a JSON array of rows.
func Render(template []byte, data any, format string) ([]byte, error) {
	r, err := newReport(template, data)
	if err != nil {
		return nil, err
	}
	t := r.t
	switch format {
	case "csv":
		cells, _ := r.table()
		return toCSV(cells, t.CSVSeparator)
	case "xlsx":
		return toXLSX(r.table())
	case "xls":
		return toXLS(r.table())
	case "html", "pdf":
		if err := t.fits(); err != nil {
			return nil, err
		}
		if err := r.initPDF(); err != nil {
			return nil, fmt.Errorf("ddreport: font: %w", err)
		}
		// first pass only counts the pages, for {{pages}}
		pages := r.paginate(len(r.paginate(0)))
		if format == "html" {
			return r.toHTML(pages), nil
		}
		return r.toPDF(pages)
	}
	return nil, fmt.Errorf("ddreport: unknown format %q", format)
}

// grouped says whether the group bands print: with GroupBy, or when they have
// something in them (then all the rows are one group).
func (t *Template) grouped() bool {
	b := t.Bands
	return t.GroupBy != "" || len(b.GroupHeader.Elements) > 0 || len(b.GroupFooter.Elements) > 0
}

// fits checks that the fixed bands leave room for the tallest flowing band,
// and says by how much they do not.
func (t *Template) fits() error {
	b := t.Bands
	top, _, bottom, _ := t.Page.margins()
	tallest, name := b.Detail.Height, "detail"
	for _, f := range []struct {
		band Band
		name string
		on   bool
	}{{b.Summary, "summary", true}, {b.GroupHeader, "groupHeader", t.grouped()}, {b.GroupFooter, "groupFooter", t.grouped()}} {
		if f.on && f.band.Height > tallest {
			tallest, name = f.band.Height, f.name
		}
	}
	need := top + bottom + b.Header.Height + b.ColumnHeader.Height + tallest + b.Footer.Height
	if need <= t.Page.Height {
		return nil
	}
	return fmt.Errorf("ddreport: bands do not fit on the page: margins %g + header %g + columnHeader %g + %s %g + footer %g = %gpt, the page is %gpt tall; make a band lower or the page taller",
		top+bottom, b.Header.Height, b.ColumnHeader.Height, name, tallest, b.Footer.Height, need, t.Page.Height)
}

// newReport parses the template, applies its defaults and normalises the data.
func newReport(template []byte, data any) (*report, error) {
	var t Template
	if err := json.Unmarshal(template, &t); err != nil {
		return nil, fmt.Errorf("ddreport: template: %w", err)
	}
	if t.Page.Width <= 0 || t.Page.Height <= 0 {
		t.Page.Width, t.Page.Height = 595, 842 // A4
	}
	if t.DataKey == "" {
		t.DataKey = "rows"
	}
	if t.Decimal == "" {
		t.Decimal = "."
		if t.Thousands == "" {
			t.Thousands = ","
		}
	}
	if t.Thousands == "none" {
		t.Thousands = ""
	}
	t.now = time.Now()
	bands := &t.Bands
	images := []string{t.Page.BackgroundImage}
	for _, b := range []*Band{&bands.Header, &bands.ColumnHeader, &bands.GroupHeader, &bands.Detail, &bands.GroupFooter, &bands.Summary, &bands.Footer} {
		for _, e := range b.Elements {
			if e.Type == "image" {
				images = append(images, e.Src)
			}
		}
	}
	for _, src := range images {
		// a placeholder ({{photo}}) is checked once it is filled in, and drawn only if it is a picture
		if src != "" && !strings.Contains(src, "{{") && !dataImage.MatchString(src) {
			return nil, errors.New("ddreport: image: src must be a PNG, JPEG or GIF data URI (data:image/png;base64,...)")
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("ddreport: data: %w", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("ddreport: data: %w", err)
	}
	r := &report{t: &t}
	r.root, _ = v.(map[string]any)
	var ok bool
	if r.rows, ok = v.([]any); !ok {
		r.rows, _ = r.root[t.DataKey].([]any)
		if _, has := r.root[t.DataKey]; !has && len(bands.Detail.Elements) > 0 {
			// the detail band waits for rows that are under another name: say so instead of printing none
			for k, x := range r.root {
				if _, list := x.([]any); list {
					return nil, fmt.Errorf("ddreport: data: no %q list for the detail band, but there is %q; set the rows key (dataKey) to %q", t.DataKey, k, k)
				}
			}
		}
	}
	t.sort(r.rows)
	return r, nil
}

// sort orders the rows by SortBy: numbers as numbers, the rest as text, rows
// without the field last. Rows that compare equal keep their order.
func (t *Template) sort(rows []any) {
	var keys []string
	for _, k := range strings.Split(t.SortBy, ",") {
		if k = strings.TrimSpace(k); k != "" && k != "-" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return
	}
	slices.SortStableFunc(rows, func(a, b any) int {
		ra, _ := a.(map[string]any)
		rb, _ := b.(map[string]any)
		for _, k := range keys {
			desc := strings.HasPrefix(k, "-")
			k = strings.TrimPrefix(k, "-")
			x, y := lookup(k, ra), lookup(k, rb)
			var c int
			switch {
			case x == nil && y == nil:
			case x == nil:
				c = 1
			case y == nil:
				c = -1
			default:
				f, okx := num(x)
				g, oky := num(y)
				if c = strings.Compare(str(x), str(y)); okx && oky {
					c = compareFloat(f, g)
				}
				if desc {
					c = -c
				}
			}
			if c != 0 {
				return c
			}
		}
		return 0
	})
}

type report struct {
	t        *Template
	root     map[string]any
	rows     []any
	pdf      *fpdf.Fpdf // the PDF being written; also measures text for both PDF and HTML
	family   string     // Helvetica, or "custom" when Font is set
	familyTr func(string) string
	tr       func(string) string // UTF-8 to the current PDF font's encoding
	added    map[string]bool     // faces from Families already in the PDF
	tables   int                 // numbers the tables laid out, so their rows know which table they belong to
	units    int                 // numbers table rows that stay together on a page
}

func (r *report) initPDF() error {
	p := r.t.Page
	r.pdf = fpdf.NewCustom(&fpdf.InitType{UnitStr: "pt", Size: fpdf.SizeType{Wd: p.Width, Ht: p.Height}})
	r.pdf.SetMargins(0, 0, 0)
	r.pdf.SetAutoPageBreak(false, 0)
	r.pdf.SetCellMargin(0)
	r.added = map[string]bool{}
	latin := r.pdf.UnicodeTranslatorFromDescriptor("")
	r.family, r.familyTr = "Helvetica", func(s string) string {
		var b strings.Builder
		for _, c := range s {
			if out := latin(string(c)); c < 128 || out != "." {
				b.WriteString(out)
			} else {
				b.WriteByte('?')
			}
		}
		return b.String()
	}
	if Font != nil {
		bold := FontBold
		if bold == nil {
			bold = Font
		}
		r.pdf.AddUTF8FontFromBytes("custom", "", Font)
		r.pdf.AddUTF8FontFromBytes("custom", "B", bold)
		r.family, r.familyTr = "custom", func(s string) string { return s }
	}
	r.tr = r.familyTr
	return r.pdf.Error()
}

func (r *report) font(e Element, p run) {
	// a family from Families, its faces added to the PDF the first time they are used
	if f := Families[e.Font]; f != nil && f.Regular != nil {
		style := ""
		if p.bold {
			style += "B"
		}
		if p.italic {
			style += "I"
		}
		name := "f" + strings.ReplaceAll(e.Font, " ", "")
		if !r.added[name+style] {
			r.added[name+style] = true
			r.pdf.AddUTF8FontFromBytes(name, style, f.face(p.bold, p.italic))
		}
		if p.underline {
			style += "U"
		}
		r.pdf.SetFont(name, style, e.size())
		r.tr = func(s string) string { return s }
		return
	}
	family, style := r.family, ""
	r.tr = r.familyTr
	if Font == nil && (e.Font == "Times" || e.Font == "Courier") {
		family = e.Font
	}
	if p.bold {
		style += "B"
	}
	if p.italic && Font == nil { // a custom font comes without an italic face
		style += "I"
	}
	if p.underline {
		style += "U"
	}
	r.pdf.SetFont(family, style, e.size())
}

// span is a word or a stretch of spaces, measured.
type span struct {
	run
	w float64
}

type textLine struct {
	spans []span
	w     float64
	end   bool // the last line of a paragraph, which justified text leaves as it is
}

// softBreak breaks a line without ending its paragraph; split uses it to carry
// the lines of a text over to the next page exactly as they were.
const softBreak = " "

var token = regexp.MustCompile(`\n|\x{2028}|[^\S\n]+|[^\s\x{2028}]+`)

// chop cuts a word that is wider than the box (a URL, a long id) into pieces that fit.
func (r *report) chop(word string, w float64) []string {
	if w <= 0 || r.pdf.GetStringWidth(r.tr(word)) <= w {
		return []string{word}
	}
	var out []string
	piece := ""
	for _, c := range word {
		if piece != "" && r.pdf.GetStringWidth(r.tr(piece+string(c))) > w {
			out, piece = append(out, piece), ""
		}
		piece += string(c)
	}
	return append(out, piece)
}

// layout wraps the runs of a text element into lines no wider than e.W.
func (r *report) layout(e Element) []textLine {
	lines := []textLine{{}}
	var space *span // spaces waiting for the next word; dropped at a line break
	for _, p := range e.pieces() {
		r.font(e, p)
		measured := func(s string) span {
			return span{run{s, p.bold, p.italic, p.underline}, r.pdf.GetStringWidth(r.tr(s))}
		}
		for _, tok := range token.FindAllString(p.text, -1) {
			if tok == "\n" || tok == softBreak {
				lines[len(lines)-1].end = tok == "\n"
				lines, space = append(lines, textLine{}), nil
				continue
			}
			if strings.TrimSpace(tok) == "" {
				s := measured(tok)
				space = &s
				continue
			}
			for i, part := range r.chop(tok, e.W) {
				s := measured(part)
				cur := &lines[len(lines)-1]
				// a line breaks at a space, or inside a word too wide for the box;
				// "foo<b>bar</b>" stays together
				if len(cur.spans) > 0 && (i > 0 || space != nil && cur.w+space.w+s.w > e.W) {
					lines, space = append(lines, textLine{}), nil
					cur = &lines[len(lines)-1]
				}
				if space != nil {
					cur.spans, cur.w, space = append(cur.spans, *space), cur.w+space.w, nil
				}
				cur.spans, cur.w = append(cur.spans, s), cur.w+s.w
			}
		}
	}
	lines[len(lines)-1].end = true
	return lines
}

// repeatHeads puts the header rows of a table that goes on over the cut at
// the top of what continues, so every page of a long table has its headings.
func (r *report) repeatHeads(now, rest []Element, restH float64) ([]Element, float64) {
	heads := map[int][]Element{}
	for _, e := range now {
		if e.head {
			heads[e.tbl] = append(heads[e.tbl], e)
		}
	}
	var copies []Element
	room := 0.0
	done := map[int]bool{}
	for _, e := range rest {
		if e.tbl == 0 || e.head || done[e.tbl] || heads[e.tbl] == nil {
			continue
		}
		done[e.tbl] = true
		top, bottom := math.Inf(1), math.Inf(-1)
		for _, x := range heads[e.tbl] {
			top, bottom = min(top, x.Y), max(bottom, x.Y+x.H)
		}
		for _, x := range heads[e.tbl] {
			x.Y += room - top
			copies = append(copies, x)
		}
		room += bottom - top
	}
	if room == 0 {
		return rest, restH
	}
	for i := range rest {
		rest[i].Y += room
	}
	return append(copies, rest...), restH + room
}

// carry turns lines back into runs that lay out as the same lines.
func carry(lines []textLine) []run {
	out := []run{}
	for i, l := range lines {
		if i > 0 {
			brk := softBreak
			if lines[i-1].end {
				brk = "\n"
			}
			out = append(out, run{text: brk})
		}
		for _, s := range l.spans {
			out = append(out, s.run)
		}
	}
	return out
}

// split cuts a placed band at height cut: what is above stays, what is below
// moves up to start the next page, and a text or a frame that crosses the cut
// is divided, line by line for a text. It returns the part that continues and
// its height.
func (r *report) split(els []Element, h, cut float64) (now, rest []Element, restH float64) {
	// a table row is never cut through: the cut moves up to the top of the
	// first row it would cross (and again, as a merged cell may reach higher)
	unitTop := map[int]float64{}
	for _, e := range els {
		if e.unit != 0 {
			if top, ok := unitTop[e.unit]; !ok || e.Y < top {
				unitTop[e.unit] = e.Y
			}
		}
	}
	for moved := true; moved; {
		moved = false
		for _, e := range els {
			if e.unit != 0 && e.Y < cut && e.Y+e.H > cut && unitTop[e.unit] < cut {
				cut, moved = unitTop[e.unit], true
			}
		}
	}
	if cut <= 0 {
		return nil, els, h
	}
	restH = h - cut
	for _, e := range els {
		switch {
		case e.Y+e.H <= cut:
			now = append(now, e)
			continue
		case e.Y >= cut:
			e.Y -= cut
		case e.Type == "text":
			lines, lh := r.layout(e), e.size()*e.leading()
			n := min(max(int((cut-e.Y)/lh), 0), len(lines))
			if n > 0 {
				top := e
				top.runs, top.H = carry(lines[:n]), float64(n)*lh
				now = append(now, top)
			}
			if n == len(lines) {
				continue
			}
			e.runs, e.Y, e.H = carry(lines[n:]), 0, float64(len(lines)-n)*lh
		case e.Type == "rect" || e.Type == "line" && e.H > e.W && e.Slope == "":
			top := e
			top.H = cut - e.Y
			now = append(now, top)
			e.Y, e.H = 0, e.Y+e.H-cut
		default: // a picture, a chart, a barcode: whole, on the next page
			e.Y = 0
		}
		rest = append(rest, e)
		restH = max(restH, e.Y+e.H)
	}
	rest, restH = r.repeatHeads(now, rest, restH)
	return now, rest, restH
}

// paginate lays the bands out on pages.
func (r *report) paginate(total int) [][]Element {
	t := r.t
	p, b := t.Page, t.Bands
	mTop, _, mBottom, mLeft := p.margins()
	var pages [][]Element
	var y, top, bottom float64
	var group func() // repeats the header of the group under way at the top of a new page
	put := func(els []Element, at float64) {
		for i := range els {
			els[i].X += mLeft
			els[i].Y += at
		}
		pages[len(pages)-1] = append(pages[len(pages)-1], els...)
	}
	// on says whether a band prints on the current page: a band marked "first"
	// prints on the first page only, one marked "last" (footer) on the last
	on := func(band Band, last bool) bool {
		n := len(pages)
		return band.Pages != "first" && band.Pages != "last" || band.Pages == "first" && n == 1 || band.Pages == "last" && last && n == total
	}
	newPage := func() {
		var back []Element
		if hexColor.MatchString(p.Background) {
			back = append(back, Element{Type: "rect", W: p.Width, H: p.Height, Fill: p.Background})
		}
		if p.BackgroundImage != "" {
			back = append(back, Element{Type: "image", W: p.Width, H: p.Height, Src: p.BackgroundImage})
		}
		pages = append(pages, back)
		sc := scope{t, r.rows, []map[string]any{{"page": float64(len(pages)), "pages": float64(total)}, r.root}}
		y = mTop
		for _, band := range []Band{b.Header, b.ColumnHeader} {
			if on(band, false) {
				els, h := r.place(band, sc)
				put(els, y)
				y += h
			}
		}
		// the footer sits on the bottom margin, as high as its text grows; its room
		// is kept on every page, also when it prints on the last one only
		els, h := r.place(b.Footer, sc)
		bottom = p.Height - mBottom - max(h, b.Footer.Height)
		if on(b.Footer, true) {
			put(els, bottom)
		}
		top = y
		if group != nil {
			group()
		}
	}
	flow := func(band Band, rows []any, ctx ...map[string]any) {
		els, h := r.place(band, scope{t, rows, ctx})
		// a band moves to a new page whole, unless it is taller than a page
		if y > top && (band.NewPage || y+h > bottom && h <= bottom-top) {
			newPage()
		}
		// one that is taller (a long letter) fills the page and goes on over the next ones
		for y+h > bottom {
			now, rest, restH := r.split(els, h, bottom-y)
			if len(now) == 0 {
				if y == top {
					break // not even one line fits on an empty page: placed as it is
				}
				newPage()
				continue
			}
			put(now, y)
			newPage()
			els, h = rest, restH
		}
		put(els, y)
		y += h
	}
	newPage()
	for i, g := range t.groups(r.rows) {
		var first map[string]any
		if t.grouped() {
			// a group header never stays behind alone at the bottom of a page
			if t.GroupPageBreak && i > 0 || y > top && y+b.GroupHeader.Height+b.Detail.Height > bottom {
				newPage()
			}
			if len(g) > 0 {
				first, _ = g[0].(map[string]any)
			}
			flow(b.GroupHeader, g, first, r.root)
			if len(b.GroupHeader.Elements) > 0 {
				group = func() {
					els, h := r.place(b.GroupHeader, scope{t, g, []map[string]any{first, r.root}})
					put(els, y)
					y += h
				}
			}
		}
		for _, row := range g {
			m, _ := row.(map[string]any)
			flow(b.Detail, g, m, r.root)
		}
		if t.grouped() {
			flow(b.GroupFooter, g, first, r.root)
		}
		group = nil
	}
	if len(b.Summary.Elements) > 0 {
		flow(b.Summary, r.rows, r.root)
	}
	return pages
}

// groups splits the rows by the GroupBy field, groups in order of first appearance.
func (t *Template) groups(rows []any) [][]any {
	if t.GroupBy == "" {
		return [][]any{rows}
	}
	var out [][]any
	index := map[string]int{}
	for _, r := range rows {
		row, _ := r.(map[string]any)
		key := str(lookup(t.GroupBy, row))
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], r)
	}
	return out
}

// number formats f with dec decimals and the template's separators.
func (t *Template) number(f float64, dec int) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return ""
	}
	pow := math.Pow(10, float64(dec))
	f = math.Round(f*pow) / pow // halves round away from zero (2.5 -> 3); FormatFloat alone rounds them to even
	digits := strconv.FormatFloat(math.Abs(f), 'f', dec, 64)
	whole, frac, _ := strings.Cut(digits, ".")
	var b strings.Builder
	if f < 0 && strings.Trim(digits, "0.") != "" { // no "-0.00"
		b.WriteByte('-')
	}
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteString(t.Thousands)
		}
		b.WriteRune(c)
	}
	if frac != "" {
		b.WriteString(t.Decimal + frac)
	}
	return b.String()
}

// lookup finds a dotted path in the first context that has it.
func lookup(path string, ctx ...map[string]any) any {
	for _, c := range ctx {
		var v any = c
		for _, k := range strings.Split(path, ".") {
			m, _ := v.(map[string]any)
			v = m[k]
		}
		if v != nil {
			return v
		}
	}
	return nil
}

func str(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return ""
		}
		// 15 significant digits, so binary leftovers do not show: 0.1 + 0.2 is 0.3
		v, _ = strconv.ParseFloat(strconv.FormatFloat(v, 'g', 15, 64), 64)
		return strconv.FormatFloat(v, 'f', -1, 64)
	case time.Time:
		return v.Format("2006-01-02 15:04")
	}
	return fmt.Sprint(v)
}

var palette = []string{"#4e79a7", "#f28e2b", "#e15759", "#76b7b2", "#59a14f", "#edc948", "#b07aa1", "#9c755f"}

// chart turns a chart element into the rects, lines, texts and polys that draw it.
// Bars, lines and hbars get a value scale that includes zero, so negative values
// reach below it; a pie or donut cannot show them and draws them as zero.
func chart(t *Template, e Element, rows []any) []Element {
	var labels []string
	sums := map[string]float64{}
	for _, r := range rows {
		row, _ := r.(map[string]any)
		l := str(lookup(e.Label, row))
		if _, ok := sums[l]; !ok {
			labels = append(labels, l)
		}
		v, _ := (scope{t, nil, []map[string]any{row}}).eval(e.Value) // a field or an expression such as qty * price
		f, _ := num(v)
		sums[l] += f
	}
	if len(labels) == 0 {
		return nil
	}
	num := func(f float64) string { // whole numbers without decimals, the rest with two
		if f == math.Trunc(f) {
			return t.number(f, 0)
		}
		return t.number(f, 2)
	}
	text := func(x, y, w float64, s, align string) Element {
		return Element{Type: "text", X: e.X + x, Y: e.Y + y, W: w, H: 9, Text: s, FontSize: 7, Align: align}
	}
	var out []Element

	if e.Chart == "pie" || e.Chart == "donut" {
		// the legend gets as many lines as fit next to the pie; the smallest
		// slices beyond that are summed into one, so it never runs off the chart
		if fit := max(int(e.H/11), 1); len(labels) > fit {
			byValue := slices.Clone(labels)
			slices.SortStableFunc(byValue, func(a, b string) int { return compareFloat(sums[b], sums[a]) })
			keep := map[string]bool{}
			for _, l := range byValue[:fit-1] {
				keep[l] = true
			}
			other := "Other"
			if t.Locale == "id" {
				other = "Lainnya"
			}
			var kept []string
			rest := 0.0
			for _, l := range labels {
				if keep[l] {
					kept = append(kept, l)
				} else {
					rest += sums[l]
				}
			}
			labels, sums[other] = append(kept, other), sums[other]+rest
		}
		total := 0.0
		for _, l := range labels {
			total += max(sums[l], 0)
		}
		// the pie takes at most 55% of the width, so the legend always has room beside it
		rad, angle := min(e.H, e.W*0.55)/2, -math.Pi/2
		for i, l := range labels {
			c := palette[i%len(palette)]
			if total > 0 {
				sweep := 2 * math.Pi * max(sums[l], 0) / total
				steps := int(sweep/0.1) + 1
				arc := func(radius float64, k int) (float64, float64) {
					a := angle + sweep*float64(k)/float64(steps)
					return rad + radius*math.Cos(a), rad + radius*math.Sin(a)
				}
				pts := []float64{rad, rad} // a pie slice starts at the centre
				if e.Chart == "donut" {    // a donut slice comes back along the inner edge
					pts = nil
					for k := steps; k >= 0; k-- {
						x, y := arc(rad*0.55, k)
						pts = append(pts, x, y)
					}
				}
				for k := 0; k <= steps; k++ {
					x, y := arc(rad, k)
					pts = append(pts, x, y)
				}
				angle += sweep
				out = append(out, Element{Type: "poly", X: e.X, Y: e.Y, W: e.W, H: e.H, Points: pts, Fill: c})
			}
			y := float64(i) * 11
			out = append(out,
				Element{Type: "rect", X: e.X + 2*rad + 8, Y: e.Y + y + 1, W: 7, H: 7, Fill: c},
				text(2*rad+18, y, e.W-2*rad-18, l+": "+num(sums[l]), "left"))
		}
		return out
	}

	lo, hi := 0.0, 0.0 // the scale always includes zero
	for _, v := range sums {
		lo, hi = min(lo, v), max(hi, v)
	}
	step := tickStep(hi - lo)
	lo, hi = math.Floor(lo/step)*step, math.Ceil(hi/step)*step
	if hi == lo {
		hi = lo + step
	}
	var ticks []float64
	for i := 0; i <= int(math.Round((hi-lo)/step)); i++ {
		ticks = append(ticks, lo+float64(i)*step)
	}
	// ponytail: label widths are estimated (7pt, ~3.9pt per character) rather than
	// measured with the PDF font, so long labels in a wide font can still touch.
	wide := func(s string) float64 { return float64(utf8.RuneCountInString(s)) * 3.9 }
	grid := func(x, y, w, h float64, c string) Element {
		return Element{Type: "line", X: e.X + x, Y: e.Y + y, W: w, H: h, Border: 0.5, Color: c}
	}
	c := color(e.Color, palette[0])
	n := float64(len(labels))

	if e.Chart == "hbar" { // label, bar, value on each line; the value scale runs along the bottom
		labelW, valueW := min(e.W*0.3, 90), 50.0
		plotW, slot := e.W-labelW-valueW, (e.H-10)/n
		x := func(v float64) float64 { return labelW + plotW*(v-lo)/(hi-lo) }
		for _, v := range ticks {
			out = append(out, grid(x(v), 0, 0, e.H-10, "#dddddd"), text(x(v)-25, e.H-9, 50, num(v), "center"))
		}
		every := int(math.Ceil(9 / slot)) // rows thinner than a line of text are labelled every few bars
		for i, l := range labels {
			v, y := sums[l], slot*float64(i)
			out = append(out, Element{Type: "rect", X: e.X + x(min(v, 0)), Y: e.Y + y + slot*0.15, W: x(max(v, 0)) - x(min(v, 0)), H: slot * 0.7, Fill: c})
			if i%every == 0 {
				out = append(out, text(0, y+slot/2-4.5, labelW-4, l, "right"), text(x(max(v, 0))+3, y+slot/2-4.5, valueW-3, num(v), "left"))
			}
		}
		return append(out, grid(x(0), 0, 0, e.H-10, "#888888"))
	}

	axisW := 0.0 // tick labels on the left
	for _, v := range ticks {
		axisW = max(axisW, wide(num(v))+4)
	}
	plot, slot := e.H-20, (e.W-axisW)/n // 10pt above for values, 10pt below for labels
	y := func(v float64) float64 { return 10 + plot*(hi-v)/(hi-lo) }
	for _, v := range ticks {
		out = append(out, grid(axisW, y(v), e.W-axisW, 0, "#dddddd"), text(0, y(v)-4.5, axisW-4, num(v), "right"))
	}
	widest := 0.0
	for _, l := range labels {
		widest = max(widest, wide(l), wide(num(sums[l])))
	}
	every := int(math.Ceil((widest + 2) / slot)) // labels wider than their slot show every few bars
	var pts []float64
	for i, l := range labels {
		v, x := sums[l], axisW+slot*float64(i)
		at := y(v) // the value goes above its point, or above the zero line where a negative bar leaves room
		if e.Chart == "line" {
			pts = append(pts, x+slot/2, y(v))
		} else {
			at = y(max(v, 0))
			out = append(out, Element{Type: "rect", X: e.X + x + slot*0.15, Y: e.Y + at, W: slot * 0.7, H: y(min(v, 0)) - at, Fill: c})
		}
		if i%every == 0 {
			out = append(out, text(x, at-9, slot, num(v), "center"), text(x, e.H-9, slot, l, "center"))
		}
	}
	if e.Chart == "line" {
		out = append(out, Element{Type: "poly", X: e.X, Y: e.Y, W: e.W, H: e.H, Points: pts, Color: c, Border: 1.5})
		for i := 0; i+1 < len(pts); i += 2 { // a dot on every value, so a single one shows too
			out = append(out, Element{Type: "rect", X: e.X + pts[i] - 2, Y: e.Y + pts[i+1] - 2, W: 4, H: 4, Fill: c})
		}
	}
	return append(out, grid(axisW, y(0), e.W-axisW, 0, "#888888"))
}

// tickStep is a round step (1, 2 or 5 times a power of ten) that cuts span into about four.
func tickStep(span float64) float64 {
	raw := span / 4
	if raw <= 0 {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5} {
		if m*mag >= raw {
			return m * mag
		}
	}
	return 10 * mag
}

var dataImage = regexp.MustCompile(`^data:image/(png|jpe?g|gif);base64,`) // what the PDF can draw

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// color only lets #rrggbb through: it ends up inside a style attribute.
func color(c, def string) string {
	if hexColor.MatchString(c) {
		return c
	}
	return def
}

func rgb(c string) (int, int, int) {
	n, _ := strconv.ParseUint(color(c, "#000000")[1:], 16, 32)
	return int(n >> 16), int(n >> 8 & 255), int(n & 255)
}

func (e Element) size() float64 {
	if e.FontSize > 0 {
		return e.FontSize
	}
	return 10
}

func (e Element) align() string {
	if e.Align == "center" || e.Align == "right" {
		return e.Align
	}
	return "left"
}

func (e Element) leading() float64 {
	if e.LineHeight > 0 {
		return e.LineHeight
	}
	return 1.2
}

func (e Element) stroke() float64 {
	if e.Border > 0 {
		return e.Border
	}
	return 1
}

// drop is how far the lines of a text start below its top: middle and bottom
// alignment, as far as the box is taller than the lines.
func (e Element) drop(lines int) float64 {
	room := e.H - float64(lines)*e.size()*e.leading()
	switch {
	case room <= 0:
		return 0
	case e.VAlign == "middle":
		return room / 2
	case e.VAlign == "bottom":
		return room
	}
	return 0
}

// gap is the room justified text adds to each space of a line so it reaches
// the right edge; the last line of a paragraph keeps its natural spacing.
func (e Element) gap(l textLine) float64 {
	if e.Align != "justify" || l.end {
		return 0
	}
	spaces := 0
	for _, s := range l.spans {
		spaces += strings.Count(s.text, " ")
	}
	if spaces == 0 {
		return 0
	}
	return max(e.W-l.w, 0) / float64(spaces)
}

func (r *report) toHTML(pages [][]Element) []byte {
	var b, head strings.Builder
	type face struct {
		name         string
		bold, italic bool
	}
	var faces []face // faces of Families the report uses, embedded in its style sheet
	fmt.Fprintf(&head, `<!doctype html><html><head><meta charset="utf-8"><style>`+
		`@page{size:%[1]gpt %[2]gpt;margin:0}body{margin:0;font:10pt/1.2 Helvetica,Arial,sans-serif}`+
		`.p{position:relative;width:%[1]gpt;height:%[2]gpt;overflow:hidden;page-break-after:always}`+
		`.p>*{position:absolute;box-sizing:border-box;overflow:hidden;white-space:pre}`, r.t.Page.Width, r.t.Page.Height)
	for _, page := range pages {
		b.WriteString(`<div class="p">`)
		for _, e := range page {
			style := fmt.Sprintf("left:%gpt;top:%gpt;width:%gpt;height:%gpt;", e.X, e.Y, e.W, e.H)
			stroke := color(e.Color, "#000000")
			switch e.Type {
			case "text":
				style += fmt.Sprintf("font-size:%gpt;line-height:%g;text-align:%s;color:%s;", e.size(), e.leading(), e.align(), stroke)
				switch e.Font {
				case "Times":
					style += "font-family:'Times New Roman',Times,serif;"
				case "Courier":
					style += "font-family:'Courier New',Courier,monospace;"
				}
				f := Families[e.Font]
				if f == nil || f.Regular == nil || !safeName.MatchString(e.Font) {
					f = nil
				} else {
					style += "font-family:'" + e.Font + "',sans-serif;"
				}
				if hexColor.MatchString(e.Fill) {
					style += "background:" + e.Fill + ";"
				}
				lines := r.layout(e)
				if d := e.drop(len(lines)); d > 0 {
					style += fmt.Sprintf("padding-top:%gpt;", d)
				}
				fmt.Fprintf(&b, `<div style="%s">`, style)
				// the lines are broken where the PDF breaks them, so a browser font that
				// runs wider cannot add a line and clip the last one of a grown text
				for i, line := range lines {
					if i > 0 {
						b.WriteString("\n")
					}
					gap := e.gap(line) // justified: the same spacing as in the PDF
					if gap > 0 {
						fmt.Fprintf(&b, `<span style="word-spacing:%.3gpt">`, gap)
					}
					var runs []run // the line's words, joined back into one run per style
					for _, s := range line.spans {
						if k := len(runs) - 1; k >= 0 && runs[k].bold == s.bold && runs[k].italic == s.italic && runs[k].underline == s.underline {
							runs[k].text += s.text
						} else {
							runs = append(runs, s.run)
						}
					}
					for _, p := range runs {
						if fc := (face{e.Font, p.bold, p.italic}); f != nil && !slices.Contains(faces, fc) {
							faces = append(faces, fc)
						}
						open, end := "", ""
						for _, tag := range []struct {
							on   bool
							name string
						}{{p.bold, "b"}, {p.italic, "i"}, {p.underline, "u"}} {
							if tag.on {
								open, end = open+"<"+tag.name+">", "</"+tag.name+">"+end
							}
						}
						b.WriteString(open + html.EscapeString(p.text) + end)
					}
					if gap > 0 {
						b.WriteString(`</span>`)
					}
				}
				b.WriteString(`</div>`)
			case "rect":
				if e.Border > 0 {
					style += fmt.Sprintf("border:%gpt solid %s;", e.Border, stroke)
				}
				fmt.Fprintf(&b, `<div style="%sbackground:%s"></div>`, style, color(e.Fill, "none"))
			case "line":
				if e.Slope != "" {
					y1, y2 := 0.0, e.H
					if e.Slope == "up" {
						y1, y2 = e.H, 0
					}
					fmt.Fprintf(&b, `<svg style="%soverflow:visible" viewBox="0 0 %g %g"><line x1="0" y1="%g" x2="%g" y2="%g" stroke="%s" stroke-width="%g"/></svg>`,
						style, e.W, e.H, y1, e.W, y2, stroke, e.stroke())
					continue
				}
				side := "top"
				if e.H > e.W {
					side = "left"
				}
				fmt.Fprintf(&b, `<div style="%sborder-%s:%gpt solid %s"></div>`, style, side, e.stroke(), stroke)
			case "image":
				if strings.HasPrefix(e.Src, "data:image/") {
					fmt.Fprintf(&b, `<img style="%s" src="%s">`, style, html.EscapeString(e.Src))
				}
			case "code":
				var d strings.Builder
				for i := 0; i+3 < len(e.Points); i += 4 {
					fmt.Fprintf(&d, "M%.5g %.5gh%.5gv%.5gh%.5gz", e.Points[i], e.Points[i+1], e.Points[i+2], e.Points[i+3], -e.Points[i+2])
				}
				fmt.Fprintf(&b, "<svg style=\"%s\" viewBox=\"0 0 %g %g\"><path fill=\"%s\" d=\"%s\"/></svg>", style, e.W, e.H, stroke, d.String())
			case "poly":
				shape, paint := "polygon", fmt.Sprintf(`fill="%s"`, color(e.Fill, "none"))
				if !hexColor.MatchString(e.Fill) {
					shape, paint = "polyline", fmt.Sprintf(`fill="none" stroke="%s" stroke-width="%g"`, stroke, e.stroke())
				}
				fmt.Fprintf(&b, `<svg style="%s" viewBox="0 0 %g %g"><%s %s points="%s"/></svg>`,
					style, e.W, e.H, shape, paint, strings.Trim(fmt.Sprint(e.Points), "[]"))
			}
		}
		b.WriteString(`</div>`)
	}
	for _, fc := range faces {
		weight, slant := "normal", "normal"
		if fc.bold {
			weight = "bold"
		}
		if fc.italic {
			slant = "italic"
		}
		fmt.Fprintf(&head, `@font-face{font-family:'%s';font-weight:%s;font-style:%s;font-display:swap;src:url(data:font/ttf;base64,%s)}`,
			fc.name, weight, slant, base64.StdEncoding.EncodeToString(Families[fc.name].face(fc.bold, fc.italic)))
	}
	head.WriteString(`</style></head><body>`)
	b.WriteString(`</body></html>`)
	return []byte(head.String() + b.String())
}

func (r *report) toPDF(pages [][]Element) ([]byte, error) {
	pdf := r.pdf
	for _, page := range pages {
		pdf.AddPage()
		for _, e := range page {
			switch e.Type {
			case "text":
				if hexColor.MatchString(e.Fill) {
					pdf.SetFillColor(rgb(e.Fill))
					pdf.Rect(e.X, e.Y, e.W, e.H, "F")
				}
				pdf.SetTextColor(rgb(e.Color))
				pdf.ClipRect(e.X, e.Y, e.W, e.H, false)
				lh := e.size() * e.leading()
				lines := r.layout(e)
				drop := e.drop(len(lines))
				for i, line := range lines {
					x := e.X
					switch e.align() {
					case "center":
						x += (e.W - line.w) / 2
					case "right":
						x += e.W - line.w
					}
					base := e.Y + drop + float64(i)*lh + lh/2 + 0.3*e.size()
					gap := e.gap(line)
					for _, s := range line.spans {
						r.font(e, s.run)
						pdf.Text(x, base, r.tr(s.text))
						x += s.w + gap*float64(strings.Count(s.text, " "))
					}
				}
				pdf.ClipEnd()
			case "rect":
				style := ""
				if hexColor.MatchString(e.Fill) {
					pdf.SetFillColor(rgb(e.Fill))
					style = "F"
				}
				if e.Border > 0 {
					pdf.SetDrawColor(rgb(e.Color))
					pdf.SetLineWidth(e.Border)
					style = "D" + style
				}
				if style != "" {
					pdf.Rect(e.X, e.Y, e.W, e.H, style)
				}
			case "line":
				pdf.SetDrawColor(rgb(e.Color))
				pdf.SetLineWidth(e.stroke())
				switch {
				case e.Slope == "down":
					pdf.Line(e.X, e.Y, e.X+e.W, e.Y+e.H)
				case e.Slope == "up":
					pdf.Line(e.X, e.Y+e.H, e.X+e.W, e.Y)
				case e.H > e.W:
					pdf.Line(e.X, e.Y, e.X, e.Y+e.H)
				default:
					pdf.Line(e.X, e.Y, e.X+e.W, e.Y)
				}
			case "code":
				pdf.SetFillColor(rgb(e.Color))
				for i := 0; i+3 < len(e.Points); i += 4 {
					pdf.Rect(e.X+e.Points[i], e.Y+e.Points[i+1], e.Points[i+2], e.Points[i+3], "F")
				}
			case "poly":
				var pts []fpdf.PointType
				for i := 0; i+1 < len(e.Points); i += 2 {
					pts = append(pts, fpdf.PointType{X: e.X + e.Points[i], Y: e.Y + e.Points[i+1]})
				}
				if hexColor.MatchString(e.Fill) {
					pdf.SetFillColor(rgb(e.Fill))
					pdf.Polygon(pts, "F")
					continue
				}
				pdf.SetDrawColor(rgb(e.Color))
				pdf.SetLineWidth(e.stroke())
				for i := 1; i < len(pts); i++ {
					pdf.Line(pts[i-1].X, pts[i-1].Y, pts[i].X, pts[i].Y)
				}
			case "image":
				head, b64, ok := strings.Cut(e.Src, ";base64,")
				kind := strings.TrimPrefix(head, "data:image/")
				if !ok || kind == head {
					continue
				}
				opt := fpdf.ImageOptions{ImageType: kind}
				if pdf.GetImageInfo(e.Src) == nil {
					img, err := base64.StdEncoding.DecodeString(b64)
					if err != nil {
						return nil, fmt.Errorf("ddreport: image: %w", err)
					}
					pdf.RegisterImageOptionsReader(e.Src, opt, bytes.NewReader(img))
				}
				pdf.ImageOptions(e.Src, e.X, e.Y, e.W, e.H, false, opt, 0, "")
			}
		}
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("ddreport: pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// csvSafe stops a spreadsheet from running a text value as a formula: text that
// starts like one ("=cmd|...", "@SUM(...)", "+1+cmd|...") gets a leading quote.
// Plain numbers such as "-5" and a lone "-" are left alone.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '@', '\t', '\r':
		return "'" + s
	case '+', '-':
		if _, err := strconv.ParseFloat(s, 64); err != nil && len(s) > 1 {
			return "'" + s
		}
	}
	return s
}

// toCSV writes UTF-8 with a byte order mark, which is how Excel knows the file
// is UTF-8 (without it "Téh" opens as "TÃ©h"). sep is the separator: "," by
// default, ";" for an Excel whose list separator is ";" (an Indonesian or
// European Windows), or "\t".
func toCSV(rows [][]cell, sep string) ([]byte, error) {
	buf := bytes.NewBufferString(string(rune(0xFEFF)))
	w := csv.NewWriter(buf)
	if c, n := utf8.DecodeRuneInString(sep); n == len(sep) && n > 0 && c != '"' && c != '\r' && c != '\n' {
		w.Comma = c
	}
	for _, r := range rows {
		rec := make([]string, len(r))
		for i, c := range r {
			rec[i] = str(c.v)
			if s, ok := c.v.(string); ok {
				rec[i] = csvSafe(s)
			}
		}
		w.Write(rec)
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// sheetStyles numbers the looks of spreadsheet cells. The plain ones are the
// same in every sheet: 2*format + bold, where format 0 is General and format
// d+1 a number with d decimals. Those with a background or a text colour (a
// table's header, a highlighted text) follow, in the order they come.
type sheetStyles struct {
	extra []cell
	index map[cell]int
}

const plainStyles = 2 * (maxDecimals + 2)

func (s *sheetStyles) of(c cell) int {
	n := 2 * (c.dec + 1)
	if c.bold {
		n++
	}
	if c.fill == "" && c.color == "" {
		return n
	}
	k := cell{dec: c.dec, bold: c.bold, fill: c.fill, color: c.color}
	if i, ok := s.index[k]; ok {
		return i
	}
	if s.index == nil {
		s.index = map[cell]int{}
	}
	s.index[k] = plainStyles + len(s.extra)
	s.extra = append(s.extra, k)
	return s.index[k]
}

// looks lists every style: the plain ones, then the extra ones.
func (s *sheetStyles) looks() []cell {
	var all []cell
	for f := 0; f <= maxDecimals+1; f++ {
		all = append(all, cell{dec: f - 1}, cell{dec: f - 1, bold: true})
	}
	return append(all, s.extra...)
}

// numberCode is the Excel number format for d decimals, "" for General.
func numberCode(d int) string {
	if d < 0 {
		return ""
	}
	code := "#,##0"
	if d > 0 {
		code += "." + strings.Repeat("0", d)
	}
	return code
}

// toXLS writes XML Spreadsheet 2003, which Excel opens as .xls (after one
// "format and extension don't match" prompt). Styles are numbered like toXLSX.
func toXLS(rows [][]cell, widths []float64) ([]byte, error) {
	var st sheetStyles
	var table bytes.Buffer
	for _, w := range widths {
		fmt.Fprintf(&table, `<Column ss:Width="%g"/>`, max(w, 20))
	}
	for _, r := range rows {
		table.WriteString(`<Row>`)
		for _, c := range r {
			if c.v == nil {
				table.WriteString(`<Cell/>`)
				continue
			}
			kind := "String"
			if _, ok := c.v.(float64); ok {
				kind = "Number"
			}
			fmt.Fprintf(&table, `<Cell ss:StyleID="s%d"><Data ss:Type="%s">`, st.of(c), kind)
			xml.EscapeText(&table, []byte(str(c.v)))
			table.WriteString(`</Data></Cell>`)
		}
		table.WriteString(`</Row>`)
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><?mso-application progid="Excel.Sheet"?>` +
		`<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet"><Styles>`)
	for i, l := range st.looks() {
		inner := ""
		if l.bold || l.color != "" {
			inner = `<Font`
			if l.bold {
				inner += ` ss:Bold="1"`
			}
			if l.color != "" {
				inner += ` ss:Color="` + l.color + `"`
			}
			inner += "/>"
		}
		if l.fill != "" {
			inner += `<Interior ss:Color="` + l.fill + `" ss:Pattern="Solid"/>`
		}
		if code := numberCode(l.dec); code != "" {
			inner += `<NumberFormat ss:Format="` + code + `"/>`
		}
		fmt.Fprintf(&b, `<Style ss:ID="s%d">%s</Style>`, i, inner)
	}
	b.WriteString(`</Styles><Worksheet ss:Name="Report"><Table>`)
	b.Write(table.Bytes())
	b.WriteString(`</Table></Worksheet></Workbook>`)
	return b.Bytes(), nil
}

// toXLSX writes a one-sheet workbook, with the styles of sheetStyles.
func toXLSX(rows [][]cell, widths []float64) ([]byte, error) {
	const (
		head  = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
		main  = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
		rels  = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
		rel   = `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/%s" Target="%s"/>`
		ctype = `application/vnd.openxmlformats-officedocument.spreadsheetml.`
	)
	var st sheetStyles
	var sheet bytes.Buffer
	sheet.WriteString(head + `<worksheet ` + main + `>`)
	if len(widths) > 0 {
		sheet.WriteString(`<cols>`)
		for i, w := range widths { // points to Excel's character units, roughly
			fmt.Fprintf(&sheet, `<col min="%[1]d" max="%[1]d" width="%.1f" customWidth="1"/>`, i+1, max(w/5, 4))
		}
		sheet.WriteString(`</cols>`)
	}
	sheet.WriteString(`<sheetData>`)
	for _, r := range rows {
		sheet.WriteString(`<row>`)
		for _, c := range r {
			if c.v == nil {
				sheet.WriteString(`<c/>`)
				continue
			}
			if f, ok := c.v.(float64); ok {
				fmt.Fprintf(&sheet, `<c s="%d"><v>%s</v></c>`, st.of(c), str(f))
				continue
			}
			fmt.Fprintf(&sheet, `<c s="%d" t="inlineStr"><is><t xml:space="preserve">`, st.of(c))
			xml.EscapeText(&sheet, []byte(str(c.v)))
			sheet.WriteString(`</t></is></c>`)
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	// fonts and fills are numbered as they are needed: font 0 plain, 1 bold; fill 0 none, 1 the required gray125
	fonts := []string{`<font><sz val="11"/><name val="Calibri"/></font>`, `<font><b/><sz val="11"/><name val="Calibri"/></font>`}
	fills := []string{`<fill><patternFill patternType="none"/></fill>`, `<fill><patternFill patternType="gray125"/></fill>`}
	number := func(list *[]string, item string) int {
		if i := slices.Index(*list, item); i >= 0 {
			return i
		}
		*list = append(*list, item)
		return len(*list) - 1
	}
	argb := func(c string) string { return "FF" + strings.ToUpper(strings.TrimPrefix(c, "#")) }
	var fmts, xfs strings.Builder
	for f := 0; f <= maxDecimals; f++ {
		fmt.Fprintf(&fmts, `<numFmt numFmtId="%d" formatCode="%s"/>`, 164+f, numberCode(f))
	}
	looks := st.looks()
	for _, l := range looks {
		font := `<font>`
		if l.bold {
			font += `<b/>`
		}
		if l.color != "" {
			font += `<color rgb="` + argb(l.color) + `"/>`
		}
		font += `<sz val="11"/><name val="Calibri"/></font>`
		fill := 0
		if l.fill != "" {
			fill = number(&fills, `<fill><patternFill patternType="solid"><fgColor rgb="`+argb(l.fill)+`"/><bgColor indexed="64"/></patternFill></fill>`)
		}
		id := 0
		if l.dec >= 0 {
			id = 164 + l.dec
		}
		fmt.Fprintf(&xfs, `<xf numFmtId="%d" fontId="%d" fillId="%d" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1"/>`, id, number(&fonts, font), fill)
	}
	styles := head + `<styleSheet ` + main + `><numFmts count="` + strconv.Itoa(maxDecimals+1) + `">` + fmts.String() + `</numFmts>` +
		`<fonts count="` + strconv.Itoa(len(fonts)) + `">` + strings.Join(fonts, "") + `</fonts>` +
		`<fills count="` + strconv.Itoa(len(fills)) + `">` + strings.Join(fills, "") + `</fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="` + strconv.Itoa(len(looks)) + `">` + xfs.String() + `</cellXfs></styleSheet>`

	files := [][2]string{
		{"[Content_Types].xml", head + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="` + ctype + `sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="` + ctype + `worksheet+xml"/>` +
			`<Override PartName="/xl/styles.xml" ContentType="` + ctype + `styles+xml"/></Types>`},
		{"_rels/.rels", head + rels + fmt.Sprintf(rel, 1, "officeDocument", "xl/workbook.xml") + `</Relationships>`},
		{"xl/workbook.xml", head + `<workbook ` + main + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Report" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", head + rels + fmt.Sprintf(rel, 1, "worksheet", "worksheets/sheet1.xml") +
			fmt.Sprintf(rel, 2, "styles", "styles.xml") + `</Relationships>`},
		{"xl/styles.xml", styles},
		{"xl/worksheets/sheet1.xml", sheet.String()},
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := z.Create(f[0])
		if err != nil {
			return nil, err
		}
		w.Write([]byte(f[1]))
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
