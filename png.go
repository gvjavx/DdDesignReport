package ddreport

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	icolor "image/color"
	"image/draw"
	_ "image/gif" // pictures in templates
	_ "image/jpeg"
	"image/png"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// pngScale is pixels per point: 2 is 144 dpi, sharp on a phone.
const pngScale = 2

// pngMaxPixels caps one picture at about 100 MB of memory: some 12 A4 pages.
// ponytail: one picture for all pages; a zip of one PNG per page if long reports are ever wanted as pictures
const pngMaxPixels = 25_000_000

// toPNG draws the pages one under the other in one picture, for sending in a
// chat (WhatsApp) where a PDF is one tap too many. Texts print with the
// bundled fonts as in PDF/A, so they break where the PDF's lines break.
func (r *report) toPNG(pages [][]Element) ([]byte, error) {
	const gap = 16 // px of grey between pages
	w, h := int(math.Ceil(r.t.Page.Width*pngScale)), int(math.Ceil(r.t.Page.Height*pngScale))
	total := len(pages)*(h+gap) - gap
	if w*total > pngMaxPixels {
		return nil, fmt.Errorf("ddreport: png: %d pages are too long for one picture; print it as pdf", len(pages))
	}
	img := image.NewRGBA(image.Rect(0, 0, w, total))
	draw.Draw(img, img.Bounds(), image.NewUniform(icolor.RGBA{0xe5, 0xe7, 0xeb, 0xff}), image.Point{}, draw.Src)
	faces := map[string]font.Face{}
	for n, page := range pages {
		top := n * (h + gap)
		p := img.SubImage(image.Rect(0, top, w, top+h)).(*image.RGBA)
		draw.Draw(p, p.Bounds(), image.White, image.Point{}, draw.Src)
		// pt to px on this page
		px := func(x, y float64) (float64, float64) { return x * pngScale, y*pngScale + float64(top) }
		box := func(x, y, bw, bh float64) image.Rectangle {
			x0, y0 := px(x, y)
			x1, y1 := px(x+bw, y+bh)
			return image.Rect(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1))).Intersect(p.Bounds())
		}
		fill := func(rect image.Rectangle, c string) {
			draw.Draw(p, rect, image.NewUniform(paint(c)), image.Point{}, draw.Over)
		}
		// shape fills a polygon of points in pt, antialiased
		shape := func(pts []float64, c string) {
			if len(pts) < 6 {
				return
			}
			minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
			for i := 0; i+1 < len(pts); i += 2 {
				x, y := px(pts[i], pts[i+1])
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
			b := image.Rect(int(minX), int(minY), int(math.Ceil(maxX)), int(math.Ceil(maxY))).Intersect(p.Bounds())
			if b.Empty() {
				return
			}
			z := vector.NewRasterizer(b.Dx(), b.Dy())
			for i := 0; i+1 < len(pts); i += 2 {
				x, y := px(pts[i], pts[i+1])
				x, y = x-float64(b.Min.X), y-float64(b.Min.Y)
				if i == 0 {
					z.MoveTo(float32(x), float32(y))
				} else {
					z.LineTo(float32(x), float32(y))
				}
			}
			z.ClosePath()
			z.Draw(p, b, image.NewUniform(paint(c)), image.Point{})
		}
		// line strokes from (x1, y1) to (x2, y2) in pt, width wd
		line := func(x1, y1, x2, y2, wd float64, c string) {
			l := math.Hypot(x2-x1, y2-y1)
			if l == 0 {
				return
			}
			nx, ny := -(y2-y1)/l*wd/2, (x2-x1)/l*wd/2
			shape([]float64{x1 + nx, y1 + ny, x2 + nx, y2 + ny, x2 - nx, y2 - ny, x1 - nx, y1 - ny}, c)
		}
		for _, e := range page {
			stroke := color(e.Color, "#000000")
			switch e.Type {
			case "text":
				if hexColor.MatchString(e.Fill) {
					fill(box(e.X, e.Y, e.W, e.H), e.Fill)
				}
				clip := p.SubImage(box(e.X, e.Y, e.W, e.H)).(*image.RGBA)
				lh := e.size() * e.leading()
				lines := r.layout(e)
				drop := e.drop(len(lines))
				backwards := rightToLeft(e.plain())
				for i, l := range lines {
					x := e.X
					switch e.align() {
					case "center":
						x += (e.W - l.w) / 2
					case "right":
						x += e.W - l.w
					}
					base := e.Y + drop + float64(i)*lh + lh/2 + 0.3*e.size()
					gap := e.gap(l)
					spans := l.spans
					if backwards {
						spans = bidiOrder(spans)
					}
					for _, s := range spans {
						text := visual(s.text) // as the PDF draws it: Arabic joined, right to left reversed
						dx, dy := px(x, base)
						d := font.Drawer{Dst: clip, Src: image.NewUniform(paint(stroke)), Face: r.pngFace(faces, e, s.run),
							Dot: fixed.Point26_6{X: fixed.Int26_6(dx * 64), Y: fixed.Int26_6(dy * 64)}}
						d.DrawString(text)
						if s.underline && strings.TrimSpace(s.text) != "" {
							u := e.size() / 15
							fill(box(x, base+u*1.5, s.w, u).Intersect(clip.Bounds()), stroke)
						}
						x += s.w + gap*float64(strings.Count(s.text, " "))
					}
				}
			case "rect":
				if hexColor.MatchString(e.Fill) {
					fill(box(e.X, e.Y, e.W, e.H), e.Fill)
				}
				if b := e.Border; b > 0 {
					fill(box(e.X-b/2, e.Y-b/2, e.W+b, b), stroke)
					fill(box(e.X-b/2, e.Y+e.H-b/2, e.W+b, b), stroke)
					fill(box(e.X-b/2, e.Y-b/2, b, e.H+b), stroke)
					fill(box(e.X+e.W-b/2, e.Y-b/2, b, e.H+b), stroke)
				}
			case "line":
				switch {
				case e.Slope == "down":
					line(e.X, e.Y, e.X+e.W, e.Y+e.H, e.stroke(), stroke)
				case e.Slope == "up":
					line(e.X, e.Y+e.H, e.X+e.W, e.Y, e.stroke(), stroke)
				case e.H > e.W:
					fill(box(e.X-e.stroke()/2, e.Y, max(e.stroke(), 1/pngScale), e.H), stroke)
				default:
					fill(box(e.X, e.Y-e.stroke()/2, e.W, max(e.stroke(), 1/pngScale)), stroke)
				}
			case "code": // whole pixels, so a scanner sees sharp modules
				for i := 0; i+3 < len(e.Points); i += 4 {
					fill(box(e.X+e.Points[i], e.Y+e.Points[i+1], e.Points[i+2], e.Points[i+3]), stroke)
				}
			case "poly":
				var pts []float64
				for i := 0; i+1 < len(e.Points); i += 2 {
					pts = append(pts, e.X+e.Points[i], e.Y+e.Points[i+1])
				}
				if hexColor.MatchString(e.Fill) {
					shape(pts, e.Fill)
					continue
				}
				for i := 2; i+1 < len(pts); i += 2 {
					line(pts[i-2], pts[i-1], pts[i], pts[i+1], e.stroke(), stroke)
				}
			case "image":
				_, b64, ok := strings.Cut(e.Src, ";base64,")
				raw, err := base64.StdEncoding.DecodeString(b64)
				if !ok || err != nil {
					continue
				}
				pic, _, err := image.Decode(bytes.NewReader(raw))
				if err != nil {
					continue
				}
				xdraw.CatmullRom.Scale(p, box(e.X, e.Y, e.W, e.H), pic, pic.Bounds(), draw.Over, nil)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("ddreport: png: %w", err)
	}
	return buf.Bytes(), nil
}

// pngFace is the font a run of a text draws with, at its size in pixels:
// DDREPORT_FONT when set, else the bundled family PDF/A uses for it.
func (r *report) pngFace(faces map[string]font.Face, e Element, p run) font.Face {
	ttf, name := Font, "custom"
	if p.bold && FontBold != nil {
		ttf, name = FontBold, "custom bold"
	}
	if Font == nil {
		name = pdfaFont(e.Font)
		ttf = Families[name].face(p.bold, p.italic)
	}
	key := fmt.Sprint(name, p.bold, p.italic, e.size())
	if f, ok := faces[key]; ok {
		return f
	}
	var face font.Face = noFace{}
	if f, err := opentype.Parse(ttf); err == nil {
		if ff, err := opentype.NewFace(f, &opentype.FaceOptions{Size: e.size() * pngScale, DPI: 72, Hinting: font.HintingNone}); err == nil {
			face = ff
		}
	}
	faces[key] = face
	return face
}

// noFace draws nothing: a font that cannot be read leaves its text out instead of failing the report.
type noFace struct{ font.Face }

func (noFace) Glyph(fixed.Point26_6, rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return image.Rectangle{}, nil, image.Point{}, 0, false
}
func (noFace) GlyphAdvance(rune) (fixed.Int26_6, bool) { return 0, false }
func (noFace) Kern(rune, rune) fixed.Int26_6           { return 0 }

// paint is a #rrggbb colour as an image colour, black when it is not one.
func paint(c string) icolor.RGBA {
	r, g, b := rgb(c)
	return icolor.RGBA{uint8(r), uint8(g), uint8(b), 0xff}
}
