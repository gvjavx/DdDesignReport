package ddreport

import (
	"bytes"
	"cmp"
	"image"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// The "escpos" and "txt" formats lay a report out in lines of characters: ESC/POS
// for receipt printers (58 and 80 mm thermal, over USB, network or Bluetooth),
// and plain text for previews, logs and dot-matrix printers. A text goes in the
// columns its box covers, aligned as designed and wrapped inside them; a flat
// line becomes a row of dashes; texts of 11 pt and over print double height.
// The paper is one long roll: header first, footer last, no page breaks.
// In ESC/POS a QR code or Code 128 is drawn by the printer itself and a picture
// is printed as dots; plain text writes a barcode's content and leaves pictures out.
// ponytail: charts are left out of both; draw them as dots (like pictures) if receipts ever need one

// tline is one printed line.
type tline struct {
	chars []rune
	bold  []bool
	tall  bool   // double height
	raw   []byte // escpos: a barcode or picture, printed instead of chars
}

// columns is the characters per line: TextColumns, or what the paper fits
// with the printer's normal font: 32 on 58 mm, 48 on 80 mm, 80 wider (dot matrix).
func (t *Template) columns() int {
	if t.TextColumns > 0 {
		return t.TextColumns
	}
	switch w := t.Page.Width; {
	case w <= 170: // up to 60 mm
		return 32
	case w <= 260: // up to 92 mm
		return 48
	}
	return 80
}

// textLayout is the report as printed lines, bands in the order they print.
func (r *report) textLayout() []tline {
	t, b := r.t, r.t.Bands
	_, right, _, left := t.Page.margins()
	width := max(t.Page.Width-left-right, 1)
	cols := t.columns()
	dots := 384.0 // the printable dots of a 58 mm printer, 576 on 80 mm
	if cols > 32 {
		dots = 576
	}
	col := func(x float64) int { return min(max(int(math.Round(x/width*float64(cols))), 0), cols) }
	var out []tline
	blank := func(tall bool) tline {
		return tline{chars: []rune(strings.Repeat(" ", cols)), bold: make([]bool, cols), tall: tall}
	}
	// where on the paper something of the band sits: ESC a 0, 1 or 2
	side := func(e Element) byte {
		switch mid := (e.X + e.W/2) / width; {
		case mid > 0.4 && mid < 0.6:
			return 1
		case mid >= 0.6:
			return 2
		}
		return 0
	}
	band := func(bd Band, sc scope) {
		sc.ctx = append(sc.ctx, map[string]any{"page": 1.0, "pages": 1.0}) // one long page
		flat := r.flat(bd, sc, false)
		groups := textLines(flat)
		for _, e := range flat.Elements {
			if e.Type == "barcode" || e.Type == "image" || e.Type == "line" && e.Slope == "" && e.W > e.H {
				groups = append(groups, []Element{e})
			}
		}
		top := func(g []Element) float64 {
			return slices.MinFunc(g, func(a, b Element) int { return cmp.Compare(a.Y, b.Y) }).Y
		}
		slices.SortStableFunc(groups, func(a, b []Element) int { return cmp.Compare(top(a), top(b)) })
		bottom := math.Inf(-1)
		for _, g := range groups {
			// an empty line where the design leaves a line's worth of room between two lines
			if y := top(g); !math.IsInf(bottom, -1) && y-bottom >= 9 {
				out = append(out, blank(false))
			}
			for _, e := range g {
				bottom = max(bottom, e.Y+e.H)
			}
			e := g[0]
			if !sc.shows(e) && e.Type != "text" {
				continue
			}
			switch e.Type {
			case "line":
				l := blank(false)
				a, z := col(e.X), max(col(e.X+e.W), col(e.X)+1)
				for i := a; i < min(z, cols); i++ {
					l.chars[i] = '-'
				}
				out = append(out, l)
				continue
			case "barcode":
				text := sc.resolve(e.Text)
				if text == "" {
					continue
				}
				l := blank(false)
				put(l, text, col(e.X), max(col(e.X+e.W), col(e.X)+1), "center", false)
				l.raw = barcodeCommand(e, text, side(e), e.W/width*dots)
				out = append(out, l)
				continue
			case "image":
				data, _, ok := pictureOf(e, sc)
				img, _, err := image.Decode(bytes.NewReader(data))
				if !ok || err != nil {
					continue
				}
				out = append(out, tline{raw: rasterCommand(img, side(e), e.W/width*dots)})
				continue
			}
			// a line of texts: each in its columns, wrapped there, so the line can take several rows
			tall := slices.ContainsFunc(g, func(e Element) bool { return e.FontSize >= 11 })
			var rows []tline
			for _, e := range g {
				if !sc.shows(e) {
					continue
				}
				s := sc
				if e.own != nil { // a cell of a data table: filled in from its item
					s = *e.own
				}
				bold := e.Bold || strings.HasPrefix(e.Text, "<b>") && strings.HasSuffix(e.Text, "</b>")
				a, z := col(e.X), col(e.X+e.W)
				if a >= cols {
					continue
				}
				z = max(z, a+1)
				for i, text := range wrap(s.resolve(e.plain()), z-a) {
					for len(rows) <= i {
						rows = append(rows, blank(tall))
					}
					put(rows[i], text, a, z, e.Align, bold)
				}
			}
			out = append(out, rows...)
		}
	}

	// the same walk as the spreadsheets: header and column header once, each
	// group around its rows, a detail band per row, then summary and footer
	band(b.Header, scope{t, r.rows, []map[string]any{r.root}})
	band(b.ColumnHeader, scope{t, r.rows, []map[string]any{r.root}})
	levels := t.levels()
	var walk func(rows []any, lv int)
	walk = func(rows []any, lv int) {
		if lv == len(levels) {
			for i, row := range rows {
				m, _ := row.(map[string]any)
				band(b.Detail, scope{t, rows, []map[string]any{m, {"index": float64(i + 1)}, r.root}})
			}
			return
		}
		for _, g := range groupRows(rows, levels[lv].by) {
			var first map[string]any
			if len(g) > 0 {
				first, _ = g[0].(map[string]any)
			}
			band(levels[lv].header, scope{t, g, []map[string]any{first, r.root}})
			walk(g, lv+1)
			band(levels[lv].footer, scope{t, g, []map[string]any{first, r.root}})
		}
	}
	walk(r.rows, 0)
	band(b.Summary, scope{t, r.rows, []map[string]any{r.root}})
	band(b.Footer, scope{t, r.rows, []map[string]any{r.root}})
	return out
}

// put writes text into columns a to z of a line, aligned, over what is there.
func put(l tline, text string, a, z int, align string, bold bool) {
	n := utf8.RuneCountInString(text)
	at := a
	switch {
	case n > z-a: // wider than its box, as a barcode's text: from its left edge
	case align == "center":
		at = a + (z-a-n)/2
	case align == "right":
		at = z - n
	}
	for i, c := range []rune(text) {
		if k := at + i; k >= 0 && k < len(l.chars) {
			l.chars[k], l.bold[k] = c, bold
		}
	}
}

// wrap breaks a text into lines of at most width characters, at spaces where it can.
func wrap(text string, width int) []string {
	var out []string
	for _, para := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		line := ""
		for _, w := range strings.Fields(para) {
			for utf8.RuneCountInString(w) > width { // a word longer than the line is cut
				if line != "" {
					out, line = append(out, line), ""
				}
				r := []rune(w)
				out, w = append(out, string(r[:width])), string(r[width:])
			}
			switch {
			case line == "":
				line = w
			case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) <= width:
				line += " " + w
			default:
				out, line = append(out, line), w
			}
		}
		out = append(out, line)
	}
	if len(out) == 1 && out[0] == "" {
		return nil // nothing to print: the line is left out, as an empty field
	}
	return out
}

// toText writes the lines as plain text.
func toText(lines []tline) []byte {
	var b bytes.Buffer
	for _, l := range lines {
		if l.chars == nil {
			continue // a picture
		}
		b.WriteString(strings.TrimRight(string(l.chars), " "))
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// toEscpos writes the lines as ESC/POS commands: the printer reset, the
// Windows-1252 code page, the lines, then a feed and a cut (ignored by printers
// without a cutter).
func toEscpos(lines []tline) []byte {
	b := bytes.NewBuffer([]byte{0x1b, '@', 0x1b, 't', 16})
	for _, l := range lines {
		if l.raw != nil {
			b.Write(l.raw)
			continue
		}
		if l.tall {
			b.Write([]byte{0x1d, '!', 0x01})
		}
		on, end := false, len(l.chars)
		for end > 0 && l.chars[end-1] == ' ' {
			end--
		}
		for i, c := range l.chars[:end] {
			if l.bold[i] != on {
				on = l.bold[i]
				b.Write([]byte{0x1b, 'E', map[bool]byte{true: 1, false: 0}[on]})
			}
			b.WriteByte(cp1252(c))
		}
		if on {
			b.Write([]byte{0x1b, 'E', 0})
		}
		if l.tall {
			b.Write([]byte{0x1d, '!', 0})
		}
		b.WriteByte('\n')
	}
	b.Write([]byte{0x1d, 'V', 66, 3}) // feed 3 lines, then cut
	return b.Bytes()
}

// cp1252 is a character in the printer's Windows-1252 code page, or ? when it has none.
func cp1252(c rune) byte {
	switch {
	case c < 0x80 || c >= 0xa0 && c <= 0xff:
		return byte(c)
	}
	if b, ok := map[rune]byte{'€': 0x80, '…': 0x85, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97}[c]; ok {
		return b
	}
	return '?'
}

// barcodeCommand prints a QR code or Code 128 with the printer's own commands,
// at the side of the paper it was designed on, about as wide as its box.
func barcodeCommand(e Element, text string, side byte, dots float64) []byte {
	b := bytes.NewBuffer([]byte{0x1b, 'a', side})
	if e.Format == "code128" {
		h := byte(min(max(e.H*203/72*0.7, 24), 255)) // the bars, without the room for the text under them
		hri := byte(0)
		if e.ShowText {
			hri = 2
		}
		data := "{B" + strings.Map(func(c rune) rune { // Code 128 set B: printable ASCII
			if c < 32 || c > 126 {
				return '?'
			}
			return c
		}, text)
		b.Write([]byte{0x1d, 'h', h, 0x1d, 'w', 2, 0x1d, 'H', hri, 0x1d, 'k', 73, byte(min(len(data), 255))})
		b.WriteString(data[:min(len(data), 255)])
	} else {
		// a short text makes a QR of about 29 modules, with the room around it
		size := byte(min(max(int(dots/33), 2), 8))
		n := len(text) + 3
		b.Write([]byte{0x1d, '(', 'k', 4, 0, 49, 65, 50, 0}) // model 2
		b.Write([]byte{0x1d, '(', 'k', 3, 0, 49, 67, size})
		b.Write([]byte{0x1d, '(', 'k', 3, 0, 49, 69, 49}) // error correction M, as the PDF's
		b.Write([]byte{0x1d, '(', 'k', byte(n), byte(n >> 8), 49, 80, 48})
		b.WriteString(text)
		b.Write([]byte{0x1d, '(', 'k', 3, 0, 49, 81, 48})
	}
	b.Write([]byte{'\n', 0x1b, 'a', 0})
	return b.Bytes()
}

// rasterCommand prints a picture as black and white dots (GS v 0), dots wide,
// in strips a printer's buffer can take.
func rasterCommand(img image.Image, side byte, dots float64) []byte {
	src := img.Bounds()
	w := max(int(dots)/8*8, 8)
	h := max(src.Dy()*w/max(src.Dx(), 1), 1)
	b := bytes.NewBuffer([]byte{0x1b, 'a', side})
	for y0 := 0; y0 < h; y0 += 128 {
		rows := min(128, h-y0)
		b.Write([]byte{0x1d, 'v', '0', 0, byte(w / 8), byte(w / 8 >> 8), byte(rows), byte(rows >> 8)})
		for y := y0; y < y0+rows; y++ {
			for x0 := 0; x0 < w; x0 += 8 {
				var bits byte
				for k := range 8 {
					c := img.At(src.Min.X+(x0+k)*src.Dx()/w, src.Min.Y+y*src.Dy()/h)
					r, g, bl, a := c.RGBA()
					// on white paper: what is see-through is white (RGBA is premultiplied by alpha)
					lum := (r*299+g*587+bl*114)/1000 + 0xffff - a
					if lum < 0x8000 {
						bits |= 0x80 >> k
					}
				}
				b.WriteByte(bits)
			}
		}
	}
	b.Write([]byte{0x1b, 'a', 0})
	return b.Bytes()
}
