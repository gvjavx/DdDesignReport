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
	cols, rows := code.Bounds().Dx(), code.Bounds().Dy()
	mw, mh := e.W/float64(cols), e.H // Code 128: one row of bars as tall as the element
	if rows > 1 {                    // QR: square modules
		mw = min(e.W, e.H) / float64(cols)
		mh = mw
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
	return []Element{{Type: "code", X: e.X, Y: e.Y, W: e.W, H: e.H, Color: color(e.Color, "#000000"), Points: runs}}
}
