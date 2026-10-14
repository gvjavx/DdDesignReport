package ddreport

import "testing"

func TestCSVSafe(t *testing.T) {
	for in, want := range map[string]string{
		"":         "",
		"Kopi":     "Kopi",
		"=1+1":     "'=1+1",
		"@SUM(A1)": "'@SUM(A1)",
		"+1+cmd|x": "'+1+cmd|x",
		"-2+3":     "'-2+3",
		"\tx":      "'\tx",
		"+62 812":  "'+62 812",
		"-5":       "-5",  // a number
		"-":        "-",   // a dash for "no value"
		"a=b":      "a=b", // only the first character matters
	} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}

	// numbers from the data are numbers, not text: they are never quoted
	tpl := `{"bands":{"detail":{"height":10,"elements":[{"type":"text","x":0,"text":"{{n}}"},{"type":"text","x":50,"text":"{{s}}"}]}}}`
	out, err := Render([]byte(tpl), []map[string]any{{"n": -5, "s": `=HYPERLINK("http://x")`}}, "csv")
	if want := "-5,\"'=HYPERLINK(\"\"http://x\"\")\"\n"; err != nil || string(out) != want {
		t.Errorf("csv = %q, %v, want %q", out, err, want)
	}
}
