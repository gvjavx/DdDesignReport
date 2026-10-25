package ddreport

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"image"
	_ "image/gif" // the picture formats the PDF draws
	_ "image/jpeg"
	_ "image/png"
	"math"
	"slices"
	"strings"
)

// place resolves one instance of a band: placeholders, tables, growing texts,
// charts and barcodes. Coordinates stay relative to the band; it returns the
// final height, which is at least as tall as what is in the band.
//
// A text that grows pushes down everything that starts at or below its old
// bottom edge. Pushes add up down the band (a second grown text under the first
// moves by the first one's growth and then pushes by its own), while texts side
// by side push by the larger of the two. Boxes and vertical rules that reach
// past a grown text stretch, so frames stay closed. A table grows as a whole:
// its rows are as tall as they need, and a data table has a row per item.
func (r *report) place(b Band, sc scope) ([]Element, float64) {
	type grower struct{ top, bottom, grow, shift float64 }
	var els []Element
	var anchor []float64 // the top of the table an element belongs to, which moves it; NaN for the rest
	var growers []grower
	for _, e := range b.Elements {
		if !sc.shows(e) {
			continue
		}
		e.Color, e.Fill = sc.paint(e.Color), sc.paint(e.Fill)
		switch {
		case e.Type == "table" || e.Type == "subreport":
			var cells []Element
			var h float64
			if e.Type == "table" {
				cells, h = r.grid(e, sc, true, false)
			} else {
				cells, h = r.subreport(e, sc)
			}
			for _, c := range cells {
				els, anchor = append(els, c), append(anchor, e.Y)
			}
			if h > e.H {
				growers = append(growers, grower{e.Y, e.Y + e.H, h - e.H, 0})
			}
			continue
		case e.Type == "image" && strings.Contains(e.Src, "{{"): // a picture from the data
			if e.Src = sc.resolve(e.Src); !picture(e.Src) {
				continue // no picture, or not one the PDF can draw: the space stays empty
			}
		}
		els, anchor = append(els, e), append(anchor, math.NaN())
	}
	height := make([]float64, len(els)) // new height of the texts that grew
	for i := range els {
		e := &els[i]
		if e.done { // laid out inside a sub-report already
			continue
		}
		if e.Type == "barcode" {
			e.Text = sc.resolve(e.Text)
		}
		if e.Type != "text" {
			continue
		}
		rs := sc // a cell of a data table is filled in from its item
		if e.own != nil {
			rs = *e.own
		}
		e.runs = e.parse()
		e.Text = ""
		for j := range e.runs {
			e.runs[j].text = rs.resolve(e.runs[j].text)
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
		switch {
		case !math.IsNaN(anchor[i]): // part of a table: moves with the table, already sized
			e.Y += shift(anchor[i], all)
		case height[i] > 0:
			e.Y += shift(e.Y, all)
			e.H = height[i]
		case e.Type == "rect" || e.Type == "line" && e.H > e.W && e.Slope == "":
			e.Y += shift(e.Y, all)
			e.H = bottom + shift(bottom, all) - e.Y
		default:
			e.Y += shift(e.Y, all)
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
	h := b.Height + shift(math.Inf(1), all)
	for _, e := range out { // a band never cuts off what is in it
		h = max(h, e.Y+e.H)
	}
	return out, h
}

// picture says whether a value from the data is a picture the PDF can draw: a
// PNG, JPEG or GIF data URI whose image actually decodes. A broken picture in
// one row then leaves its space empty instead of failing the whole report.
func picture(src string) bool {
	_, b64, ok := strings.Cut(src, ";base64,")
	if !ok || !dataImage.MatchString(src) {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return false
	}
	_, _, err = image.DecodeConfig(bytes.NewReader(raw))
	return err == nil
}
