package ddreport

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/draw"
	"image/png"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
)

// sheetPic is a picture on a worksheet: an image of the report, or a barcode
// drawn as one. It sits at the top left of a cell and keeps its size in points.
type sheetPic struct {
	row, col int
	w, h     float64
	data     []byte
	ext      string // png or jpeg
}

// encode is the barcode of an element: Code 128, or a QR code.
func encode(e Element) (barcode.Barcode, error) {
	if e.Format == "code128" {
		return code128.Encode(e.Text)
	}
	return qr.Encode(e.Text, qr.M, qr.Auto)
}

// pictureOf is what a spreadsheet shows for an image or barcode element, filled
// in from sc: the image's own PNG or JPEG (a GIF becomes a PNG), or the barcode
// drawn at twice its size in points, sharp enough to scan from the screen.
func pictureOf(e Element, sc scope) (data []byte, ext string, ok bool) {
	switch e.Type {
	case "image":
		src := sc.resolve(e.Src)
		if !picture(src) {
			return nil, "", false
		}
		_, b64, _ := strings.Cut(src, ";base64,")
		raw, _ := base64.StdEncoding.DecodeString(b64)
		img, format, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, "", false
		}
		if format == "png" || format == "jpeg" {
			return raw, format, true
		}
		var b bytes.Buffer
		png.Encode(&b, img)
		return b.Bytes(), "png", true
	case "barcode":
		e.Text = sc.resolve(e.Text)
		code, err := encode(e)
		if err != nil || e.Text == "" {
			return nil, "", false
		}
		const scale = 2 // pixels per point
		cols := code.Bounds().Dx()
		var canvas *image.RGBA
		if code.Bounds().Dy() > 1 { // QR: square, in a quiet zone two modules wide
			mod := max(int(min(e.W, e.H)*scale)/(cols+4), 1)
			canvas = image.NewRGBA(image.Rect(0, 0, mod*(cols+4), mod*(cols+4)))
			draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
			scaled, err := barcode.Scale(code, mod*cols, mod*cols)
			if err != nil {
				return nil, "", false
			}
			draw.Draw(canvas, scaled.Bounds().Add(image.Pt(2*mod, 2*mod)), scaled, image.Point{}, draw.Src)
		} else { // Code 128: bars as wide and tall as the element
			w, h := max(int(e.W*scale)/cols, 1)*cols, max(int(e.H*scale), 1)
			scaled, err := barcode.Scale(code, w, h)
			if err != nil {
				return nil, "", false
			}
			canvas = image.NewRGBA(scaled.Bounds())
			draw.Draw(canvas, canvas.Bounds(), scaled, image.Point{}, draw.Src)
		}
		if hexColor.MatchString(e.Color) { // the bars in their colour
			r, g, b := rgb(e.Color)
			for i := 0; i < len(canvas.Pix); i += 4 {
				if canvas.Pix[i] == 0 {
					canvas.Pix[i], canvas.Pix[i+1], canvas.Pix[i+2] = uint8(r), uint8(g), uint8(b)
				}
			}
		}
		var b bytes.Buffer
		png.Encode(&b, canvas)
		return b.Bytes(), "png", true
	}
	return nil, "", false
}
