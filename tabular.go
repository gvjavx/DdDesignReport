package ddreport

import (
	"cmp"
	"math"
	"slices"
)

// cell is one spreadsheet cell. dec is the decimals of a formatted number, or -1.
type cell struct {
	v           any
	bold        bool
	dec         int
	fill, color string // #rrggbb background and text colour, "" for none
	align       string // center or right; "" for the spreadsheet's own
	boxed       bool   // lines on every side, as in a table with a full grid
}

// textLines returns the text elements of a band as the lines a reader sees:
// top to bottom, and left to right within a line. A text starts a new line
// when it begins below the middle of the line before it.
func textLines(band Band) [][]Element {
	els := slices.DeleteFunc(slices.Clone(band.Elements), func(e Element) bool { return e.Type != "text" })
	slices.SortStableFunc(els, func(a, b Element) int { return cmp.Compare(a.Y, b.Y) })
	var out [][]Element
	var next float64
	for _, e := range els {
		if len(out) == 0 || e.Y >= next {
			out = append(out, nil)
			next = e.Y + max(e.H/2, 1)
		}
		out[len(out)-1] = append(out[len(out)-1], e)
	}
	for _, line := range out {
		slices.SortStableFunc(line, func(a, b Element) int { return cmp.Compare(a.X, b.X) })
	}
	return out
}

// table is the tabular view used by CSV, XLSX and XLS.
//
// Every text of the detail band is a column, in reading order, and every data
// row is one line. The other bands (column header, group header and footer,
// summary) add one line per visual line of text, each text landing in the
// column it is aligned with, so a subtotal sits under the numbers it sums.
//
// A report without a detail band (a letter, a certificate) has no columns:
// its texts are exported line by line as they read.
//
// It also returns the column widths.
func (r *report) table() ([][]cell, []float64) {
	t, b := r.t, r.t.Bands
	anchor := func(e Element, align string) float64 {
		switch align {
		case "right":
			return e.X + e.W
		case "center":
			return e.X + e.W/2
		}
		return e.X
	}
	// the columns come from the detail band as designed: a data table there gives its row once
	none := scope{t, nil, nil}
	cols := slices.Concat(textLines(r.flat(b.Detail, none, true))...)
	fromDetail := len(cols) > 0
	if !fromDetail {
		cols = slices.Concat(textLines(r.flat(b.ColumnHeader, none, true))...)
	}
	// a data table in the detail band has a line per item, so each row is written line by
	// line, and its header, item and total rows share columns: one column per place
	nested := slices.ContainsFunc(b.Detail.Elements, func(e Element) bool { return e.Type == "table" && e.Data != "" || e.Type == "subreport" })
	if nested {
		cols = slices.CompactFunc(slices.SortedStableFunc(slices.Values(cols), func(a, b Element) int { return cmp.Compare(a.X, b.X) }),
			func(a, b Element) bool { return a.X == b.X && a.W == b.W })
	}

	var out [][]cell
	// lines adds one row per visual line of the band
	lines := func(band Band, rows []any, ctx ...map[string]any) {
		sc := scope{t, rows, ctx}
		for _, line := range textLines(r.flat(band, sc, false)) {
			if len(cols) == 0 { // free-form document: the texts as they read
				var cells []cell
				for _, e := range line {
					if sc.shows(e) {
						cells = append(cells, sc.cell(e))
					}
				}
				out = append(out, cells)
				continue
			}
			cells := make([]cell, len(cols))
			for _, e := range line {
				if !sc.shows(e) {
					continue
				}
				// the column with the nearest edge on the side the text is aligned to;
				// between columns that share that edge, the one most alike in extent
				best, a := 0, e.align()
				score := func(c Element) (float64, float64) {
					return math.Abs(anchor(c, a) - anchor(e, a)), math.Abs(c.X-e.X) + math.Abs(c.X+c.W-e.X-e.W)
				}
				for i, c := range cols {
					d, tie := score(c)
					if bd, btie := score(cols[best]); d < bd || d == bd && tie < btie {
						best = i
					}
				}
				c := sc.cell(e)
				if prev := cells[best]; prev.v != nil { // two texts over one column
					c.v, c.dec = str(prev.v)+" "+str(c.v), -1
				}
				cells[best] = c
			}
			out = append(out, cells)
		}
	}
	// detail adds one row per data row: each text is its own column
	detail := func(rows []any, ctx ...map[string]any) {
		if !fromDetail {
			return
		}
		sc := scope{t, rows, ctx}
		cells := make([]cell, len(cols))
		for i, e := range cols {
			if sc.shows(e) { // a hidden text leaves its column empty
				cells[i] = sc.cell(e)
			}
		}
		out = append(out, cells)
	}

	if len(cols) == 0 {
		lines(b.Header, r.rows, r.root)
	}
	lines(b.ColumnHeader, r.rows, r.root)
	levels := t.levels()
	var walk func(rows []any, lv int)
	walk = func(rows []any, lv int) {
		if lv == len(levels) {
			for _, row := range rows {
				m, _ := row.(map[string]any)
				if nested {
					lines(b.Detail, rows, m, r.root)
				} else {
					detail(rows, m, r.root)
				}
			}
			return
		}
		for _, g := range groupRows(rows, levels[lv].by) {
			var first map[string]any
			if len(g) > 0 {
				first, _ = g[0].(map[string]any)
			}
			lines(levels[lv].header, g, first, r.root)
			walk(g, lv+1)
			lines(levels[lv].footer, g, first, r.root)
		}
	}
	walk(r.rows, 0)
	lines(b.Summary, r.rows, r.root)
	widths := make([]float64, len(cols))
	for i, c := range cols {
		widths[i] = c.W
	}
	return out, widths
}
