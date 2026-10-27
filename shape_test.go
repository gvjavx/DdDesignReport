package ddreport

import (
	"fmt"
	"strings"
	"testing"
)

func codes(s string) string {
	var out []string
	for _, r := range s {
		out = append(out, fmt.Sprintf("%04X", r))
	}
	return strings.Join(out, " ")
}

func TestVisual(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		// seen initial, then lam-alef as one letter (final, as seen joins it), meem alone: drawn right to left
		{"سلام", "ﻡﻼﺳ"},
		// reh joins nothing after it: qaf starts, meem ends
		{"رقم", "ﻢﻗﺭ"},
		// numbers in a right-to-left word keep their order, on its left
		{"رقم2026", "2026ﻢﻗﺭ"},
		// a lam-alef that starts a word is the isolated ligature
		{"لا", "ﻻ"},
		// a mark stays after its letter: beh with fatha, alone
		{"بَ", "ﺏَ"},
		// Persian peh joins both ways
		{"پپپ", "ﭗﭙﭘ"},
		// Hebrew has no joined forms, only the order
		{"שלום", "םולש"},
		// what reads left to right is left alone, Thai marks included
		{"Total", "Total"}, {"ภาษาไทย", "ภาษาไทย"}, {"Привет", "Привет"},
	} {
		if got := visual(c.in); got != c.want {
			t.Errorf("visual(%q) = %s, want %s", c.in, codes(got), codes(c.want))
		}
	}
	for s, want := range map[string]bool{"فاتورة 2026": true, "2026 فاتورة": true, "Total untuk فاتورة": false, "": false, "שלום": true} {
		if rightToLeft(s) != want {
			t.Errorf("rightToLeft(%q) = %v", s, !want)
		}
	}
}

// Right-to-left text: the words of a line in PDF go right to left, and HTML
// leaves the order to the browser.
func TestRightToLeft(t *testing.T) {
	tpl := []byte(`{"bands":{"detail":{"height":20,"elements":[{"type":"text","w":300,"h":14,"text":"{{a}} {{b}}"}]}}}`)
	html := string(render(t, tpl, []map[string]any{{"a": "سلام", "b": "عليكم"}}, "html"))
	if !strings.Contains(html, `dir="auto">سلام عليكم`) {
		t.Errorf("html: want the text as written, with dir=auto: %s", html)
	}
	if html := string(render(t, tpl, []map[string]any{{"a": "x", "b": "y"}}, "html")); strings.Contains(html, "dir=") {
		t.Error("dir on a left-to-right text")
	}
}
