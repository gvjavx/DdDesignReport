package ddreport

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
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
	dx       float64 // from the column's left edge, in points
	w, h     float64
	data     []byte
	ext      string      // png or jpeg
	chart    *sheetChart // instead of a picture: a chart Excel draws, and can change
}

// sheetChart is a chart element as a native spreadsheet chart, its data written into it.
type sheetChart struct {
	kind   string // bar, hbar, line, pie or donut
	labels []string
	values []float64
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

// chartXML is a spreadsheet chart with its data written into it (no cells to
// point at), in the report's colours: one series, a colour per slice for a pie.
func chartXML(c *sheetChart) string {
	var b strings.Builder
	esc := func(s string) string {
		var x strings.Builder
		xml.EscapeText(&x, []byte(s))
		return x.String()
	}
	colour := func(i int) string {
		return `<c:spPr><a:solidFill><a:srgbClr val="` + strings.ToUpper(palette[i%len(palette)][1:]) + `"/></a:solidFill></c:spPr>`
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<c:chart><c:autoTitleDeleted val="1"/><c:plotArea><c:layout/>`)
	round := c.kind == "pie" || c.kind == "donut"
	switch c.kind {
	case "pie":
		b.WriteString(`<c:pieChart><c:varyColors val="1"/>`)
	case "donut":
		b.WriteString(`<c:doughnutChart><c:varyColors val="1"/>`)
	case "line":
		b.WriteString(`<c:lineChart><c:grouping val="standard"/><c:varyColors val="0"/>`)
	default:
		dir := "col"
		if c.kind == "hbar" {
			dir = "bar"
		}
		b.WriteString(`<c:barChart><c:barDir val="` + dir + `"/><c:grouping val="clustered"/><c:varyColors val="0"/>`)
	}
	b.WriteString(`<c:ser><c:idx val="0"/><c:order val="0"/>`)
	switch {
	case round:
		for i := range c.labels {
			fmt.Fprintf(&b, `<c:dPt><c:idx val="%d"/><c:bubble3D val="0"/>%s</c:dPt>`, i, colour(i))
		}
	case c.kind == "line":
		b.WriteString(`<c:spPr><a:ln w="22225"><a:solidFill><a:srgbClr val="` + strings.ToUpper(palette[0][1:]) + `"/></a:solidFill></a:ln></c:spPr><c:marker><c:symbol val="circle"/><c:size val="5"/></c:marker>`)
	default:
		b.WriteString(colour(0) + `<c:invertIfNegative val="0"/>`)
	}
	fmt.Fprintf(&b, `<c:cat><c:strLit><c:ptCount val="%d"/>`, len(c.labels))
	for i, l := range c.labels {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, esc(l))
	}
	fmt.Fprintf(&b, `</c:strLit></c:cat><c:val><c:numLit><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, len(c.values))
	for i, v := range c.values {
		if round {
			v = max(v, 0) // as the PDF draws them: a slice cannot be negative
		}
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, str(v))
	}
	b.WriteString(`</c:numLit></c:val>`)
	if c.kind == "line" {
		b.WriteString(`<c:smooth val="0"/>`)
	}
	b.WriteString(`</c:ser>`)
	switch c.kind {
	case "pie":
		b.WriteString(`<c:firstSliceAng val="0"/></c:pieChart>`)
	case "donut":
		b.WriteString(`<c:firstSliceAng val="0"/><c:holeSize val="50"/></c:doughnutChart>`)
	case "line":
		b.WriteString(`<c:marker val="1"/><c:axId val="1"/><c:axId val="2"/></c:lineChart>`)
	default:
		b.WriteString(`<c:gapWidth val="60"/><c:axId val="1"/><c:axId val="2"/></c:barChart>`)
	}
	if !round {
		cat, val := "b", "l"
		if c.kind == "hbar" {
			cat, val = "l", "b"
		}
		b.WriteString(`<c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="` + cat + `"/><c:numFmt formatCode="General" sourceLinked="0"/><c:tickLblPos val="low"/><c:crossAx val="2"/><c:crosses val="autoZero"/></c:catAx>` +
			`<c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="` + val + `"/><c:majorGridlines/><c:numFmt formatCode="#,##0" sourceLinked="0"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>`)
	}
	b.WriteString(`</c:plotArea>`)
	if round {
		b.WriteString(`<c:legend><c:legendPos val="r"/><c:overlay val="0"/></c:legend>`)
	}
	b.WriteString(`<c:plotVisOnly val="1"/></c:chart></c:chartSpace>`)
	return b.String()
}
