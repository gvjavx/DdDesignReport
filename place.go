package ddreport

import (
	"cmp"
	"math"
	"slices"
)

// place resolves one instance of a band: placeholders, growing texts, charts
// and barcodes. Coordinates stay relative to the band; it returns the final height.
//
// A text that grows pushes down everything that starts at or below its old
// bottom edge. Pushes add up down the band (a second grown text under the first
// moves by the first one's growth and then pushes by its own), while texts side
// by side push by the larger of the two. Boxes and vertical rules that reach
// past a grown text stretch, so frames stay closed.
func (r *report) place(b Band, sc scope) ([]Element, float64) {
	type grower struct{ top, bottom, grow, shift float64 }
	els := slices.Clone(b.Elements)
	height := make([]float64, len(els)) // new height of the texts that grew
	var growers []grower
	for i := range els {
		e := &els[i]
		if e.Type == "barcode" {
			e.Text = sc.resolve(e.Text)
		}
		if e.Type != "text" {
			continue
		}
		e.runs = e.parse()
		e.Text = ""
		for j := range e.runs {
			e.runs[j].text = sc.resolve(e.runs[j].text)
			e.Text += e.runs[j].text
		}
		if !e.Grow {
			continue
		}
		if need := float64(len(r.layout(*e))) * e.size() * e.leading(); need > e.H {
			height[i] = need
			growers = append(growers, grower{e.Y, e.Y + e.H, need - e.H, 0})
		}
	}

	// shift(y) is how far something at y moves down, given the first n growers
	slices.SortFunc(growers, func(a, b grower) int { return cmp.Compare(a.bottom, b.bottom) })
	shift := func(y float64, n int) float64 {
		s := 0.0
		for _, g := range growers[:n] {
			if g.bottom <= y {
				s = max(s, g.shift+g.grow)
			}
		}
		return s
	}
	for i := range growers { // sorted by bottom edge, so everything above a grower is already known
		growers[i].shift = shift(growers[i].top, i)
	}
	all := len(growers)

	var out []Element
	for i, e := range els {
		bottom := e.Y + e.H
		e.Y += shift(e.Y, all)
		switch {
		case height[i] > 0:
			e.H = height[i]
		case e.Type == "rect" || e.Type == "line" && e.H > e.W:
			e.H = bottom + shift(bottom, all) - e.Y
		}
		switch e.Type {
		case "chart":
			out = append(out, chart(sc.t, e, sc.rows)...)
		case "barcode":
			out = append(out, bars(e)...)
		default:
			out = append(out, e)
		}
	}
	return out, b.Height + shift(math.Inf(1), all)
}
