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
		{"minimal", Ticket{ID: "fanir7", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "T"}},
		{"full", Ticket{ID: "fanir7", Status: StatusInProgress, Type: TypeTask, BlockedBy: []string{"lovet2"},
			Children: []string{"kamop3", "ritus9"}, Created: ts, Title: "Add parser", Body: "Some text\n\n## Note\nmore\n"}},
		{"body no trailing newline", Ticket{ID: "a1", Status: StatusClosed, Type: TypeTask, Created: ts, Title: "T", Body: "x"}},
		{"body leading blank lines", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "T", Body: "\n\nx\n"}},
		{"body only newline", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "T", Body: "\n"}},
		{"body with fences", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "T", Body: "---\nid: x\n---\n# H\n"}},
		{"title with colon and hash", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "fix: # thing"}},
		{"unicode", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "héllo ✓", Body: "日本語\n"}},
		{"bug", Ticket{ID: "a1", Status: StatusOpen, Type: TypeBug, Created: ts, Title: "T"}},
		{"feature", Ticket{ID: "a1", Status: StatusOpen, Type: TypeFeature, Created: ts, Title: "T"}},
		{"chore", Ticket{ID: "a1", Status: StatusOpen, Type: TypeChore, Created: ts, Title: "T"}},
		{"non-utc created", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Created: ts.In(time.FixedZone("x", 3600)), Title: "T"}},
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
			want := tt.in
			if want.Type == "" {
				want.Type = TypeTask
			}
			if !reflect.DeepEqual(got, &want) {
				t.Errorf("got %+v, want %+v", got, &want)
			}
		})
	}
}

func TestMarshalFormat(t *testing.T) {
	data, err := Marshal(&Ticket{ID: "fanir7", Status: StatusOpen, Type: TypeTask, BlockedBy: []string{"lovet2"},
		Children: []string{"kamop3", "ritus9"}, Created: ts, Title: "Title", Body: "Body\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nid: fanir7\nstatus: open\ntype: task\nblocked-by: [lovet2]\nchildren: [kamop3, ritus9]\ncreated: 2026-10-05T12:00:00Z\n---\n# Title\n\nBody\n"
	if string(data) != want {
		t.Errorf("got:\n%s\nwant:\n%s", data, want)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	tests := []struct{ name, in string }{
		{"no frontmatter", "# T\n"},
		{"unterminated", "---\nid: a1\n"},
		{"bad status", "---\nid: a1\nstatus: nope\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
		{"missing created", "---\nid: a1\nstatus: open\n---\n# T\n"},
		{"zero created", "---\nid: a1\nstatus: open\ncreated: 0001-01-01T00:00:00Z\n---\n# T\n"},
		{"no title", "---\nid: a1\nstatus: open\ncreated: 2026-10-05T12:00:00Z\n---\nbody\n"},
		{"bad yaml", "---\nid: [\n---\n# T\n"},
		{"unknown key", "---\nid: a1\nstatus: open\nblockd-by: [x]\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
		{"v1 key", "---\nid: a1\nstatus: open\npriority: 1\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
		{"bad type", "---\nid: a1\nstatus: open\ntype: epic\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"},
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

func TestUnmarshalCRLF(t *testing.T) {
	in := "---\r\nid: a1\r\nstatus: open\r\nchildren: [b2]\r\ncreated: 2026-10-05T12:00:00Z\r\n---\r\n# Title\r\n\r\nline1\r\nline2\r\n"
	got, err := Unmarshal([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Title" || got.Body != "line1\nline2\n" || !reflect.DeepEqual(got.Children, []string{"b2"}) {
		t.Errorf("got %+v", got)
	}
}

func TestMissingCreatedDefaults(t *testing.T) {
	fb := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, in := range []string{
		"---\nid: a1\nstatus: open\n---\n# T\n",
		"---\nid: a1\nstatus: open\ncreated: 0001-01-01T00:00:00Z\n---\n# T\n",
	} {
		got, err := UnmarshalAt([]byte(in), fb)
		if err != nil || !got.Created.Equal(fb) {
			t.Errorf("got %v, %v; want created %v", got, err, fb)
		}
	}

	// Load falls back to the file's modification time.
	s := newStore(t)
	path := filepath.Join(s.Dir, "a1.md")
	if err := os.WriteFile(path, []byte("---\nid: a1\nstatus: open\n---\n# T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fb, fb); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("a1")
	if err != nil || !got.Created.Equal(fb) {
		t.Errorf("got %v, %v; want created %v", got, err, fb)
	}
}

func TestMarshalInvalid(t *testing.T) {
	tests := []struct {
		name string
		in   Ticket
	}{
		{"bad status", Ticket{ID: "a1", Status: "x", Type: TypeTask, Title: "T"}},
		{"empty id", Ticket{Status: StatusOpen, Type: TypeTask, Title: "T"}},
		{"path id", Ticket{ID: "a/b", Status: StatusOpen, Type: TypeTask, Title: "T"}},
		{"reserved id", Ticket{ID: "ROOT", Status: StatusOpen, Type: TypeTask, Title: "T"}},
		{"multiline title", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Title: "a\nb"}},
		{"zero created", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Title: "T"}},
		{"empty title", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask}},
		{"empty type", Ticket{ID: "a1", Status: StatusOpen, Created: ts, Title: "T"}},
		{"invalid type", Ticket{ID: "a1", Status: StatusOpen, Type: "epic", Created: ts, Title: "T"}},
		{"blank title", Ticket{ID: "a1", Status: StatusOpen, Type: TypeTask, Title: "  "}},
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
	a := &Ticket{ID: "fanir7", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "A", Body: "body\n"}
	b := &Ticket{ID: "lovet2", Status: StatusClosed, Type: TypeTask, Created: ts, Title: "B"}
	for _, tk := range []*Ticket{b, a} {
		if err := s.Save(tk); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load("fanir7")
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("Load = %+v, %v", got, err)
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
	tk := &Ticket{ID: "fanir7", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "A"}
	if err := s.Save(tk); err != nil {
		t.Fatal(err)
	}
	tk.Title = "B"
	if err := s.Save(tk); err != nil {
		t.Fatal(err)
	}
	// No temp files remain, and an invalid save leaves the old file intact.
	bad := &Ticket{ID: "fanir7", Status: "bogus", Type: TypeTask, Title: "C"}
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
	if err := s.Save(&Ticket{ID: "fanir7", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "A"}); err != nil {
		t.Fatal(err)
	}
	ids, err := s.IDs()
	if err != nil || !reflect.DeepEqual(ids, []string{"fanir7"}) {
		t.Errorf("IDs = %v, %v", ids, err)
	}
}

func TestLoadIDMismatch(t *testing.T) {
	s := newStore(t)
	data, _ := Marshal(&Ticket{ID: "lovet2", Status: StatusOpen, Type: TypeTask, Created: ts, Title: "A"})
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

func TestRootsCRLF(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ROOT.md"), []byte("---\r\nroots: [a1, b2]\r\n---\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := New(dir).LoadRoots()
	if err != nil || !reflect.DeepEqual(got, []string{"a1", "b2"}) {
		t.Errorf("got %v, %v", got, err)
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

func TestMissingTypeIsTask(t *testing.T) {
	tk, err := Unmarshal([]byte("---\nid: a1\nstatus: open\ncreated: 2026-10-05T12:00:00Z\n---\n# T\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tk.Type != TypeTask {
		t.Errorf("type = %q, want task", tk.Type)
	}
	if _, err := Marshal(&Ticket{ID: "a1", Status: StatusOpen, Type: "epic", Created: ts, Title: "T"}); err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestParseType(t *testing.T) {
	for in, want := range map[string]Type{"task": TypeTask, "BUG": TypeBug, " Feature ": TypeFeature, "chore": TypeChore} {
		if got, err := ParseType(in); err != nil || got != want {
			t.Errorf("ParseType(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "epic", "x"} {
		if _, err := ParseType(in); err == nil || !strings.Contains(err.Error(), "task, bug, feature, chore") {
			t.Errorf("ParseType(%q) err = %v", in, err)
		}
	}
}

func TestValidateRejectsEmptyType(t *testing.T) {
	tk := Ticket{ID: "a1", Status: StatusOpen, Created: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), Title: "T"}
	if err := tk.Validate(); err == nil {
		t.Fatal("Validate accepted empty type")
	}
	if _, err := Marshal(&tk); err == nil {
		t.Fatal("Marshal accepted empty type")
	}
	tk.Type = TypeTask
	if err := tk.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestArchive(t *testing.T) {
	s := newStore(t)
	if _, err := os.Stat(s.ArchiveDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive dir exists before first archive: %v", err)
	}
	src := filepath.Join(s.Dir, "fanir7.md")
	// Taken names in the archive get the shortest free numeric suffix.
	want := []string{"fanir7.md", "fanir7-2.md", "fanir7-3.md"}
	for i, name := range want {
		data := []byte("---\nid: fanir7\nstatus: closed\ntitle-ish: not parsed\n---\n# v" + string(rune('1'+i)) + "\n")
		if err := os.WriteFile(src, data, 0o644); err != nil {
			t.Fatal(err)
		}
		dst, err := s.Archive("fanir7")
		if err != nil || dst != filepath.Join(s.ArchiveDir(), name) {
			t.Fatalf("Archive = %q, %v; want %s", dst, err, name)
		}
		if got, _ := os.ReadFile(dst); !reflect.DeepEqual(got, data) {
			t.Errorf("%s not moved byte for byte: %q", name, got)
		}
		if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("source still present after archiving to %s", name)
		}
	}
	if ids, _ := s.IDs(); len(ids) != 0 {
		t.Errorf("archived files listed as live: %v", ids)
	}
	// A suffixed name is free again only when its exact file is gone.
	os.Remove(filepath.Join(s.ArchiveDir(), "fanir7-2.md"))
	os.WriteFile(src, []byte("x"), 0o644)
	if dst, err := s.Archive("fanir7"); err != nil || filepath.Base(dst) != "fanir7-2.md" {
		t.Errorf("Archive = %q, %v", dst, err)
	}
	if _, err := s.Archive("fanir7"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Archive missing: %v", err)
	}
	if _, err := s.Archive("../x"); err == nil {
		t.Error("expected error for invalid id")
	}
}
