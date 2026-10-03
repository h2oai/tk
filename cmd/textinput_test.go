package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shellHostile contains every character class the stdin/file paths must
// preserve verbatim: backticks, $VARS, quotes, backslashes and newlines.
const shellHostile = "Handle `code spans`, $VARS, \"double\", 'single', \\backslashes,\n\nand multiple lines."

// execWithStdin runs a command with os.Stdin replaced by the given input.
func (ctx *testContext) execWithStdin(input string, args ...string) (string, error) {
	ctx.t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		ctx.t.Fatalf("pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		w.Write([]byte(input))
		w.Close()
	}()
	return ctx.exec(args...)
}

// writeTempFile writes content to a file in a fresh temp dir and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// readTicketFile returns the raw markdown of a ticket.
func (ctx *testContext) readTicketFile(id string) string {
	ctx.t.Helper()
	data, err := os.ReadFile(filepath.Join(ctx.ticketsDir, id+".md"))
	if err != nil {
		ctx.t.Fatalf("reading ticket %s: %v", id, err)
	}
	return string(data)
}

// stripCreated removes id and created lines so tickets can be compared byte-wise.
func stripCreated(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "id: ") || strings.HasPrefix(line, "created: ") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func TestResolveText(t *testing.T) {
	file := writeTempFile(t, "body.md", "from file\n")

	tests := []struct {
		name    string
		inline  string
		file    string
		stdin   string
		want    string
		wantErr string
	}{
		{name: "inline", inline: "inline text", want: "inline text"},
		{name: "empty", want: ""},
		{name: "stdin via dash", inline: "-", stdin: "from stdin\n", want: "from stdin"},
		{name: "file", file: file, want: "from file"},
		{name: "file dash is stdin", file: "-", stdin: "from stdin", want: "from stdin"},
		{name: "trims exactly one newline", inline: "-", stdin: "a\n\nb\n\n", want: "a\n\nb\n"},
		{name: "preserves shell-hostile text", inline: "-", stdin: shellHostile + "\n", want: shellHostile},
		{name: "mutually exclusive", inline: "x", file: file, wantErr: "cannot use both --body and --file"},
		{name: "missing file", file: filepath.Join(t.TempDir(), "nope"), wantErr: "reading file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCmd
			cmd.SetIn(strings.NewReader(tt.stdin))
			defer cmd.SetIn(nil)

			got, err := resolveText(cmd, tt.inline, tt.file, "--body")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewBodyInput(t *testing.T) {
	t.Run("body from stdin is verbatim", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		out, err := ctx.execWithStdin(shellHostile+"\n", "new", "T", "-b", "-")
		if err != nil {
			t.Fatalf("new error: %v", err)
		}
		tk, err := ctx.store().Get(strings.TrimSpace(out))
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if tk.Body != shellHostile {
			t.Errorf("Body = %q, want %q", tk.Body, shellHostile)
		}
	})

	t.Run("file and stdin produce byte-identical tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		path := writeTempFile(t, "body.md", shellHostile+"\n")
		idFile, err := ctx.exec("new", "T", "-F", path)
		if err != nil {
			t.Fatalf("new -F error: %v", err)
		}
		cleanup()
		idStdin, err := ctx.execWithStdin(shellHostile+"\n", "new", "T", "-b", "-")
		if err != nil {
			t.Fatalf("new -b - error: %v", err)
		}

		a := stripCreated(ctx.readTicketFile(strings.TrimSpace(idFile)))
		b := stripCreated(ctx.readTicketFile(strings.TrimSpace(idStdin)))
		if a != b {
			t.Errorf("tickets differ:\n--- file ---\n%s\n--- stdin ---\n%s", a, b)
		}
	})

	t.Run("inline body", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		out, err := ctx.exec("new", "T", "-b", "inline body")
		if err != nil {
			t.Fatalf("new error: %v", err)
		}
		tk, _ := ctx.store().Get(strings.TrimSpace(out))
		if tk.Body != "inline body" {
			t.Errorf("Body = %q, want %q", tk.Body, "inline body")
		}
	})

	t.Run("body and file are mutually exclusive", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		path := writeTempFile(t, "body.md", "x")
		_, err := ctx.exec("new", "T", "-b", "x", "-F", path)
		if err == nil || !strings.Contains(err.Error(), "cannot use both --body and --file") {
			t.Errorf("err = %v, want mutual-exclusion error", err)
		}
	})

	t.Run("removed flags are rejected", func(t *testing.T) {
		for _, flag := range []string{"--description", "--design", "--acceptance", "-d"} {
			ctx, cleanup := setupTestCmd(t)
			_, err := ctx.exec("new", "T", flag, "x")
			cleanup()
			if err == nil {
				t.Errorf("%s: expected unknown flag error", flag)
			}
		}
	})
}

func TestNoteTextInput(t *testing.T) {
	newTicket := func(ctx *testContext) string {
		out, err := ctx.exec("new", "Note Input")
		if err != nil {
			ctx.t.Fatalf("new error: %v", err)
		}
		return strings.TrimSpace(out)
	}

	t.Run("note from stdin is verbatim", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()
		id := newTicket(ctx)

		if _, err := ctx.execWithStdin(shellHostile+"\n", "note", id, "-"); err != nil {
			t.Fatalf("note error: %v", err)
		}
		if !strings.Contains(ctx.readTicketFile(id), "\n\n"+shellHostile+"\n") {
			t.Errorf("note not stored verbatim:\n%s", ctx.readTicketFile(id))
		}
	})

	t.Run("note from file", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()
		id := newTicket(ctx)

		path := writeTempFile(t, "note.md", shellHostile+"\n")
		if _, err := ctx.exec("note", id, "-F", path); err != nil {
			t.Fatalf("note error: %v", err)
		}
		if !strings.Contains(ctx.readTicketFile(id), "\n\n"+shellHostile+"\n") {
			t.Errorf("note not stored verbatim:\n%s", ctx.readTicketFile(id))
		}
	})

	t.Run("stdin is not read implicitly", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()
		id := newTicket(ctx)

		_, err := ctx.execWithStdin("should be ignored", "note", id)
		if err == nil || !strings.Contains(err.Error(), "no note provided") {
			t.Errorf("err = %v, want 'no note provided'", err)
		}
		if strings.Contains(ctx.readTicketFile(id), "should be ignored") {
			t.Error("stdin was read without -")
		}
	})

	t.Run("text and file are mutually exclusive", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()
		id := newTicket(ctx)

		path := writeTempFile(t, "note.md", "x")
		_, err := ctx.exec("note", id, "text", "-F", path)
		if err == nil || !strings.Contains(err.Error(), "cannot use both") {
			t.Errorf("err = %v, want mutual-exclusion error", err)
		}
	})
}

func TestQueryFilterInput(t *testing.T) {
	setup := func(t *testing.T) (*testContext, func(), string) {
		ctx, cleanup := setupTestCmd(t)
		out, err := ctx.exec("new", "High", "--priority", "0")
		if err != nil {
			t.Fatalf("new error: %v", err)
		}
		if _, err := ctx.exec("new", "Low", "--priority", "3"); err != nil {
			t.Fatalf("new error: %v", err)
		}
		return ctx, cleanup, strings.TrimSpace(out)
	}

	const filter = `.status == "open" and .priority == "0"`

	check := func(t *testing.T, out, id string) {
		t.Helper()
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 1 || !strings.Contains(lines[0], id) {
			t.Errorf("expected only %s, got:\n%s", id, out)
		}
	}

	t.Run("filter from stdin", func(t *testing.T) {
		ctx, cleanup, id := setup(t)
		defer cleanup()

		out, err := ctx.execWithStdin(filter+"\n", "query", "-")
		if err != nil {
			t.Fatalf("query error: %v", err)
		}
		check(t, out, id)
	})

	t.Run("filter from file", func(t *testing.T) {
		ctx, cleanup, id := setup(t)
		defer cleanup()

		path := writeTempFile(t, "filter.jq", filter+"\n")
		out, err := ctx.exec("query", "-F", path)
		if err != nil {
			t.Fatalf("query error: %v", err)
		}
		check(t, out, id)
	})

	t.Run("inline filter", func(t *testing.T) {
		ctx, cleanup, id := setup(t)
		defer cleanup()

		out, err := ctx.exec("query", filter)
		if err != nil {
			t.Fatalf("query error: %v", err)
		}
		check(t, out, id)
	})

	t.Run("no filter dumps all without reading stdin", func(t *testing.T) {
		ctx, cleanup, _ := setup(t)
		defer cleanup()

		out, err := ctx.execWithStdin(".nope", "query")
		if err != nil {
			t.Fatalf("query error: %v", err)
		}
		if n := len(strings.Split(strings.TrimSpace(out), "\n")); n != 2 {
			t.Errorf("expected 2 tickets, got %d:\n%s", n, out)
		}
	})

	t.Run("filter and file are mutually exclusive", func(t *testing.T) {
		ctx, cleanup, _ := setup(t)
		defer cleanup()

		path := writeTempFile(t, "filter.jq", "x")
		_, err := ctx.exec("query", "x", "-F", path)
		if err == nil || !strings.Contains(err.Error(), "cannot use both") {
			t.Errorf("err = %v, want mutual-exclusion error", err)
		}
	})
}
