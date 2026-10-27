// Command ddreport renders a report: it reads {"template": ..., "data": ...}
// on stdin and writes the report to stdout. The Java, Kotlin and Node.js
// wrappers call this binary.
//
// With -serve it hosts the designer instead, with POST /render?format=...
// taking the same JSON body, which is what the designer's preview uses. When
// DDREPORT_TOKEN is set, POST /render needs "Authorization: Bearer <token>".
// Without a token it only answers to localhost and IP addresses, so a site that
// points its own name at this machine (DNS rebinding) cannot use it.
// The designer's "My templates" are kept on the server, one JSON file each in
// the -templates folder: GET /mine lists them, PUT and DELETE /mine/{id} change
// one, GET /mine/{id}/history and /mine/{id}/history/{version} give the last
// 20 versions of each. POST /import/jrxml reads a JasperReports report as a
// template. They need the token too.
//
// With -from-jrxml it reads a .jrxml on stdin and writes the template; the
// "jrxml" format writes a template as one. DDREPORT_SIGN_CERT and
// DDREPORT_SIGN_KEY (PEM) sign every PDF, with DDREPORT_SIGN_REASON as the reason.
//
// With -stdio it stays running and renders one request after another, so a
// server pays for starting it once. A request is a line "<format> <length>" and
// that many bytes of {"template": ..., "data": ...}; the answer a line
// "ok <length>" and the document, or "error <length>" and the message.
//
// DDREPORT_FONT and DDREPORT_FONT_BOLD name TrueType files that replace the
// built-in Latin-only PDF font. DDREPORT_FONTS names a folder of Family-Face.ttf
// files (Inter-Regular.ttf, Inter-Bold.ttf, ...) that templates can use by name;
// -serve hands them to the designer too, and lists every font at GET /fonts.json.
package main

import (
	"bufio"
	"cmp"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	ddreport "github.com/gvjavx/DdDesignReport"
	"github.com/gvjavx/DdDesignReport/designer"
)

var mime = map[string]string{
	"pdf":   "application/pdf",
	"html":  "text/html; charset=utf-8",
	"xlsx":  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"csv":   "text/csv; charset=utf-8",
	"xls":   "application/vnd.ms-excel",
	"jrxml": "application/xml; charset=utf-8",
}

func render(in io.Reader, format string) ([]byte, error) {
	var req struct{ Template, Data json.RawMessage }
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return nil, err
	}
	return renderSigned(req.Template, req.Data, format)
}

// signCert, signKey and signReason sign every PDF, when DDREPORT_SIGN_CERT and
// DDREPORT_SIGN_KEY name a certificate and its key (PEM).
var signCert, signKey []byte
var signReason string

// renderSigned renders, and signs a PDF when there is a key to sign with.
func renderSigned(template, data json.RawMessage, format string) ([]byte, error) {
	doc, err := ddreport.Render(template, data, format)
	if err != nil || format != "pdf" || signKey == nil {
		return doc, err
	}
	return ddreport.SignPDF(doc, signCert, signKey, signReason)
}

// loadSigner reads the certificate and key DDREPORT_SIGN_CERT and DDREPORT_SIGN_KEY name, if both are set.
func loadSigner() error {
	cert, key := os.Getenv("DDREPORT_SIGN_CERT"), os.Getenv("DDREPORT_SIGN_KEY")
	if cert == "" && key == "" {
		return nil
	}
	if cert == "" || key == "" {
		return errors.New("ddreport: set both DDREPORT_SIGN_CERT and DDREPORT_SIGN_KEY to sign PDFs")
	}
	var err error
	if signCert, err = os.ReadFile(cert); err == nil {
		signKey, err = os.ReadFile(key)
	}
	if err != nil {
		return err
	}
	signReason = os.Getenv("DDREPORT_SIGN_REASON")
	// a key that cannot sign says so now, not on the first report
	_, err = ddreport.SignPDF(nil, signCert, signKey, "")
	if err != nil && !strings.Contains(err.Error(), "not a PDF") {
		return err
	}
	return nil
}

// handler serves the designer on / and renders on POST /render?format=...
// A token, when not empty, is what every render must send as its bearer token.
// dir is where the designer's own templates (My templates) are kept.
func handler(token, dir string) http.Handler {
	mux := http.NewServeMux()
	// allowed refuses a request from a page of another site, or without the token
	allowed := func(w http.ResponseWriter, r *http.Request) bool {
		// a page on another site, open in the same browser, cannot use this server:
		// browsers say where a request comes from, programs calling it send no Origin
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				http.Error(w, "ddreport: requests from other sites are refused", http.StatusForbidden)
				return false
			}
		}
		// without a token, only by localhost or an IP address: a site whose name was
		// pointed at this machine (DNS rebinding) has the browser send its own name
		if token == "" && !localName(r.Host) {
			http.Error(w, "ddreport: set DDREPORT_TOKEN to use this server by a name other than localhost or its IP address", http.StatusForbidden)
			return false
		}
		if token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "ddreport: a valid token is needed (Authorization: Bearer <DDREPORT_TOKEN>)", http.StatusUnauthorized)
			return false
		}
		return true
	}
	mine(mux, dir, allowed)
	mux.Handle("/", http.FileServerFS(designer.Files))
	// so the designer draws the bundled fonts, and those of DDREPORT_FONTS before them
	fonts := http.FileServerFS(ddreport.BundledFonts)
	if d := os.Getenv("DDREPORT_FONTS"); d != "" {
		dir, bundled := os.DirFS(d), fonts
		own := http.FileServerFS(dir)
		fonts = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// at the folder's root, or in a fonts/ folder in it, as AddFonts reads them
			name := strings.TrimPrefix(r.URL.Path, "/fonts/")
			for _, p := range []string{name, "fonts/" + name} {
				if st, err := fs.Stat(dir, p); err == nil && !st.IsDir() {
					r2 := *r
					u := *r.URL
					u.Path = "/" + p
					r2.URL = &u
					own.ServeHTTP(w, &r2)
					return
				}
			}
			bundled.ServeHTTP(w, r)
		})
	}
	mux.Handle("/fonts/", fonts)
	// every font a template can name, with the faces it has, for the designer's font menu
	mux.HandleFunc("GET /fonts.json", func(w http.ResponseWriter, r *http.Request) {
		list := map[string][]string{}
		for name, f := range ddreport.Families {
			list[name] = f.Faces()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})
	// as many renders at a time as there are processors; the others wait their
	// turn, or give up when their client does, so a burst cannot exhaust the machine
	busy := make(chan struct{}, runtime.NumCPU())
	// a JasperReports report read as a template: {"template": ..., "notes": [...]}
	mux.HandleFunc("POST /import/jrxml", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(w, r) {
			return
		}
		in, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tpl, notes, err := ddreport.FromJRXML(in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"template": json.RawMessage(tpl), "notes": append([]string{}, notes...)})
	})
	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(w, r) {
			return
		}
		select {
		case busy <- struct{}{}:
			defer func() { <-busy }()
		case <-r.Context().Done():
			return
		}
		f := r.URL.Query().Get("format")
		out, err := render(http.MaxBytesReader(w, r.Body, 32<<20), f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", mime[f])
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(out)
	})
	return mux
}

// an id names a template's file, so it can never be a path
var validID = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// mine serves the designer's own templates, one file <id>.json each in dir.
// ponytail: GET /mine sends them all at once; page through them if there are ever hundreds
func mine(mux *http.ServeMux, dir string, allowed func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /mine", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(w, r) {
			return
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.json")) // sorted by id, which is when it was made
		list := []json.RawMessage{}
		for _, f := range files {
			if b, err := os.ReadFile(f); err == nil && json.Valid(b) {
				list = append(list, b)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})
	mux.HandleFunc("PUT /mine/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !allowed(w, r) {
			return
		}
		if !validID.MatchString(id) {
			http.Error(w, "ddreport: not a template id", http.StatusBadRequest)
			return
		}
		var t map[string]json.RawMessage
		var name string
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&t); err != nil || json.Unmarshal(t["name"], &name) != nil || name == "" || t["template"] == nil {
			http.Error(w, "ddreport: a template needs a name and a template", http.StatusBadRequest)
			return
		}
		t["id"], _ = json.Marshal(id)
		b, _ := json.Marshal(t)
		// the version it replaces goes to its history first
		if err := keepVersion(dir, id); err != nil {
			http.Error(w, "ddreport: could not keep the previous version: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// written beside it and then moved in place, so a failed write never leaves half a template
		tmp := ""
		err := os.MkdirAll(dir, 0o755)
		if err == nil { // a name of its own, so two saves at once do not write into one file
			var f *os.File
			if f, err = os.CreateTemp(dir, id+"-*.tmp"); err == nil {
				tmp = f.Name()
				_, err = f.Write(b)
				if cerr := f.Close(); err == nil {
					err = cerr
				}
			}
		}
		if err == nil {
			err = os.Rename(tmp, filepath.Join(dir, id+".json"))
		}
		if err != nil {
			if tmp != "" {
				os.Remove(tmp)
			}
			http.Error(w, "ddreport: could not save the template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// the earlier versions of a template, newest first: when each was replaced, and its name then
	mux.HandleFunc("GET /mine/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !allowed(w, r) {
			return
		}
		if !validID.MatchString(id) {
			http.Error(w, "ddreport: not a template id", http.StatusBadRequest)
			return
		}
		type version struct {
			Version string          `json:"version"`
			Name    json.RawMessage `json:"name"`
			Updated json.RawMessage `json:"updated"`
		}
		list := []version{}
		for _, v := range history(dir, id) {
			var t version
			if b, err := os.ReadFile(filepath.Join(dir, "history", id, v+".json")); err == nil && json.Unmarshal(b, &t) == nil {
				t.Version = v
				list = append(list, t)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})
	// one earlier version, whole: open it, or PUT it back to restore it
	mux.HandleFunc("GET /mine/{id}/history/{version}", func(w http.ResponseWriter, r *http.Request) {
		id, v := r.PathValue("id"), r.PathValue("version")
		if !allowed(w, r) {
			return
		}
		if !validID.MatchString(id) || !validVersion.MatchString(v) {
			http.Error(w, "ddreport: not a template version", http.StatusBadRequest)
			return
		}
		b, err := os.ReadFile(filepath.Join(dir, "history", id, v+".json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	mux.HandleFunc("DELETE /mine/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !allowed(w, r) {
			return
		}
		if !validID.MatchString(id) {
			http.Error(w, "ddreport: not a template id", http.StatusBadRequest)
			return
		}
		os.RemoveAll(filepath.Join(dir, "history", id)) // deleted for good: its old versions too
		if err := os.Remove(filepath.Join(dir, id+".json")); err != nil && !os.IsNotExist(err) {
			http.Error(w, "ddreport: could not delete the template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// versionsKept is how many earlier versions of each template its history keeps.
const versionsKept = 20

var validVersion = regexp.MustCompile(`^[0-9]{1,20}$`)

// keepVersion copies the template as it is now, if there is one, into
// history/<id>/<unix nanoseconds>.json, and forgets the oldest past versionsKept.
func keepVersion(dir, id string) error {
	old, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	h := filepath.Join(dir, "history", id)
	if err := os.MkdirAll(h, 0o755); err != nil {
		return err
	}
	// a new name for each, even for two saves the clock cannot tell apart (it is coarse on Windows)
	for n := time.Now().UnixNano(); ; n++ {
		f, err := os.OpenFile(filepath.Join(h, strconv.FormatInt(n, 10)+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = f.Write(old)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		break
	}
	versions := history(dir, id)
	for _, v := range versions[min(len(versions), versionsKept):] {
		os.Remove(filepath.Join(h, v+".json"))
	}
	return nil
}

// history names the kept versions of a template, newest first.
func history(dir, id string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "history", id, "*.json"))
	var versions []string
	for _, f := range files {
		if v := strings.TrimSuffix(filepath.Base(f), ".json"); validVersion.MatchString(v) {
			versions = append(versions, v)
		}
	}
	slices.SortFunc(versions, func(a, b string) int { return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(b, a)) })
	return versions
}

// localName says whether a Host header names this machine by localhost or an IP address.
func localName(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || net.ParseIP(host) != nil
}

// exposed says whether an address to listen on is reachable from other machines.
func exposed(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	ip := net.ParseIP(host)
	return host != "localhost" && (ip == nil || !ip.IsLoopback())
}

// font reads the TrueType file an environment variable points to, if any.
func font(env string) ([]byte, error) {
	path := os.Getenv(env)
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}

func main() {
	log.SetFlags(0)
	format := flag.String("format", "pdf", "pdf, html, xlsx, xls or csv")
	stdio := flag.Bool("stdio", false, "keep running and render one request after another from stdin (for the Java and Node.js renderers)")
	fromJRXML := flag.Bool("from-jrxml", false, "read a JasperReports .jrxml on stdin and write it as a template; what did not come over goes to stderr")
	serve := flag.String("serve", "", "serve the designer and POST /render on this address, e.g. 127.0.0.1:8080")
	templates := flag.String("templates", "ddreport-templates", "with -serve: the folder the designer's My templates are kept in")
	flag.Parse()
	var err error
	if ddreport.Font, err = font("DDREPORT_FONT"); err == nil {
		ddreport.FontBold, err = font("DDREPORT_FONT_BOLD")
	}
	if dir := os.Getenv("DDREPORT_FONTS"); dir != "" && err == nil {
		err = ddreport.AddFonts(os.DirFS(dir))
	}
	if err == nil {
		err = loadSigner()
	}
	if err != nil {
		log.Fatal(err)
	}

	if *fromJRXML {
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		tpl, notes, err := ddreport.FromJRXML(in)
		if err != nil {
			log.Fatal(err)
		}
		for _, n := range notes {
			fmt.Fprintln(os.Stderr, "note: "+n)
		}
		os.Stdout.Write(tpl)
		return
	}
	if *stdio {
		if err := serveStdio(os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *serve != "" {
		token := os.Getenv("DDREPORT_TOKEN")
		if token == "" && exposed(*serve) {
			fmt.Fprintln(os.Stderr, "warning: anyone who can reach "+*serve+" can render; set DDREPORT_TOKEN to require a token")
		}
		if token != "" {
			fmt.Fprintln(os.Stderr, "designer on http://"+*serve+"/#token=<DDREPORT_TOKEN>")
		} else {
			fmt.Fprintln(os.Stderr, "designer on http://"+*serve)
		}
		fmt.Fprintln(os.Stderr, "My templates in "+*templates)
		// time limits, so a stalled or malicious client cannot tie the server up for good
		srv := &http.Server{Addr: *serve, Handler: handler(token, *templates), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 2 * time.Minute}
		log.Fatal(srv.ListenAndServe())
	}

	out, err := render(os.Stdin, *format)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		log.Fatal(err)
	}
}

// serveStdio renders requests one after another until stdin ends. Every
// request says how long it is, so whatever is in one can never be read as the
// next: a body that is not exactly one JSON object is an error for that request.
// A request line that cannot be read ends it, since the stream cannot be followed.
func serveStdio(in io.Reader, out io.Writer) error {
	r, w := bufio.NewReader(in), bufio.NewWriter(out)
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF && line == "" {
			return nil
		}
		var format string
		var n int
		if _, scan := fmt.Sscanf(line, "%s %d", &format, &n); err != nil || scan != nil || n < 0 || n > 1<<30 {
			return fmt.Errorf("ddreport: not a request line: %q", line)
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return err
		}
		var req struct{ Template, Data json.RawMessage }
		doc, err := []byte(nil), json.Unmarshal(body, &req)
		if err == nil {
			doc, err = renderSigned(req.Template, req.Data, format)
		}
		if err != nil {
			fmt.Fprintf(w, "error %d\n%s", len(err.Error()), err.Error())
		} else {
			fmt.Fprintf(w, "ok %d\n", len(doc))
			w.Write(doc)
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
}
