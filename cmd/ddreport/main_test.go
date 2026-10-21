package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const request = `{"template":{"bands":{"detail":{"height":20,"elements":[{"type":"text","x":0,"y":0,"w":100,"h":14,"text":"{{name}}"}]}}},"data":[{"name":"Kopi"},{"name":"Teh"}]}`

func TestRender(t *testing.T) {
	out, err := render(strings.NewReader(request), "csv")
	if err != nil || string(out) != "\xef\xbb\xbfKopi\nTeh\n" {
		t.Errorf("render = %q, %v", out, err)
	}
	if _, err := render(strings.NewReader("not json"), "csv"); err == nil {
		t.Error("bad JSON: want error")
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
	post := func(path, body string) *http.Response {
		t.Helper()
		res, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res
	}

	for format, ctype := range mime {
		res := post("/render?format="+format, request)
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != ctype {
			t.Errorf("%s: status %d, type %q", format, res.StatusCode, res.Header.Get("Content-Type"))
		}
	}
	if res := post("/render?format=docx", request); res.StatusCode != 400 {
		t.Errorf("unknown format: status %d, want 400", res.StatusCode)
	}
	if res := post("/render?format=pdf", "{"); res.StatusCode != 400 {
		t.Errorf("bad body: status %d, want 400", res.StatusCode)
	}

	// the designer's own page may render; a page of another site open in the same browser may not
	for origin, want := range map[string]int{srv.URL: 200, "http://evil.example": 403, "null": 403} {
		req, _ := http.NewRequest("POST", srv.URL+"/render?format=csv", strings.NewReader(request))
		req.Header.Set("Origin", origin)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("Origin %s: status %d, want %d", origin, res.StatusCode, want)
		}
	}

	// the designer is embedded in the binary
	for _, path := range []string{"/", "/designer.js"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("GET %s: status %d", path, res.StatusCode)
		}
	}
	if res, _ := http.Get(srv.URL + "/render"); res.StatusCode == 200 {
		t.Error("GET /render: rendering must need a POST")
	}
}

func TestFont(t *testing.T) {
	t.Setenv("DDREPORT_TEST_FONT", "")
	if b, err := font("DDREPORT_TEST_FONT"); b != nil || err != nil {
		t.Errorf("unset: %v, %v", b, err)
	}
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, []byte("ttf"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DDREPORT_TEST_FONT", path)
	if b, err := font("DDREPORT_TEST_FONT"); string(b) != "ttf" || err != nil {
		t.Errorf("set: %q, %v", b, err)
	}
	t.Setenv("DDREPORT_TEST_FONT", path+".missing")
	if _, err := font("DDREPORT_TEST_FONT"); err == nil {
		t.Error("missing file: want error")
	}
}

// One request must not be able to take the server down.
func TestHandlerSurvivesHostileTemplates(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
	deep := "{\"template\":{\"bands\":{\"detail\":{\"height\":10,\"elements\":[{\"type\":\"text\",\"w\":50,\"h\":10,\"text\":\"{{" + strings.Repeat("(", 3_000_000) + "1" + strings.Repeat(")", 3_000_000) + "}}\"}]}}},\"data\":[{}]}"
	res, err := http.Post(srv.URL+"/render?format=pdf", "application/json", strings.NewReader(deep))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("deeply nested expression: status %d", res.StatusCode)
	}
	if res, err := http.Post(srv.URL+"/render?format=csv", "application/json", strings.NewReader(request)); err != nil || res.StatusCode != 200 {
		t.Errorf("server did not answer afterwards: %v", err)
	}
}
