package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chainFixture creates an epic plus the requested number of task tickets and
// returns their full IDs.
func chainFixture(t *testing.T, ctx *testContext, tasks int) (string, []string) {
	t.Helper()

	epic, err := ctx.exec("new", "Epic", "--type", "epic")
	if err != nil {
		t.Fatalf("creating epic: %v", err)
	}
	epic = strings.TrimSpace(epic)

	ids := make([]string, 0, tasks)
	for i := 0; i < tasks; i++ {
		id, err := ctx.exec("new", "Task")
		if err != nil {
			t.Fatalf("creating task: %v", err)
		}
		ids = append(ids, strings.TrimSpace(id))
	}

	return epic, ids
}

func TestChainCommand(t *testing.T) {
	t.Run("happy path wires parents and sequential deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 3)

		output, err := ctx.exec("chain", epic, tasks[0], tasks[1], tasks[2])
		if err != nil {
			t.Fatalf("chain command error: %v", err)
		}

		if !strings.Contains(output, epic) {
			t.Errorf("summary should mention epic %s, got: %s", epic, output)
		}
		if !strings.Contains(output, "3") {
			t.Errorf("summary should mention ticket count, got: %s", output)
		}
		if !strings.Contains(output, tasks[1]+" depends on "+tasks[0]) {
			t.Errorf("summary missing edge %s -> %s, got: %s", tasks[1], tasks[0], output)
		}
		if !strings.Contains(output, tasks[2]+" depends on "+tasks[1]) {
			t.Errorf("summary missing edge %s -> %s, got: %s", tasks[2], tasks[1], output)
		}

		for _, id := range tasks {
			tk, err := ctx.store().Get(id)
			if err != nil {
				t.Fatalf("get %s: %v", id, err)
			}
			if tk.Parent != epic {
				t.Errorf("%s parent = %q, want %q", id, tk.Parent, epic)
			}
		}

		t1, _ := ctx.store().Get(tasks[0])
		if len(t1.Deps) != 0 {
			t.Errorf("t1 deps = %v, want empty", t1.Deps)
		}

		t2, _ := ctx.store().Get(tasks[1])
		if !containsString(t2.Deps, tasks[0]) {
			t.Errorf("t2 deps = %v, want to contain %s", t2.Deps, tasks[0])
		}
		if containsString(t2.Deps, tasks[2]) {
			t.Errorf("t2 deps = %v, should not contain t3 %s", t2.Deps, tasks[2])
		}

		t3, _ := ctx.store().Get(tasks[2])
		if !containsString(t3.Deps, tasks[1]) {
			t.Errorf("t3 deps = %v, want to contain %s", t3.Deps, tasks[1])
		}
	})

	t.Run("sequencing exposes one ready ticket at a time", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 3)

		if _, err := ctx.exec("chain", epic, tasks[0], tasks[1], tasks[2]); err != nil {
			t.Fatalf("chain command error: %v", err)
		}

		output, err := ctx.exec("ready", epic)
		if err != nil {
			t.Fatalf("ready command error: %v", err)
		}
		if !strings.Contains(output, tasks[0]) {
			t.Errorf("ready should include %s, got: %s", tasks[0], output)
		}
		if strings.Contains(output, tasks[1]) || strings.Contains(output, tasks[2]) {
			t.Errorf("ready should only include the first ticket, got: %s", output)
		}

		if _, err := ctx.exec("close", tasks[0]); err != nil {
			t.Fatalf("close command error: %v", err)
		}

		output, err = ctx.exec("ready", epic)
		if err != nil {
			t.Fatalf("ready command error: %v", err)
		}
		if !strings.Contains(output, tasks[1]) {
			t.Errorf("ready should include %s after closing %s, got: %s", tasks[1], tasks[0], output)
		}
		if strings.Contains(output, tasks[2]) {
			t.Errorf("ready should not include %s yet, got: %s", tasks[2], output)
		}
	})

	t.Run("re-running is idempotent", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 3)

		if _, err := ctx.exec("chain", epic, tasks[0], tasks[1], tasks[2]); err != nil {
			t.Fatalf("first chain error: %v", err)
		}

		output, err := ctx.exec("chain", epic, tasks[0], tasks[1], tasks[2])
		if err != nil {
			t.Fatalf("second chain error: %v", err)
		}
		if strings.TrimSpace(output) != "Chain already up to date" {
			t.Errorf("expected up-to-date message, got: %q", output)
		}

		t2, _ := ctx.store().Get(tasks[1])
		if len(t2.Deps) != 1 {
			t.Errorf("t2 deps = %v, want exactly 1 entry", t2.Deps)
		}

		t3, _ := ctx.store().Get(tasks[2])
		if len(t3.Deps) != 1 {
			t.Errorf("t3 deps = %v, want exactly 1 entry", t3.Deps)
		}
	})

	t.Run("partial IDs work for epic and tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 2)

		output, err := ctx.exec("chain", epic[:3], tasks[0][:3], tasks[1][:3])
		if err != nil {
			t.Fatalf("chain command error: %v", err)
		}
		if !strings.Contains(output, "Chain") && !strings.Contains(output, "depends on") {
			t.Errorf("unexpected output: %s", output)
		}

		t2, _ := ctx.store().Get(tasks[1])
		if t2.Parent != epic {
			t.Errorf("t2 parent = %q, want %q", t2.Parent, epic)
		}
		if !containsString(t2.Deps, tasks[0]) {
			t.Errorf("t2 deps = %v, want to contain %s", t2.Deps, tasks[0])
		}
	})

	t.Run("errors on non-epic parent", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		notEpic, _ := ctx.exec("new", "Plain Task")
		notEpic = strings.TrimSpace(notEpic)
		other, _ := ctx.exec("new", "Another Task")
		other = strings.TrimSpace(other)

		_, err := ctx.exec("chain", notEpic, other)
		if err == nil {
			t.Fatal("expected error for non-epic parent")
		}
		if !strings.Contains(err.Error(), "is not an epic") {
			t.Errorf("error = %v, want 'is not an epic'", err)
		}
	})

	t.Run("errors on nonexistent epic", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		other, _ := ctx.exec("new", "Task")
		other = strings.TrimSpace(other)

		_, err := ctx.exec("chain", "zzzzz", other)
		if err == nil {
			t.Fatal("expected error for nonexistent epic")
		}
	})

	t.Run("errors when a ticket arg is the epic", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 1)

		_, err := ctx.exec("chain", epic, tasks[0], epic)
		if err == nil {
			t.Fatal("expected error when chaining the epic to itself")
		}
		if !strings.Contains(err.Error(), "epic itself") {
			t.Errorf("error = %v, want mention of the epic itself", err)
		}
	})

	t.Run("requires at least two arguments", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, _ := chainFixture(t, ctx, 1)

		if _, err := ctx.exec("chain", epic); err == nil {
			t.Fatal("expected error for missing ticket arguments")
		}
	})

	t.Run("does not duplicate deps when ticket already depends on predecessor", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 2)

		if _, err := ctx.exec("dep", tasks[1], tasks[0]); err != nil {
			t.Fatalf("pre-existing dep error: %v", err)
		}

		output, err := ctx.exec("chain", epic, tasks[0], tasks[1])
		if err != nil {
			t.Fatalf("chain error: %v", err)
		}
		if strings.Contains(output, tasks[1]+" depends on "+tasks[0]) {
			t.Errorf("existing edge should not be reported as added: %s", output)
		}

		t2, _ := ctx.store().Get(tasks[1])
		if len(t2.Deps) != 1 {
			t.Errorf("t2 deps = %v, want exactly 1 entry", t2.Deps)
		}
	})

	t.Run("writes use atomic field updates preserving deps formatting", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		epic, tasks := chainFixture(t, ctx, 2)

		if _, err := ctx.exec("chain", epic, tasks[0], tasks[1]); err != nil {
			t.Fatalf("chain error: %v", err)
		}

		raw, err := os.ReadFile(filepath.Join(ctx.ticketsDir, tasks[1]+".md"))
		if err != nil {
			t.Fatalf("reading ticket file: %v", err)
		}
		if !strings.Contains(string(raw), "deps: ["+tasks[0]+"]") {
			t.Errorf("expected inline deps array in file, got:\n%s", raw)
		}
	})
}
