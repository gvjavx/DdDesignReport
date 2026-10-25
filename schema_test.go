package ddreport

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func loadSchema(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("designer/template.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// The schema has exactly the template's fields: one added to the Go types
// without the schema (or the other way round) fails here.
func TestSchemaMatchesTheTypes(t *testing.T) {
	s := loadSchema(t)
	defs := s["$defs"].(map[string]any)
	props := func(m any) []string {
		var out []string
		for k := range m.(map[string]any)["properties"].(map[string]any) {
			out = append(out, strings.ToLower(k))
		}
		slices.Sort(out)
		return out
	}
	fields := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := range rt.NumField() {
			if f := rt.Field(i); f.IsExported() && f.Name != "Points" { // Points is internal: charts and barcodes
				out = append(out, strings.ToLower(f.Name)) // encoding/json matches names regardless of case
			}
		}
		slices.Sort(out)
		return out
	}
	for name, c := range map[string]struct{ schema, typ any }{
		"template": {s, Template{}},
		"bands":    {s["properties"].(map[string]any)["bands"], Template{}.Bands},
		"page":     {defs["page"], Page{}},
		"band":     {defs["band"], Band{}},
		"element":  {defs["element"], Element{}},
		"cell":     {defs["cellStyle"], CellStyle{}},
	} {
		if got, want := props(c.schema), fields(c.typ); !slices.Equal(got, want) {
			t.Errorf("%s: schema has %v\n Go has %v", name, got, want)
		}
	}
}

// validate checks a document against the parts of JSON Schema the template
// schema uses: $ref, type, enum, pattern, properties, additionalProperties,
// propertyNames, items and required. It returns the first problem.
func validate(root, s map[string]any, v any, at string) error {
	if ref, ok := s["$ref"].(string); ok {
		target := root
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#"), "/")[1:] {
			target = target[part].(map[string]any)
		}
		if err := validate(root, target, v, at); err != nil {
			return err
		}
	}
	if types, ok := s["type"]; ok {
		var list []any
		if l, ok := types.([]any); ok {
			list = l
		} else {
			list = []any{types}
		}
		if !slices.ContainsFunc(list, func(t any) bool { return is(t.(string), v) }) {
			return fmt.Errorf("%s: %v is not %v", at, v, types)
		}
	}
	if enum, ok := s["enum"].([]any); ok && !slices.Contains(enum, v) {
		return fmt.Errorf("%s: %q is not one of %v", at, v, enum)
	}
	if p, ok := s["pattern"].(string); ok {
		if str, ok := v.(string); ok && !regexp.MustCompile(p).MatchString(str) {
			return fmt.Errorf("%s: %q does not match %s", at, str, p)
		}
	}
	if obj, ok := v.(map[string]any); ok {
		required, _ := s["required"].([]any)
		for _, r := range required {
			if _, has := obj[r.(string)]; !has {
				return fmt.Errorf("%s: %s is missing", at, r)
			}
		}
		props, _ := s["properties"].(map[string]any)
		keys := slices.Sorted(func(yield func(string) bool) {
			for k := range obj {
				if !yield(k) {
					return
				}
			}
		})
		for _, k := range keys {
			if names, ok := s["propertyNames"].(map[string]any); ok {
				if err := validate(root, names, k, at+" key"); err != nil {
					return err
				}
			}
			sub, ok := props[k].(map[string]any)
			if !ok {
				switch extra := s["additionalProperties"].(type) {
				case bool:
					if !extra {
						return fmt.Errorf("%s: unknown property %q", at, k)
					}
					continue
				case map[string]any:
					sub = extra
				default:
					continue
				}
			}
			if err := validate(root, sub, obj[k], at+"."+k); err != nil {
				return err
			}
		}
	}
	if list, ok := v.([]any); ok {
		if items, ok := s["items"].(map[string]any); ok {
			for i, x := range list {
				if err := validate(root, items, x, fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func is(typ string, v any) bool {
	switch typ {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "null":
		return v == nil
	}
	return false
}

func TestSchemaAcceptsTheTemplatesAndCatchesMistakes(t *testing.T) {
	s := loadSchema(t)
	check := func(doc string) error {
		var v any
		if err := json.Unmarshal([]byte(doc), &v); err != nil {
			t.Fatal(err)
		}
		return validate(s, s, v, "template")
	}
	files, _ := filepath.Glob("designer/templates/*.json")
	for _, f := range files {
		if filepath.Base(f) == "index.json" {
			continue
		}
		raw, _ := os.ReadFile(f)
		var file struct{ Template json.RawMessage }
		json.Unmarshal(raw, &file)
		if err := check(string(file.Template)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	ok := `{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":100,"h":14,"text":"{{amt}}","color":"{{if(amt < 0, '#dc2626', '')}}"},
	  {"type":"table","w":100,"h":20,"cells":[["a"]],"cellStyle":{"0,0":{"colSpan":1}},"padding":null},
	  {"type":"subreport","w":100,"h":20,"data":"items","report":{"bands":{"detail":{"height":10}}}}]}}}`
	if err := check(ok); err != nil {
		t.Errorf("valid template: %v", err)
	}
	for _, bad := range []string{
		`{"bands":{"detail":{"elements":[{"type":"text","colour":"#ff0000"}]}}}`,    // unknown property
		`{"bands":{"detail":{"elements":[{"type":"textbox"}]}}}`,                    // unknown type
		`{"bands":{"detail":{"elements":[{"w":10}]}}}`,                              // no type
		`{"bands":{"detail":{"elements":[{"type":"text","color":"red"}]}}}`,         // not #rrggbb
		`{"bands":{"detail":{"elements":[{"type":"text","fontSize":"10"}]}}}`,       // a number as text
		`{"bands":{"details":{}}}`,                                                  // unknown band
		`{"bands":{"detail":{"elements":[{"type":"table","cellStyle":{"a":{}}}]}}}`, // key not "row,column"
		`{"bands":{"detail":{"elements":[{"type":"table","cellStyle":{"0,0":{"colSpan":1.5}}}]}}}`,
		`{"bands":{"detail":{"elements":[{"type":"subreport","report":{"page":{"size":"A4"}}}]}}}`, // inside a sub-report too
	} {
		if check(bad) == nil {
			t.Errorf("accepted: %s", bad)
		}
	}
}

// The example in llms.txt is what an LLM copies: it must be valid and render.
func TestLLMsExample(t *testing.T) {
	raw, err := os.ReadFile("llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, block, _ := strings.Cut(string(raw), "```json\n")
	block, _, _ = strings.Cut(block, "```")
	var file struct{ Template, Data json.RawMessage }
	if err := json.Unmarshal([]byte(block), &file); err != nil {
		t.Fatalf("example: %v", err)
	}
	var tpl any
	json.Unmarshal(file.Template, &tpl)
	s := loadSchema(t)
	if err := validate(s, s, tpl, "template"); err != nil {
		t.Error(err)
	}
	for _, format := range []string{"pdf", "xlsx", "xls", "csv"} {
		if _, err := Render(file.Template, file.Data, format); err != nil {
			t.Errorf("%s: %v", format, err)
		}
	}
	html := string(render(t, file.Template, file.Data, "html"))
	for _, want := range []string{"INVOICE INV-001", "26 Oktober 2026", "54.000", "-4.000", "color:#dc2626", "Page 1 of 1"} {
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if i := strings.Index(html, "{{"); i >= 0 {
		t.Errorf("unresolved placeholder: %s", html[i:min(i+40, len(html))])
	}
}
