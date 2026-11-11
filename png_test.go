package ddreport

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"strings"
	"testing"
)

// every ready-made template draws as one picture of its pages, at 2 px per pt, with ink on it
func TestPNG(t *testing.T) {
	for _, id := range []string{"invoice", "receipt", "sales-report", "certificate", "rapor"} {
		raw, err := os.ReadFile("designer/templates/" + id + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var file struct{ Template, Data json.RawMessage }
		json.Unmarshal(raw, &file)
		out, err := Render(file.Template, file.Data, "png")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		img, err := png.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		var page struct{ Page struct{ Width float64 } }
		json.Unmarshal(file.Template, &page)
		if w := img.Bounds().Dx(); w < int(page.Page.Width*pngScale) || w > int(page.Page.Width*pngScale)+1 {
			t.Errorf("%s: %d px wide for a %g pt page", id, w, page.Page.Width)
		}
		dark := 0
		for y := 0; y < img.Bounds().Dy(); y += 3 {
			for x := 0; x < img.Bounds().Dx(); x += 3 {
				if r, _, _, _ := img.At(x, y).RGBA(); r < 0x4000 {
					dark++
				}
			}
		}
		if dark < 200 {
			t.Errorf("%s: %d dark pixels, the text is missing", id, dark)
		}
	}
	// pages one under the other, but not a picture of a hundred pages
	tpl := []byte(`{"bands": {"detail": {"height": 20, "newPage": true, "elements": [{"type": "text", "w": 100, "h": 20, "text": "{{n}}"}]}}}`)
	two, _ := Render(tpl, []map[string]any{{"n": 1}, {"n": 2}}, "png")
	if img, err := png.Decode(bytes.NewReader(two)); err != nil || img.Bounds().Dy() < 2*842*pngScale {
		t.Errorf("two pages: %v", err)
	}
	many := make([]map[string]any, 100)
	if _, err := Render(tpl, many, "png"); err == nil || !strings.Contains(err.Error(), "too long for one picture") {
		t.Errorf("100 pages: err = %v", err)
	}
}
