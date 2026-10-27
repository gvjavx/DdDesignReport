package ddreport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every ready-made template must render in every format with its sample data.
func TestTemplates(t *testing.T) {
	// a template from the community also says who made it, and may link to them
	var index []struct{ ID, Name, Description, Author, URL string }
	raw, err := os.ReadFile("designer/templates/index.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &index); err != nil || len(index) == 0 {
		t.Fatalf("index.json: %v, %d templates", err, len(index))
	}
	files, _ := filepath.Glob("designer/templates/*.json")
	if len(files) != len(index)+1 {
		t.Errorf("%d files for %d templates in the index", len(files)-1, len(index))
	}
	for _, entry := range index {
		t.Run(entry.ID, func(t *testing.T) {
			// the id names the file, here and on the website
			if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(entry.ID) {
				t.Fatalf("id %q: lowercase letters, digits and dashes only", entry.ID)
			}
			if entry.URL != "" && (!strings.HasPrefix(entry.URL, "https://") || entry.Author == "") {
				t.Errorf("url %q: an https link, with the author's name", entry.URL)
			}
			raw, err := os.ReadFile("designer/templates/" + entry.ID + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var file struct {
				Name, Description string
				Template, Data    json.RawMessage
			}
			if err := json.Unmarshal(raw, &file); err != nil {
				t.Fatal(err)
			}
			if file.Name != entry.Name || file.Description == "" {
				t.Errorf("name %q / description %q do not match the index", file.Name, file.Description)
			}
			for _, format := range []string{"pdf", "html", "xlsx", "xls", "csv"} {
				if _, err := Render(file.Template, file.Data, format); err != nil {
					t.Errorf("%s: %v", format, err)
				}
			}
			// nothing in the sample data may be left unresolved
			html := string(render(t, file.Template, file.Data, "html"))
			if i := strings.Index(html, "{{"); i >= 0 {
				t.Errorf("unresolved placeholder: %s", html[i:min(i+40, len(html))])
			}
			if pdf := render(t, file.Template, file.Data, "pdf"); !bytes.HasPrefix(pdf, []byte("%PDF-")) {
				t.Error("not a PDF")
			}
		})
	}
}
