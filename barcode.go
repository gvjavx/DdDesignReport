package ddreport

import (
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
)

// bars turns a barcode element into one "code" element whose Points list the
// dark runs (x, y, w, h): a QR code, or Code 128 for scanners that read lines.
// Text that cannot be encoded (empty, or not ASCII for Code 128) draws nothing.
func bars(e Element) []Element {
	var code barcode.Barcode
	var err error
	if e.Format == "code128" {
		code, err = code128.Encode(e.Text)
	} else {
		code, err = qr.Encode(e.Text, qr.M, qr.Auto)
	}
	if err != nil || e.Text == "" {
		return nil
	}
	h := e.H
	if e.ShowText { // the text goes under the code, in the last 10pt
		h = max(e.H-10, 1)
	}
	cols, rows := code.Bounds().Dx(), code.Bounds().Dy()
	mw, mh, quiet := e.W/float64(cols), h, 0.0 // Code 128: one row of bars as tall as the element
	if rows > 1 {                              // QR: square modules, inside a quiet zone two modules wide, so it scans next to other things
		mw = min(e.W, h) / float64(cols+4)
		mh, quiet = mw, 2*mw
	}
	dark := func(x, y int) bool {
		if x >= cols {
			return false
		}
		r, _, _, _ := code.At(x, y).RGBA()
		return r == 0
	}
	var runs []float64
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			if !dark(x, y) {
				continue
			}
			start := x
			for dark(x+1, y) {
				x++
			}
			runs = append(runs, float64(start)*mw, float64(y)*mh, float64(x-start+1)*mw, mh)
		}
	}
	// the code sits inside its quiet zone; its runs start at 0, which keeps them short in HTML
	out := []Element{{Type: "code", X: e.X + quiet, Y: e.Y + quiet, W: e.W - 2*quiet, H: h - 2*quiet, Color: color(e.Color, "#000000"), Points: runs, mw: mw, mh: mh}}
	if e.ShowText {
		w := e.W
		if rows > 1 {
			w = mw * float64(cols+4)
		}
		out = append(out, Element{Type: "text", X: e.X, Y: e.Y + h + 1, W: w, H: 9, Text: e.Text, FontSize: 7, Align: "center", Color: e.Color})
	}
	return out
}
