package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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
	srv := httptest.NewServer(handler("", t.TempDir()))
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
	for _, path := range []string{"/", "/designer.js", "/template.schema.json"} {
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

func TestHandlerToken(t *testing.T) {
	srv := httptest.NewServer(handler("s3cret", t.TempDir()))
	defer srv.Close()
	for auth, want := range map[string]int{"": 401, "Bearer wrong": 401, "s3cret": 401, "Bearer s3cret": 200} {
		req, _ := http.NewRequest("POST", srv.URL+"/render?format=csv", strings.NewReader(request))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("Authorization %q: status %d, want %d", auth, res.StatusCode, want)
		}
	}
	if res, err := http.Get(srv.URL + "/"); err != nil || res.StatusCode != 200 {
		t.Errorf("the designer page needs no token: %v", err)
	}
}

func TestExposed(t *testing.T) {
	for addr, want := range map[string]bool{"127.0.0.1:8080": false, "localhost:8080": false, "[::1]:80": false, ":8080": true, "0.0.0.0:8080": true, "10.0.0.5:80": true, "nonsense": true} {
		if got := exposed(addr); got != want {
			t.Errorf("exposed(%q) = %v, want %v", addr, got, want)
		}
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
	srv := httptest.NewServer(handler("", t.TempDir()))
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

// -stdio answers request after request on one stream, errors included, and
// a body can never pass for the next request.
func TestServeStdio(t *testing.T) {
	req := func(format, body string) string { return format + " " + strconv.Itoa(len(body)) + "\n" + body }
	// a second request hidden in the data of the first is an error for the first, not a request
	smuggled := `{"template":{},"data":null}{"template":{},"data":null}`
	var out strings.Builder
	if err := serveStdio(strings.NewReader(req("csv", request)+req("docx", request)+req("csv", smuggled)+req("csv", request)), &out); err != nil {
		t.Fatal(err)
	}
	csv := "\xef\xbb\xbfKopi\nTeh\n"
	answers := strings.SplitAfter(out.String(), "\n")
	got := out.String()
	if !strings.HasPrefix(got, "ok 12\n"+csv+"error ") || !strings.Contains(got, `unknown format "docx"`) || !strings.HasSuffix(got, "ok 12\n"+csv) ||
		strings.Count(got, "ok 12\n") != 2 || strings.Count(got, "error ") != 2 {
		t.Errorf("answers: %q", answers)
	}
	// a request line that cannot be read ends the stream
	for _, broken := range []string{"oops", "csv -1\n", "csv 100\n{}"} {
		if err := serveStdio(strings.NewReader(broken), &out); err == nil {
			t.Errorf("%q: want error", broken)
		}
	}
}

func TestMine(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(handler("s3cret", dir))
	defer srv.Close()
	do := func(method, path, body string, auth bool) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if auth {
			req.Header.Set("Authorization", "Bearer s3cret")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, list := do("GET", "/mine", "", true); code != 200 || strings.TrimSpace(list) != "[]" {
		t.Errorf("empty: %d %s", code, list)
	}
	if code, _ := do("PUT", "/mine/a1", `{"name":"Faktur","template":{}}`, false); code != 401 {
		t.Errorf("without the token: %d, want 401", code)
	}
	for path, body := range map[string]string{"/mine/..%2Fx": `{"name":"x","template":{}}`, "/mine/A": `{"name":"x","template":{}}`, "/mine/b": `{"template":{}}`, "/mine/c": `{"name":"x"}`, "/mine/d": "{"} {
		if code, _ := do("PUT", path, body, true); code != 400 {
			t.Errorf("PUT %s %s: %d, want 400", path, body, code)
		}
	}
	if code, _ := do("PUT", "/mine/a1", `{"id":"other","name":"Faktur","template":{"bands":{}}}`, true); code != 204 {
		t.Errorf("save: %d", code)
	}
	if code, _ := do("PUT", "/mine/a1", `{"name":"Faktur baru","template":{}}`, true); code != 204 {
		t.Errorf("update: %d", code)
	}
	if _, list := do("GET", "/mine", "", true); !strings.Contains(list, `"name":"Faktur baru"`) || !strings.Contains(list, `"id":"a1"`) || strings.Count(list, `"id"`) != 1 {
		t.Errorf("list: %s", list)
	}
	if tmp, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(tmp) != 0 { // no half-written file left
		t.Errorf("left behind: %v", tmp)
	}
	if code, _ := do("DELETE", "/mine/a1", "", true); code != 204 {
		t.Errorf("delete: %d", code)
	}
	if _, list := do("GET", "/mine", "", true); strings.TrimSpace(list) != "[]" {
		t.Errorf("after delete: %s", list)
	}
}

func TestFonts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Mine-Regular.ttf"), []byte("own font"), 0o644)
	t.Setenv("DDREPORT_FONTS", dir)
	srv := httptest.NewServer(handler("", t.TempDir()))
	defer srv.Close()
	get := func(path string) (int, string) {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, body := get("/fonts/Mine-Regular.ttf"); code != 200 || body != "own font" {
		t.Errorf("a DDREPORT_FONTS file: %d %q", code, body)
	}
	if code, _ := get("/fonts/Roboto-Bold.ttf"); code != 200 {
		t.Errorf("a bundled file: %d", code)
	}
	if code, list := get("/fonts.json"); code != 200 || !strings.Contains(list, `"Great Vibes":["Regular"]`) || !strings.Contains(list, `"Roboto":[`) {
		t.Errorf("fonts.json: %d %s", code, list)
	}
}

func TestMineHistory(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(handler("", dir))
	defer srv.Close()
	do := func(method, path, body string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, list := do("GET", "/mine/a/history", ""); code != 200 || strings.TrimSpace(list) != "[]" {
		t.Errorf("no history yet: %d %s", code, list)
	}
	for i := range 25 {
		if code, _ := do("PUT", "/mine/a", fmt.Sprintf(`{"name":"v%d","template":{},"updated":%d}`, i, i)); code != 204 {
			t.Fatalf("save %d: %d", i, code)
		}
	}
	var list []struct{ Version, Name string }
	_, body := do("GET", "/mine/a/history", "")
	json.Unmarshal([]byte(body), &list)
	if len(list) != 20 || list[0].Name != "v23" || list[19].Name != "v4" {
		t.Fatalf("the last 20 earlier versions, newest first: %s", body)
	}
	if code, old := do("GET", "/mine/a/history/"+list[0].Version, ""); code != 200 || !strings.Contains(old, `"name":"v23"`) {
		t.Errorf("an earlier version: %d %s", code, old)
	}
	if code, _ := do("GET", "/mine/a/history/..%2F..%2Fa", ""); code != 400 {
		t.Errorf("a version that is not one: %d", code)
	}
	if _, now := do("GET", "/mine", ""); !strings.Contains(now, `"name":"v24"`) || strings.Count(now, `"id"`) != 1 {
		t.Errorf("the list has the template once, as it is now: %s", now)
	}
	do("DELETE", "/mine/a", "")
	if _, err := os.Stat(filepath.Join(dir, "history", "a")); !os.IsNotExist(err) {
		t.Error("deleting a template keeps its history")
	}
}

func TestJRXMLOverHTTP(t *testing.T) {
	srv := httptest.NewServer(handler("", t.TempDir()))
	defer srv.Close()
	res, err := http.Post(srv.URL+"/render?format=jrxml", "application/json", strings.NewReader(request))
	if err != nil {
		t.Fatal(err)
	}
	jr, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(jr), "<jasperReport ") || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/xml") {
		t.Fatalf("export: %d %s", res.StatusCode, jr)
	}
	res, err = http.Post(srv.URL+"/import/jrxml", "application/xml", bytes.NewReader(jr))
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Template map[string]any
		Notes    []string
	}
	json.NewDecoder(res.Body).Decode(&back)
	res.Body.Close()
	if res.StatusCode != 200 || back.Template["bands"] == nil || back.Notes == nil {
		t.Errorf("import: %d %v", res.StatusCode, back)
	}
	if res, _ := http.Post(srv.URL+"/import/jrxml", "application/xml", strings.NewReader("<html/>")); res.StatusCode != 400 {
		t.Errorf("not a report: %d", res.StatusCode)
	}
}

// Without a token, a request by a name other than localhost or an IP address is
// refused: a site whose name points at this machine (DNS rebinding) sends its own.
func TestHostWithoutToken(t *testing.T) {
	for token, want := range map[string]int{"": 403, "s3cret": 200} {
		srv := httptest.NewServer(handler(token, t.TempDir()))
		for _, path := range []string{"/mine", "/render?format=csv"} {
			method, body := "GET", ""
			if path != "/mine" {
				method, body = "POST", request
			}
			req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
			req.Host = "evil.example:8080"
			req.Header.Set("Origin", "http://evil.example:8080")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != want {
				t.Errorf("token %q, %s: %d, want %d", token, path, res.StatusCode, want)
			}
		}
		srv.Close()
	}
	for host, want := range map[string]bool{"localhost:8080": true, "127.0.0.1:8080": true, "[::1]:8080": true, "192.168.1.5": true, "evil.example": false, "localhost.evil.example:80": false} {
		if localName(host) != want {
			t.Errorf("localName(%q) = %v", host, !want)
		}
	}
}

func TestFontsInFontsFolder(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "fonts"), 0o755)
	os.WriteFile(filepath.Join(dir, "fonts", "Sub-Regular.ttf"), []byte("in fonts/"), 0o644)
	t.Setenv("DDREPORT_FONTS", dir)
	srv := httptest.NewServer(handler("", t.TempDir()))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/fonts/Sub-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(b) != "in fonts/" {
		t.Errorf("a font in fonts/: %d %q", res.StatusCode, b)
	}
}

func TestFontPackFolder(t *testing.T) {
	t.Setenv("DDREPORT_FONTS", "")
	t.Chdir(t.TempDir())
	if d := fontsFolder(); d != "" {
		t.Errorf("no folder: %q", d)
	}
	os.Mkdir("ddreport-fonts", 0o755)
	if d := fontsFolder(); d != "ddreport-fonts" {
		t.Errorf("the downloaded font pack: %q", d)
	}
	t.Setenv("DDREPORT_FONTS", "elsewhere")
	if d := fontsFolder(); d != "elsewhere" {
		t.Errorf("DDREPORT_FONTS first: %q", d)
	}
}

func TestHealthAndFullQueue(t *testing.T) {
	srv := httptest.NewServer(handler("s3cret", t.TempDir()))
	defer srv.Close()
	if res, err := http.Get(srv.URL + "/healthz"); err != nil || res.StatusCode != 200 {
		t.Fatalf("/healthz without the token: %v %v", res, err)
	}

	// renders whose body never ends hold every place in the queue
	var pipes []*io.PipeWriter
	defer func() {
		for _, p := range pipes {
			p.Close()
		}
	}()
	for range 4 * runtime.NumCPU() {
		pr, pw := io.Pipe()
		pipes = append(pipes, pw)
		req, _ := http.NewRequest("POST", srv.URL+"/render?format=csv", pr)
		req.Header.Set("Authorization", "Bearer s3cret")
		go http.DefaultClient.Do(req)
	}
	// a try that got into the queue before it filled waits behind the held renders;
	// it gives up after a second and leaves its place
	try := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for {
		req, _ := http.NewRequest("POST", srv.URL+"/render?format=csv", strings.NewReader(request))
		req.Header.Set("Authorization", "Bearer s3cret")
		res, err := try.Do(req)
		if err != nil {
			if time.Now().After(deadline) {
				t.Fatal(err)
			}
			continue
		}
		res.Body.Close()
		if res.StatusCode == 503 && res.Header.Get("Retry-After") != "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("full queue: status %d, want 503 with Retry-After", res.StatusCode)
		}
	}
}

func TestMCP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.pdf")
	tpl := `{"version":1,"bands":{"detail":{"height":20,"elements":[{"type":"text","x":0,"y":0,"w":100,"h":14,"text":"{{name}}"}]}}}`
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check_template","arguments":{"template":` + tpl + `}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"check_template","arguments":{"template":{"bands":{"detail":{"hieght":20}}}}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"render_template","arguments":{"template":` + tpl + `,"data":[{"name":"Kopi"}],"format":"pdf","path":` + strconv.Quote(path) + `}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"template_guide","arguments":{}}}`,
	}, "\n")
	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 { // the notification gets no answer
		t.Fatalf("%d answers, want 6:\n%s", len(lines), out.String())
	}
	for i, want := range []string{`"protocolVersion":"2025-06-18"`, `"check_template"`, `ok: the template is valid`, `unknown field \"hieght\"`, `wrote `, `## JSON Schema`} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("answer %d lacks %s: %s", i+1, want, lines[i][:min(len(lines[i]), 300)])
		}
	}
	if b, err := os.ReadFile(path); err != nil || !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Errorf("rendered file: %v", err)
	}
}
