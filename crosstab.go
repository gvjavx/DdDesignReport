package ddreport

import "strings"

// crosstab turns a crosstab element into the table it prints: a row for each
// value of RowField, a column for each value of ColumnField (both in the order
// they first come in the rows, so sortBy decides it), and in each cell Measure
// worked out over the rows that have both values, such as "sum(qty * price):2"
// or "count()". A total column, a total row and the grand total come last.
//
// The rows are those of the band (the group in a group band, all of them
// elsewhere), or the list named by Data. The result is an ordinary table, so
// borders, header and total colours, stripes, page breaks and the spreadsheets
// work as they do for tables.
func (r *report) crosstab(e Element, sc scope) Element {
	rows := sc.rows
	if e.Data != "" {
		if items, ok := lookup(e.Data, sc.ctx...).([]any); ok {
			rows = items
		}
	}
	key := func(row any, field string) string {
		m, _ := row.(map[string]any)
		return str(lookup(field, m))
	}
	// the values of a field, in the order they first come
	values := func(field string) []string {
		var out []string
		seen := map[string]bool{}
		for _, row := range rows {
			if k := key(row, field); !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
		return out
	}
	measure := strings.TrimSpace(e.Measure)
	if measure == "" {
		measure = "count()"
	}
	// a cell: the measure over the rows that match, as {{measure}} would print it in a group band
	cell := func(keep func(row any) bool) string {
		var subset []any
		for _, row := range rows {
			if keep(row) {
				subset = append(subset, row)
			}
		}
		if len(subset) == 0 {
			return ""
		}
		first, _ := subset[0].(map[string]any)
		return scope{sc.t, subset, append([]map[string]any{first}, sc.ctx...)}.resolve("{{" + measure + "}}")
	}
	// a value from the data goes into a cell as text: nothing in it is read as a placeholder or as markup
	plain := func(s string) string {
		return strings.NewReplacer("{{", "{ {", "<", "‹", ">", "›").Replace(s)
	}
	total := e.TotalLabel
	if total == "" {
		total = "Total"
	}

	down, across := values(e.RowField), values(e.ColumnField)
	head := []string{e.Text}
	for _, c := range across {
		head = append(head, plain(c))
	}
	cells := [][]string{append(head, total)}
	for _, rv := range down {
		line := []string{plain(rv)}
		for _, cv := range across {
			line = append(line, cell(func(row any) bool { return key(row, e.RowField) == rv && key(row, e.ColumnField) == cv }))
		}
		line = append(line, cell(func(row any) bool { return key(row, e.RowField) == rv }))
		cells = append(cells, line)
	}
	foot := []string{total}
	for _, cv := range across {
		foot = append(foot, cell(func(row any) bool { return key(row, e.ColumnField) == cv }))
	}
	cells = append(cells, append(foot, cell(func(any) bool { return true })))

	t := e
	t.Type, t.Data, t.Cells, t.Header, t.Footer = "table", "", cells, true, true
	t.Aligns = []string{"left"}
	t.Cols = []float64{2}
	for range len(cells[0]) - 1 {
		t.Aligns, t.Cols = append(t.Aligns, "right"), append(t.Cols, 1)
	}
	// the box in the designer shows three rows (header, one row, total): every row is that tall
	t.Heights, t.H = nil, e.H/3*float64(len(cells))
	return t
}
