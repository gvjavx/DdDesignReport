package ddreport

import (
	"cmp"
	"slices"
	"strconv"
)

// CellStyle overrides the look of one table cell, and merges it with its
// neighbours: ColSpan cells to the right, RowSpan rows down.
type CellStyle struct {
	Fill, Color, Align string // #rrggbb background and text colour; left | center | right | justify
	Bold, Italic       *bool  // unset: as the row
	ColSpan, RowSpan   int
}

// tableRow is one printed row of a table: the template row it comes from and
// the scope its placeholders are filled in with.
type tableRow struct {
	tr   int    // row of Cells
	sc   *scope // nil: the band's own scope
	unit int    // rows with the same unit stay on one page together
	head bool   // a header row, repeated on the next page when the table goes on there
	band int    // which part of the table: 0 header, 1 body, 2 total row; a merge stays inside one
	odd  bool   // a second body row (or item), for banded rows
}

// grid lays a table out in its band: backgrounds, the cell texts and the
// lines, row by row. With measure, the rows of a table that grows are as tall
// as their tallest cell. A table with Data repeats the rows between its header
// and its total row for each item of that list, in which {{field}} is a field
// of the item and {{index}} its number; the header and the total row see all
// the items, so {{sum(qty * price)}} there adds them up. With once, those rows
// come once, with the band's scope: the columns a spreadsheet takes from it.
func (r *report) grid(e Element, sc scope, measure, once bool) ([]Element, float64) {
	if len(e.Cells) == 0 || len(e.Cells[0]) == 0 {
		return nil, 0
	}
	nr, nc := len(e.Cells), len(e.Cells[0])
	r.tables++
	id := r.tables

	// column edges and the template row heights, from the relative widths and heights
	weights := func(n int, given []float64) []float64 {
		w, sum := make([]float64, n), 0.0
		for i := range w {
			w[i] = 1
			if i < len(given) && given[i] > 0 {
				w[i] = given[i]
			}
			sum += w[i]
		}
		for i := range w {
			w[i] /= sum
		}
		return w
	}
	colX := []float64{e.X}
	for _, w := range weights(nc, e.Cols) {
		colX = append(colX, colX[len(colX)-1]+w*e.W)
	}
	baseH := weights(nr, e.Heights)
	for i := range baseH {
		baseH[i] *= e.H
	}

	// the printed rows: header, body (once per item for a data table), total row
	first, last := 0, nr
	if e.Header {
		first = 1
	}
	if e.Footer && nr-1 >= first && (nr > 1 || !e.Header) {
		last = nr - 1
	}
	var items []any
	data := e.Data != "" && !once
	if data {
		list := lookup(e.Data, sc.ctx...)
		if items, _ = list.([]any); items == nil && e.Data == r.t.DataKey {
			items = sc.rows
		} else {
			sc.seen(e.Data, list)
		}
	}
	all := &scope{r.t, items, sc.ctx} // header and total row of a data table: over all its items
	var rows []tableRow
	unit := func() int { r.units++; return r.units }
	if e.Header {
		u := unit()
		rows = append(rows, tableRow{tr: 0, unit: u, head: true, band: 0})
		if data {
			rows[0].sc = all
		}
	}
	switch {
	case data:
		for i, item := range items {
			m, _ := item.(map[string]any)
			own := &scope{r.t, items, append([]map[string]any{m, {"index": float64(i + 1)}}, sc.ctx...)}
			u := unit()
			for tr := first; tr < last; tr++ {
				rows = append(rows, tableRow{tr: tr, sc: own, unit: u, band: 1, odd: i%2 == 1})
			}
		}
	default:
		for tr := first; tr < last; tr++ {
			rows = append(rows, tableRow{tr: tr, unit: unit(), band: 1, odd: (tr-first)%2 == 1})
		}
	}
	if last < nr {
		rows = append(rows, tableRow{tr: nr - 1, unit: unit(), band: 2})
		if data {
			rows[len(rows)-1].sc = all
		}
	}

	style := func(tr, c int) CellStyle { return e.CellStyle[strconv.Itoa(tr)+","+strconv.Itoa(c)] }
	// in is the scope a row's placeholders and colours are filled in with
	in := func(row tableRow) scope {
		if row.sc != nil {
			return *row.sc
		}
		return sc
	}
	// look is a row's background, text colour and weight
	look := func(row tableRow) (fill, ink string, bold bool) {
		switch {
		case row.head:
			fill, ink, bold = e.HeaderFill, e.HeaderColor, true
		case row.band == 2:
			fill, ink, bold = e.FooterFill, e.FooterColor, true
		case row.odd:
			fill, ink, bold = e.Stripe, e.TextColor, e.Bold
		default:
			ink, bold = e.TextColor, e.Bold
		}
		return in(row).paint(fill), in(row).paint(ink), bold
	}
	// span is how many columns and printed rows a cell covers, kept inside the
	// table and inside its part of it (and, for a data table, inside its item)
	span := func(o, c int) (int, int) {
		st := style(rows[o].tr, c)
		cs, rs := max(st.ColSpan, 1), max(st.RowSpan, 1)
		cs = min(cs, nc-c)
		n := 1
		for n < rs && o+n < len(rows) && rows[o+n].band == rows[o].band && rows[o+n].sc == rows[o].sc {
			n++
		}
		return cs, n
	}
	covered := map[[2]int]bool{}
	for o := range rows {
		for c := 0; c < nc; c++ {
			if covered[[2]int{o, c}] {
				continue
			}
			cs, rs := span(o, c)
			for i := range rs {
				for j := range cs {
					if i > 0 || j > 0 {
						covered[[2]int{o + i, c + j}] = true
					}
				}
			}
		}
	}

	ph, pv := 3.0, 2.0 // room between a cell's edge and its text
	if e.Padding != nil {
		ph, pv = max(*e.Padding, 0), max(*e.Padding, 0)
	}
	cell := func(o, c int) Element {
		row, st := rows[o], style(rows[o].tr, c)
		cs, _ := span(o, c)
		t := e
		t.Type, t.Text, t.Fill, t.Grow, t.Data, t.CellStyle, t.Cells = "text", e.Cells[row.tr][c], "", false, "", nil, nil
		_, t.Color, t.Bold = look(row)
		t.Align = ""
		if c < len(e.Aligns) {
			t.Align = e.Aligns[c]
		}
		if st.Color != "" {
			t.Color = in(row).paint(st.Color)
		}
		if st.Align != "" {
			t.Align = st.Align
		}
		if st.Bold != nil {
			t.Bold = *st.Bold
		}
		if st.Italic != nil {
			t.Italic = *st.Italic
		}
		t.X, t.W = colX[c]+ph, colX[c+cs]-colX[c]-2*ph
		t.own = row.sc
		return t
	}

	// row heights: as designed, or as tall as the tallest cell of a table that grows
	rowH := make([]float64, len(rows))
	for o, row := range rows {
		rowH[o] = baseH[row.tr]
		if !measure || !e.Grow {
			continue
		}
		for c := 0; c < nc; c++ {
			if covered[[2]int{o, c}] {
				continue
			}
			if _, rs := span(o, c); rs > 1 {
				continue // a merged cell takes the height of its rows
			}
			t := cell(o, c)
			rs := sc
			if t.own != nil {
				rs = *t.own
			}
			t.runs = t.parse()
			for i := range t.runs {
				t.runs[i].text = rs.resolve(t.runs[i].text)
			}
			rowH[o] = max(rowH[o], float64(len(r.layout(t)))*t.size()*t.leading()+2*pv)
		}
	}
	rowY := []float64{e.Y}
	for _, h := range rowH {
		rowY = append(rowY, rowY[len(rowY)-1]+h)
	}
	total := rowY[len(rowY)-1] - e.Y

	var out []Element
	add := func(o int, x Element) {
		x.ShowIf, x.unit, x.head, x.tbl = e.ShowIf, rows[o].unit, rows[o].head, id
		out = append(out, x)
	}
	if hexColor.MatchString(e.Fill) {
		out = append(out, Element{Type: "rect", X: e.X, Y: e.Y, W: e.W, H: total, Fill: e.Fill, ShowIf: e.ShowIf, tbl: id})
	}
	left, right, top, bottom := e.X, e.X+e.W, e.Y, e.Y+total
	// edges are the lines a cell has, as the grid draws them, for the spreadsheets: left 1, right 2, top 4, bottom 8
	edges := func(x0, x1, y0, y1 float64) uint8 {
		var s uint8
		if e.Border > 0 {
			switch e.Grid {
			case "rows":
				s = 4 | 8
			case "outer":
			default:
				s = 15
			}
		}
		if e.Border > 0 && e.Grid == "outer" || e.Frame > 0 {
			for _, side := range []struct {
				on  bool
				bit uint8
			}{{x0 == left, 1}, {x1 == right, 2}, {y0 == top, 4}, {y1 == bottom, 8}} {
				if side.on {
					s |= side.bit
				}
			}
		}
		return s
	}
	for o, row := range rows {
		for c := 0; c < nc; c++ {
			if covered[[2]int{o, c}] {
				continue
			}
			cs, rs := span(o, c)
			x0, x1, y0, y1 := colX[c], colX[c+cs], rowY[o], rowY[o+rs]
			fill, _, _ := look(row)
			if f := in(row).paint(style(row.tr, c).Fill); hexColor.MatchString(f) {
				fill = f
			}
			if hexColor.MatchString(fill) {
				add(o, Element{Type: "rect", X: x0, Y: y0, W: x1 - x0, H: y1 - y0, Fill: fill})
			}
			t := cell(o, c)
			t.Y, t.H = y0+pv, y1-y0-2*pv
			if hexColor.MatchString(fill) {
				t.bg = fill // for the spreadsheets, which have no rectangles
			}
			t.sides, t.span, t.rspan = edges(x0, x1, y0, y1), cs, rs
			add(o, t)
			if e.Border <= 0 {
				continue
			}
			// each cell draws its own edges, so merged cells have no lines across them
			// and a table cut between two pages keeps a line under its last row there
			hline := func(y float64) Element {
				return Element{Type: "line", X: x0, Y: y, W: x1 - x0, Border: e.Border, Color: e.Color}
			}
			vline := func(x float64) Element {
				return Element{Type: "line", X: x, Y: y0, H: y1 - y0, Border: e.Border, Color: e.Color}
			}
			switch e.Grid {
			case "outer":
				if y0 == top {
					add(o, hline(y0))
				}
				if y1 == bottom {
					add(o, hline(y1))
				}
				if x0 == left {
					add(o, vline(x0))
				}
				if x1 == right {
					add(o, vline(x1))
				}
			case "rows":
				add(o, hline(y0))
				add(o, hline(y1))
			default:
				add(o, hline(y0))
				add(o, hline(y1))
				add(o, vline(x0))
				add(o, vline(x1))
			}
		}
	}
	// heavier lines: under the header (repeated with it on a new page), over the total row, around the table
	for o, row := range rows {
		rule := func(y, w float64) Element {
			return Element{Type: "line", X: left, Y: y, W: right - left, Border: w, Color: e.Color}
		}
		if row.head && e.HeaderLine > 0 && (o+1 == len(rows) || !rows[o+1].head) {
			add(o, rule(rowY[o+1], e.HeaderLine))
		}
		if row.band == 2 && e.FooterLine > 0 {
			add(o, rule(rowY[o], e.FooterLine))
		}
	}
	if e.Frame > 0 { // a frame cut between two pages is closed on both
		out = append(out, Element{Type: "rect", X: left, Y: top, W: right - left, H: bottom - top, Border: e.Frame, Color: e.Color, ShowIf: e.ShowIf, tbl: id})
	}
	return out, total
}

// flat is a band with its tables and sub-reports laid out as texts, for the
// spreadsheets. As on the page, what is under a table or sub-report that came
// out taller than designed moves down, so it is read after it.
func (r *report) flat(b Band, sc scope, once bool) Band {
	type growth struct{ bottom, extra float64 } // where, as designed
	var els []Element
	var grown []growth
	// top to bottom, so a table under one that grew moves down too, with its texts
	for _, e := range slices.SortedStableFunc(slices.Values(b.Elements), func(a, b Element) int { return cmp.Compare(a.Y, b.Y) }) {
		top := e.Y
		for _, g := range grown {
			if top >= g.bottom {
				e.Y += g.extra
			}
		}
		var parts []Element
		var h float64
		if e.Type == "crosstab" {
			e = r.crosstab(e, sc)
		}
		switch e.Type {
		case "subreport":
			parts, h = r.subFlat(e, sc, once)
		case "table":
			cells, th := r.grid(e, sc, false, once)
			for _, c := range cells {
				if c.Type == "text" {
					parts = append(parts, c)
				}
			}
			h = th
		default:
			els = append(els, e)
			continue
		}
		els = append(els, parts...)
		if h > e.H {
			grown = append(grown, growth{top + e.H, h - e.H})
		}
	}
	b.Elements = els
	return b
}
