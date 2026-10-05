package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

var ts = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   Ticket
	}{
		{"minimal", Ticket{ID: "fanir7", Status: StatusOpen, Created: ts, Title: "T"}},
		{"full", Ticket{ID: "fanir7", Status: StatusInProgress, BlockedBy: []string{"lovet2"},
			Children: []string{"kamop3", "ritus9"}, Created: ts, Title: "Add parser", Body: "Some text\n\n## Note\nmore\n"}},
		{"body no trailing newline", Ticket{ID: "a1", Status: StatusClosed, Created: ts, Title: "T", Body: "x"}},
		{"body leading blank lines", Ticket{ID: "a1", Status: StatusOpen, Created: ts, Title: "T", Body: "\n\nx\n"}},
		{"body only newline", Ticket{ID: "a1", Status: StatusOpen, Created: ts, Title: "T", Body: "\n"}},
		{"body with fences", Ticket{ID: "a1", Status: StatusOpen, Created: ts, Title: "T", Body: "---\nid: x\n---\n# H\n"}},
		{"title with colon and hash", Ticket{ID: "a1", Status: StatusOpen, Created: ts, Title: "fix: # thing"}},
		{"unicode", Ticket{ID: "a1", Status: StatusOpen, Created: ts, Title: "héllo ✓", Body: "日本語\n"}},
		{"non-utc created", Ticket{ID: "a1", Status: StatusOpen, Created: ts.In(time.FixedZone("x", 3600)), Title: "T"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Marshal(&tt.in)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Unmarshal(data)
			if err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, data)
			}
			if !got.Created.Equal(tt.in.Created) {
				t.Errorf("created = %v, want %v", got.Created, tt.in.Created)
			}
			got.Created = tt.in.Created
			if !reflect.DeepEqual(got, &tt.in) {
				t.Errorf("got %+v, want %+v", got, &tt.in)
			}
		})
	}
}

func TestMarshalFormat(t *testing.T) {
	data, err := Marshal(&Ticket{ID: "fanir7", Status: StatusOpen, BlockedBy: []string{"lovet2"},
		Children: []string{"kamop3", "ritus9"}, Created: ts, Title: "Title", Body: "Body\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nid: fanir7\nstatus: open\nblocked-by: [lovet2]\nchildren: [kamop3, ritus9]\ncreated: 2026-10-05T12:00:00Z\n---\n# Title\n\nBody\n"
	if string(data) != want {
		t.Errorf("got:\n%s\nwant:\n%s", data, want)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	tests := []struct{ name, in string }{
		{"no frontmatter", "# T\n"},
		{"unterminated", "---\nid: a1\n"},
		{"bad status", "---\nid: a1\nstatus: nope\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
		{"no title", "---\nid: a1\nstatus: open\ncreated: 2026-10-05T12:00:00Z\n---\nbody\n"},
		{"bad yaml", "---\nid: [\n---\n# T\n"},
		{"unknown key", "---\nid: a1\nstatus: open\nblockd-by: [x]\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
		{"v1 key", "---\nid: a1\nstatus: open\npriority: 1\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
		{"bad id", "---\nid: ../x\nstatus: open\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Unmarshal([]byte(tt.in)); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestMarshalInvalid(t *testing.T) {
	tests := []struct {
		name string
		in   Ticket
	}{
		{"bad status", Ticket{ID: "a1", Status: "x", Title: "T"}},
		{"empty id", Ticket{Status: StatusOpen, Title: "T"}},
		{"path id", Ticket{ID: "a/b", Status: StatusOpen, Title: "T"}},
		{"reserved id", Ticket{ID: "ROOT", Status: StatusOpen, Title: "T"}},
		{"multiline title", Ticket{ID: "a1", Status: StatusOpen, Title: "a\nb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Marshal(&tt.in); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	for _, s := range []string{"open", "in_progress", "closed"} {
		if got, err := ParseStatus(s); err != nil || string(got) != s {
			t.Errorf("ParseStatus(%q) = %q, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "Open", "done"} {
		if _, err := ParseStatus(s); err == nil {
			t.Errorf("ParseStatus(%q) expected error", s)
		}
	}
}

func TestGenerateID(t *testing.T) {
	cvcvc := regexp.MustCompile(`^[` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `][1-9]$`)
	vcvcv := regexp.MustCompile(`^[` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][1-9]$`)
	seen := map[string]bool{}
	var nc, nv int
	for range 2000 {
		id := GenerateID()
		switch {
		case cvcvc.MatchString(id):
			nc++
		case vcvcv.MatchString(id):
			nv++
		default:
			t.Fatalf("bad id %q", id)
		}
		if strings.Contains(id, "0") || !validID(id) {
			t.Fatalf("bad id %q", id)
		}
		seen[id] = true
	}
	if nc == 0 || nv == 0 {
		t.Errorf("both stem shapes expected, got cvcvc=%d vcvcv=%d", nc, nv)
	}
	if len(seen) < 1900 {
		t.Errorf("too many collisions: %d unique of 2000", len(seen))
	}
}

func TestResolveID(t *testing.T) {
	ids := []string{"fanir7", "fanir71", "lovet2", "lovet3"}
	tests := []struct {
		in      string
		want    string
		wantErr string
	}{
		{"fanir7", "fanir7", ""}, // exact wins over substring of fanir71
		{"fanir71", "fanir71", ""},
		{"ovet2", "lovet2", ""},
		{"lovet", "", "ambiguous"},
		{"nope", "", "not found"},
		{"", "", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ResolveID(ids, tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	if _, err := ResolveID(ids, "zzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s := New(filepath.Join(t.TempDir(), ".tickets"))
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStoreCRUD(t *testing.T) {
	s := newStore(t)
	a := &Ticket{ID: "fanir7", Status: StatusOpen, Created: ts, Title: "A", Body: "body\n"}
	b := &Ticket{ID: "lovet2", Status: StatusClosed, Created: ts, Title: "B"}
	for _, tk := range []*Ticket{b, a} {
		if err := s.Save(tk); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load("fanir7")
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("Load = %+v, %v", got, err)
	}
	list, err := s.List()
	if err != nil || len(list) != 2 || list[0].ID != "fanir7" || list[1].ID != "lovet2" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	id, err := s.ResolveID("ovet")
	if err != nil || id != "lovet2" {
		t.Errorf("ResolveID = %q, %v", id, err)
	}
	if err := s.Delete("fanir7"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("fanir7"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after delete: %v", err)
	}
	if err := s.Delete("fanir7"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete: %v", err)
	}
	if _, err := s.Load("../x"); err == nil {
		t.Error("expected error for invalid id")
	}
}

func TestSaveAtomic(t *testing.T) {
	s := newStore(t)
	tk := &Ticket{ID: "fanir7", Status: StatusOpen, Created: ts, Title: "A"}
	if err := s.Save(tk); err != nil {
		t.Fatal(err)
	}
	tk.Title = "B"
	if err := s.Save(tk); err != nil {
		t.Fatal(err)
	}
	// No temp files remain, and an invalid save leaves the old file intact.
	bad := &Ticket{ID: "fanir7", Status: "bogus", Title: "C"}
	if err := s.Save(bad); err == nil {
		t.Fatal("expected error")
	}
	entries, _ := os.ReadDir(s.Dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
	got, err := s.Load("fanir7")
	if err != nil || got.Title != "B" {
		t.Errorf("Load = %+v, %v", got, err)
	}
	if info, _ := os.Stat(filepath.Join(s.Dir, "fanir7.md")); info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v", info.Mode())
	}
}

func TestListIgnoresNonTickets(t *testing.T) {
	s := newStore(t)
	os.WriteFile(filepath.Join(s.Dir, "notes.txt"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(s.Dir, "sub.md"), 0o755)
	if err := s.Save(&Ticket{ID: "fanir7", Status: StatusOpen, Created: ts, Title: "A"}); err != nil {
		t.Fatal(err)
	}
	ids, err := s.IDs()
	if err != nil || !reflect.DeepEqual(ids, []string{"fanir7"}) {
		t.Errorf("IDs = %v, %v", ids, err)
	}
}

func TestLoadIDMismatch(t *testing.T) {
	s := newStore(t)
	data, _ := Marshal(&Ticket{ID: "lovet2", Status: StatusOpen, Created: ts, Title: "A"})
	os.WriteFile(filepath.Join(s.Dir, "fanir7.md"), data, 0o644)
	if _, err := s.Load("fanir7"); err == nil {
		t.Error("expected id mismatch error")
	}
}

func TestRoots(t *testing.T) {
	tests := []struct {
		name  string
		roots []string
	}{
		{"empty", nil},
		{"one", []string{"fanir7"}},
		{"many ordered", []string{"lovet2", "fanir7", "kamop3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore(t)
			if err := s.SaveRoots(tt.roots); err != nil {
				t.Fatal(err)
			}
			got, err := s.LoadRoots()
			if err != nil || !reflect.DeepEqual(got, tt.roots) {
				t.Errorf("got %v, %v; want %v", got, err, tt.roots)
			}
		})
	}
}

func TestRootsEdgeCases(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if got, err := s.LoadRoots(); err != nil || got != nil {
		t.Errorf("missing ROOT.md: %v, %v", got, err)
	}
	if err := s.SaveRoots([]string{"a/b"}); err == nil {
		t.Error("expected error for invalid root id")
	}
	os.WriteFile(filepath.Join(dir, "ROOT.md"), []byte("garbage"), 0o644)
	if _, err := s.LoadRoots(); err == nil {
		t.Error("expected parse error")
	}
	if err := s.SaveRoots([]string{"fanir7", "lovet2"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "ROOT.md"))
	if want := "---\nroots: [fanir7, lovet2]\n---\n"; string(data) != want {
		t.Errorf("ROOT.md = %q, want %q", data, want)
	}
}

func TestInit(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "a", "b"))
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRoots([]string{"fanir7"}); err != nil {
		t.Fatal(err)
	}
	// Init is idempotent and does not clobber ROOT.md.
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.LoadRoots(); !reflect.DeepEqual(got, []string{"fanir7"}) {
		t.Errorf("roots = %v", got)
	}
}
