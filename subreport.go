package ddreport

// A sub-report is a report inside a band: its own header, column header,
// groups, detail and summary, printed for each item of a list in the data, in
// the box of the element. It grows its band like a table, keeps each of its
// band instances on one page, and repeats its header and column header at the
// top of a page it goes on to. Its footer is not used.

// child is the template of a sub-report, with what it does not set taken from
// the report around it: separators, locale, the time of {{now}}.
func (r *report) child(e Element) *Template {
	sub := e.Report
	if !sub.ready {
		sub.ready = true
		if sub.Decimal == "" {
			sub.Decimal, sub.Thousands = r.t.Decimal, r.t.Thousands
		}
		if sub.Thousands == "none" {
			sub.Thousands = ""
		}
		if sub.Locale == "" {
			sub.Locale = r.t.Locale
		}
		sub.now = r.t.now
	}
	return sub
}

// subBands walks the band instances of a sub-report in print order, with the
// scope each is filled in with: over the items for the header, column header
// and summary, over its group for a group band, the item itself for a detail.
func (r *report) subBands(e Element, sc scope, visit func(b Band, sc scope, head bool)) {
	sub := r.child(e)
	items, _ := lookup(e.Data, sc.ctx...).([]any)
	if items == nil && (e.Data == "" || e.Data == r.t.DataKey) {
		items = sc.rows
	}
	items = append([]any(nil), items...) // sorting must not reorder the data of the report around it
	sub.sort(items)
	b := sub.Bands
	all := scope{sub, items, sc.ctx}
	visit(b.Header, all, true)
	visit(b.ColumnHeader, all, true)
	levels := sub.levels()
	var walk func(rows []any, lv int)
	walk = func(rows []any, lv int) {
		if lv == len(levels) {
			for i, row := range rows {
				m, _ := row.(map[string]any)
				visit(b.Detail, scope{sub, rows, append([]map[string]any{m, {"index": float64(i + 1)}}, sc.ctx...)}, false)
			}
			return
		}
		for _, g := range groupRows(rows, levels[lv].by) {
			var first map[string]any
			if len(g) > 0 {
				first, _ = g[0].(map[string]any)
			}
			ctx := append([]map[string]any{first}, sc.ctx...)
			visit(levels[lv].header, scope{sub, g, ctx}, false)
			walk(g, lv+1)
			visit(levels[lv].footer, scope{sub, g, ctx}, false)
		}
	}
	walk(items, 0)
	visit(b.Summary, all, false)
}

// subreport lays a sub-report out in its box: its elements, in the
// coordinates of the band around it, and their height.
func (r *report) subreport(e Element, sc scope) ([]Element, float64) {
	if e.Report == nil {
		return nil, 0
	}
	r.tables++
	id := r.tables
	var out []Element
	y := 0.0
	r.subBands(e, sc, func(b Band, bsc scope, head bool) {
		if len(b.Elements) == 0 && b.Height == 0 {
			return
		}
		els, h := r.place(b, bsc)
		r.units++
		for _, x := range els {
			x.X += e.X
			x.Y += e.Y + y
			x.done = true
			if x.unit == 0 { // a table inside keeps its own rows together; the rest keeps the band together
				x.unit = r.units
			}
			if x.tbl == 0 {
				x.tbl = id
			}
			x.head = x.head || head
			x.ShowIf = "" // already decided, with the scope of its band
			out = append(out, x)
		}
		y += h
	})
	return out, y
}

// subFlat is a sub-report as texts for the spreadsheets: a line of the
// spreadsheet per band instance and visual line, each with its own scope.
func (r *report) subFlat(e Element, sc scope, once bool) ([]Element, float64) {
	if e.Report == nil {
		return nil, 0
	}
	if once { // the columns: its detail band as designed
		var out []Element
		for _, x := range r.flat(r.child(e).Bands.Detail, sc, true).Elements {
			if x.Type == "text" {
				x.X += e.X
				x.Y += e.Y
				out = append(out, x)
			}
		}
		return out, e.H
	}
	var out []Element
	y := e.Y
	r.subBands(e, sc, func(b Band, bsc scope, head bool) {
		for _, x := range r.flat(b, bsc, false).Elements {
			if x.Type != "text" || !bsc.shows(x) {
				continue
			}
			if x.own == nil {
				own := bsc
				x.own = &own
			}
			x.X += e.X
			x.Y += y
			x.ShowIf = ""
			out = append(out, x)
		}
		y += b.Height
	})
	return out, y - e.Y
}
