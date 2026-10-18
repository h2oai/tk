package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
)

// serve builds epic(kid, done) and other, with epic blocked by other, and
// returns a test server over that store.
func serve(t *testing.T) (*httptest.Server, *tree.Tree) {
	srv, tr, _ := serveStore(t)
	return srv, tr
}

func serveStore(t *testing.T) (*httptest.Server, *tree.Tree, *store.Store) {
	t.Helper()
	st := store.New(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	tr, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	add := func(id, parent, body string) {
		tk := &store.Ticket{ID: id, Status: store.StatusOpen, Type: store.TypeTask, Effort: store.EffortNormal,
			Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Title: "T " + id, Body: body}
		if err := tr.Add(tk, parent, tree.Place{}); err != nil {
			t.Fatal(err)
		}
	}
	add("epic", "", "")
	add("kid", "epic", "")
	add("done", "epic", "")
	add("other", "", "Depends on kid, not on gone1. <script>alert(1)</script>\n\n`kid` [x](javascript:alert(1))")
	if _, err := tr.Close("done", false); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.SetType("other", store.TypeFeature); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.SetEffort("other", store.EffortHard); err != nil {
		t.Fatal(err)
	}
	lock := func(bool) (func(), error) { return func() {}, nil }
	srv := httptest.NewServer(Handler(func() (*tree.Tree, error) { return tree.Load(st) }, lock))
	t.Cleanup(srv.Close)
	return srv, tr, st
}

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusFound {
		return resp.StatusCode, resp.Header.Get("Location")
	}
	return resp.StatusCode, string(b)
}

func check(t *testing.T, body string, want, absent []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("want %q in:\n%s", w, body)
		}
	}
	for _, a := range absent {
		if strings.Contains(body, a) {
			t.Errorf("unexpected %q in:\n%s", a, body)
		}
	}
}

func TestOutline(t *testing.T) {
	srv, tr := serve(t)
	code, body := get(t, srv, "/")
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	check(t, body, []string{
		`ready: <span class="dot d-open"></span> <a href="/t/kid">kid</a> 1.1 <span class="badge ty-task">task</span> <span class="badge ef-normal">norm</span> T kid`,
		`<span class="badge ty-feature">feat</span> <span class="badge ef-hard">hard</span> <span class="title">T other</span>`,
		`class="row s-open top"`,
		`<a href="/t/epic">epic</a>`,
		`<a href="/t/other">other</a>`,
		`<b>open</b>`,
	}, []string{`/t/done"`, "problem(s)"})

	_, body = get(t, srv, "/?all")
	check(t, body, []string{`<a href="/t/done">done</a>`, `<b>all</b>`}, nil)

	if _, err := tr.Archive("done", false, false); err != nil {
		t.Fatal(err)
	}
	_, body = get(t, srv, "/?all")
	check(t, body, nil, []string{`/t/done"`})
	_, body = get(t, srv, "/?archived")
	check(t, body, []string{`<span class="pos">-</span> <span class="dot d-archived"></span> <a href="/t/done">done</a>`, `<b>archived</b>`}, nil)
	_, body = get(t, srv, "/t/epic")
	check(t, body, []string{
		`<div class="row s-closed archived" style="padding-left: calc(0 * 2ch + 4px)">`,
		`<span class="pos">-</span> <span class="dot d-archived"></span> <a href="/t/done">done</a>`,
	}, nil)
	_, body = get(t, srv, "/t/done")
	check(t, body, []string{`<span class="dot d-archived"></span> closed, archived <span class="badge ty-task">`}, nil)
}

func TestOutlineBlocked(t *testing.T) {
	srv, tr := serve(t)
	if err := tr.AddDep("epic", "other"); err != nil {
		t.Fatal(err)
	}
	_, body := get(t, srv, "/")
	check(t, body, []string{
		`blocked: <a href="/t/kid">kid</a> “T kid” waits on <a href="/t/other">other</a> “T other” (2, open)`,
		`<span class="deps">← <a href="/t/other">other</a></span>`,
	}, nil)
}

func TestTicket(t *testing.T) {
	srv, _ := serve(t)
	code, body := get(t, srv, "/t/other")
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	check(t, body, []string{
		"<title>T other</title>",
		"<h1>T other</h1>",
		`<p class="chips"><span class="dot d-open"></span> open <span class="badge ty-feature">feat</span> <span class="badge ef-hard">hard</span></p>`,
		`<p class="meta">other · 2 · 2026-01-02T03:04:05Z</p>`,
		`Depends on <a href="/t/kid">kid</a>, not on gone1.`,
		"<code>kid</code>",
	}, []string{"<script>", "javascript:", "<h2>Children</h2>", "blocked by"})

	_, body = get(t, srv, "/t/epic")
	check(t, body, []string{
		"<h2>Children</h2>",
		`<span class="pos">1.1</span> <span class="dot d-open"></span> <a href="/t/kid">kid</a>`,
		`<span class="pos">1.2</span> <span class="dot d-closed"></span> <a href="/t/done">done</a>`,
	}, []string{"<hr>"})

	_, body = get(t, srv, "/t/kid")
	check(t, body, []string{
		`<p class="meta">kid · 1.1 · 2026-01-02T03:04:05Z</p>`,
		`<span class="label">parent</span> <span class="pos">1</span> <span class="dot d-open"></span> <a href="/t/epic">epic</a>`,
	}, []string{"· under"})
}

func TestTicketBlockers(t *testing.T) {
	srv, tr, st := serveStore(t)
	if err := tr.AddDep("other", "kid"); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Start("kid"); err != nil {
		t.Fatal(err)
	}
	tk, err := st.Load("other")
	if err != nil {
		t.Fatal(err)
	}
	tk.BlockedBy = append(tk.BlockedBy, "gone1")
	if err := st.Save(tk); err != nil {
		t.Fatal(err)
	}
	_, body := get(t, srv, "/t/other")
	check(t, body, []string{
		`<div class="link s-in_progress"><span class="label">blocked by</span> <span class="pos">1.1</span> <span class="dot d-in_progress"></span> <a href="/t/kid">kid</a>`,
		`<div class="link"><span class="label"></span> <span class="missing">gone1 (missing)</span></div>`,
	}, []string{`<span class="label">parent</span>`})
	_, body = get(t, srv, "/t/kid")
	check(t, body, []string{`<span class="dot d-in_progress"></span> in progress`}, nil)
}

func TestTicketReverseLinks(t *testing.T) {
	srv, tr, st := serveStore(t)
	if err := tr.AddDep("other", "kid"); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Archive("done", false, false); err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]string{"epic": "About kid.", "done": "Was kid.", "kid": "I am kid; see `other`."} {
		tk, err := st.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		tk.Body = body
		if err := st.Save(tk); err != nil {
			t.Fatal(err)
		}
	}
	_, body := get(t, srv, "/t/kid")
	// One line per ticket, labelled on the first line of each group, in
	// group order and then outline order.
	want := []string{
		`<span class="label">parent</span> <span class="pos">1</span> <span class="dot d-open"></span> <a href="/t/epic">epic</a>`,
		`<span class="label">blocks</span> <span class="pos">2</span> <span class="dot d-open"></span> <a href="/t/other">other</a>`,
		`<span class="title">T other <span class="deps">← <a href="/t/kid">kid</a></span></span></div>`,
		`<span class="label">mentioned by</span> <span class="pos">1</span> <span class="dot d-open"></span> <a href="/t/epic">epic</a>`,
		`<div class="link s-closed archived"><span class="label"></span> <span class="pos">-</span> <span class="dot d-archived"></span> <a href="/t/done">done</a>`,
		`<span class="label"></span> <span class="pos">2</span> <span class="dot d-open"></span> <a href="/t/other">other</a>`,
	}
	check(t, body, want, nil)
	for i := 1; i < len(want); i++ {
		if strings.Index(body, want[i-1]) > strings.Index(body, want[i]) {
			t.Errorf("%q before %q", want[i], want[i-1])
		}
	}
	_, body = get(t, srv, "/t/other")
	check(t, body, []string{`<span class="label">blocked by</span>`}, []string{`<span class="label">blocks</span>`, `<span class="label">mentioned by</span>`})
}

func TestResolve(t *testing.T) {
	srv, _ := serve(t)
	if code, loc := get(t, srv, "/t/othe"); code != http.StatusFound || loc != "/t/other" {
		t.Errorf("partial id: %d %q", code, loc)
	}
	code, body := get(t, srv, "/t/nope")
	if code != http.StatusNotFound {
		t.Errorf("unknown id: %d", code)
	}
	check(t, body, []string{"not found"}, nil)
	if code, _ := get(t, srv, "/t/e"); code != http.StatusNotFound {
		t.Errorf("ambiguous id: %d", code)
	}
	if code, _ := get(t, srv, "/nope"); code != http.StatusNotFound {
		t.Errorf("unknown path: %d", code)
	}
	resp, err := http.Post(srv.URL+"/", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", resp.StatusCode)
	}
}

func TestProblemsBanner(t *testing.T) {
	srv, _, st := serveStore(t)
	tk, err := st.Load("other")
	if err != nil {
		t.Fatal(err)
	}
	tk.BlockedBy = append(tk.BlockedBy, "gone1")
	if err := st.Save(tk); err != nil {
		t.Fatal(err)
	}
	_, body := get(t, srv, "/")
	check(t, body, []string{"1 problem(s); run <code>tk fsck</code>"}, nil)
}

func TestLocalOnly(t *testing.T) {
	h := LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for host, want := range map[string]int{
		"localhost:7777": http.StatusOK,
		"127.0.0.1:7777": http.StatusOK,
		"[::1]:7777":     http.StatusOK,
		"localhost":      http.StatusOK,
		"evil.test:7777": http.StatusForbidden,
		"10.0.0.1:7777":  http.StatusForbidden,
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = host
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("host %s: %d, want %d", host, w.Code, want)
		}
	}
}
