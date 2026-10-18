// Package web serves a read-only HTML view of the tickets for `tk serve`: the
// outline at / and one page per ticket at /t/<id>. Every request re-reads the
// tree under a shared lock, so refreshing the browser shows the current state.
// Nothing here writes to the store.
package web

import (
	"bytes"
	_ "embed"
	"html/template"
	"net"
	"net/http"
	"strings"

	"github.com/h2oai/tk/internal/render"
	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed page.html
var pageHTML string

var (
	pages = template.Must(template.New("page").Parse(pageHTML))
	// md leaves raw HTML out of ticket bodies and drops javascript: links,
	// goldmark's defaults without html.WithUnsafe.
	md = goldmark.New(goldmark.WithExtensions(extension.GFM))
)

type server struct {
	load func() (*tree.Tree, error)
	lock func(exclusive bool) (func(), error)
}

// Handler serves the pages. load reads the tree and lock takes the store lock;
// a request holds the shared lock only while it loads.
func Handler(load func() (*tree.Tree, error), lock func(exclusive bool) (func(), error)) http.Handler {
	s := &server{load: load, lock: lock}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.outline)
	mux.HandleFunc("GET /t/{id}", s.ticket)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ticket bodies are rendered markdown: never let them run script.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src * data:")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

// LocalOnly refuses requests whose Host is not localhost or a loopback
// address, so a web page cannot read the tickets through DNS rebinding. Use it
// when listening on a loopback address.
func LocalOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = strings.Trim(r.Host, "[]")
		}
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "forbidden host "+r.Host, http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// page is the data for page.html. Exactly one of Rows (with Ready), Doc or
// Err is the content.
type page struct {
	Title    string
	Problems int // fsck problem count, shown as a banner
	Filter   string
	Ready    *ready
	Rows     []row
	Doc      template.HTML
	Err      string
}

// ready is the line `tk ready` would print: Ticket is nil when nothing is
// left to do, Block is set when the top leaf is blocked.
type ready struct {
	Ticket *store.Ticket
	Pos    string
	Block  *tree.Block
}

// row is one outline line, as `tk ls` prints it.
type row struct {
	Depth    int
	Pos      string
	Ticket   *store.Ticket
	Blockers []string // unclosed blocked-by ids
	Ready    bool     // the ticket `tk ready` stops at
}

func (s *server) outline(w http.ResponseWriter, r *http.Request) {
	t, ok := s.tree(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	f := tree.Filter{All: q.Has("all") || q.Has("archived"), Archived: q.Has("archived")}
	p := page{Title: "tk", Problems: len(t.Fsck()), Filter: "open"}
	switch {
	case f.Archived:
		p.Filter = "archived"
	case f.All:
		p.Filter = "all"
	}
	res, err := t.Ready("")
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	p.Ready = &ready{Ticket: res.Ticket, Block: res.Block}
	if res.Ticket != nil {
		p.Ready.Pos = t.Position(res.Ticket.ID)
	}
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		tk := t.Get(id)
		rw := row{Depth: depth, Pos: t.Position(id), Ticket: tk, Ready: res.Ticket != nil && res.Ticket.ID == id}
		for _, b := range tk.BlockedBy {
			if bt := t.Get(b); bt != nil && bt.Status != store.StatusClosed {
				rw.Blockers = append(rw.Blockers, b)
			}
		}
		p.Rows = append(p.Rows, rw)
		for _, c := range t.Children(id) {
			if f.Shows(t, c) {
				walk(c, depth+1)
			}
		}
	}
	for _, id := range t.Roots() {
		if t.Get(id) != nil && f.Shows(t, id) {
			walk(id, 0)
		}
	}
	s.write(w, http.StatusOK, p)
}

func (s *server) ticket(w http.ResponseWriter, r *http.Request) {
	t, ok := s.tree(w)
	if !ok {
		return
	}
	arg := r.PathValue("id")
	id, err := t.Resolve(arg)
	if err != nil {
		s.fail(w, http.StatusNotFound, err.Error())
		return
	}
	if id != arg {
		http.Redirect(w, r, "/t/"+id, http.StatusFound)
		return
	}
	src := render.Linkify(render.Markdown(t, id), func(word string) string {
		if t.Get(word) != nil {
			return "/t/" + word
		}
		return ""
	})
	var b bytes.Buffer
	if err := md.Convert([]byte(src), &b); err != nil {
		s.fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.write(w, http.StatusOK, page{Title: t.Get(id).Title, Problems: len(t.Fsck()), Doc: template.HTML(b.String())})
}

// tree loads the tree under the shared lock, or writes an error page.
func (s *server) tree(w http.ResponseWriter) (*tree.Tree, bool) {
	unlock, err := s.lock(false)
	if err != nil {
		s.fail(w, http.StatusServiceUnavailable, err.Error())
		return nil, false
	}
	defer unlock()
	t, err := s.load()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	return t, true
}

func (s *server) fail(w http.ResponseWriter, code int, msg string) {
	s.write(w, code, page{Title: http.StatusText(code), Err: msg})
}

// write renders p into a buffer first, so a template error is a clean 500.
func (s *server) write(w http.ResponseWriter, code int, p page) {
	var b bytes.Buffer
	if err := pages.Execute(&b, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(b.Bytes())
}
