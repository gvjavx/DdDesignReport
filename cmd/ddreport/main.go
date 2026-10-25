// Command ddreport renders a report: it reads {"template": ..., "data": ...}
// on stdin and writes the report to stdout. The Java, Kotlin and Node.js
// wrappers call this binary.
//
// With -serve it hosts the designer instead, with POST /render?format=...
// taking the same JSON body, which is what the designer's preview uses. When
// DDREPORT_TOKEN is set, POST /render needs "Authorization: Bearer <token>".
//
// With -stdio it stays running and renders one request after another, so a
// server pays for starting it once. A request is a line "<format> <length>" and
// that many bytes of {"template": ..., "data": ...}; the answer a line
// "ok <length>" and the document, or "error <length>" and the message.
//
// DDREPORT_FONT and DDREPORT_FONT_BOLD name TrueType files that replace the
// built-in Latin-only PDF font. DDREPORT_FONTS names a folder of Family-Face.ttf
// files (Inter-Regular.ttf, Inter-Bold.ttf, ...) that templates can use by name.
package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"

	ddreport "github.com/gvjavx/DdDesignReport"
	"github.com/gvjavx/DdDesignReport/designer"
)

var mime = map[string]string{
	"pdf":  "application/pdf",
	"html": "text/html; charset=utf-8",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"csv":  "text/csv; charset=utf-8",
	"xls":  "application/vnd.ms-excel",
}

func render(in io.Reader, format string) ([]byte, error) {
	var req struct{ Template, Data json.RawMessage }
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return nil, err
	}
	return ddreport.Render(req.Template, req.Data, format)
}

// handler serves the designer on / and renders on POST /render?format=...
// A token, when not empty, is what every render must send as its bearer token.
func handler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(designer.Files))
	mux.Handle("/fonts/", http.FileServerFS(ddreport.BundledFonts)) // so the designer draws the bundled fonts
	// as many renders at a time as there are processors; the others wait their
	// turn, or give up when their client does, so a burst cannot exhaust the machine
	busy := make(chan struct{}, runtime.NumCPU())
	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		// a page on another site, open in the same browser, cannot use this server:
		// browsers say where a request comes from, programs calling it send no Origin
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				http.Error(w, "ddreport: requests from other sites are refused", http.StatusForbidden)
				return
			}
		}
		if token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "ddreport: a valid token is needed (Authorization: Bearer <DDREPORT_TOKEN>)", http.StatusUnauthorized)
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
	serve := flag.String("serve", "", "serve the designer and POST /render on this address, e.g. 127.0.0.1:8080")
	flag.Parse()
	var err error
	if ddreport.Font, err = font("DDREPORT_FONT"); err == nil {
		ddreport.FontBold, err = font("DDREPORT_FONT_BOLD")
	}
	if dir := os.Getenv("DDREPORT_FONTS"); dir != "" && err == nil {
		err = ddreport.AddFonts(os.DirFS(dir))
	}
	if err != nil {
		log.Fatal(err)
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
		// time limits, so a stalled or malicious client cannot tie the server up for good
		srv := &http.Server{Addr: *serve, Handler: handler(token), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 2 * time.Minute}
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
			doc, err = ddreport.Render(req.Template, req.Data, format)
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
