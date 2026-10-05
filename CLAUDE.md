# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`tk` is a minimal ticket tracker implemented in Go, built around one idea: tickets form an ordered tree, and `tk ready` returns the highest leaf. Tickets are stored as markdown files with YAML frontmatter in the `.tickets/` directory. The full specification is in `docs/spec.md`.

## Build and Development Commands

```bash
make build        # go build -o tk
make test         # go test -race ./...
make check        # fmt, vet, test
go run main.go [command]   # run without building
```

### Common operations
```bash
./tk new "Title" [--type T] [--under P] [--at N]   # Create a ticket (prints id)
./tk ls [--all] [id]                    # Outline of the tree with positions
./tk show <partial-id>                  # Show ticket details
./tk ready [epic]                       # Highest ready leaf (exit 0/1/2, 3 on error)
./tk type <id> <task|bug|feature|chore>  # set type
./tk start <id> / close <id> [--force] / reopen <id>
./tk mv <id> --under P --at N           # also --before/--after/--root; up/down/top/bottom
./tk dep <id> <blocker> / undep ...     # blocked-by management
./tk rm <id> [--force]
./tk fsck                               # integrity check
./tk tui                                # interactive reorder (j/k, J/K, H/L, g/G, q)
```

## Architecture

### Core Concepts

**On-disk layout**: `.tickets/ROOT.md` holds the ordered root ids (`roots: [...]` in frontmatter). Each ticket is `.tickets/<id>.md` with frontmatter `id`, `status` (`open|in_progress|closed`), `type` (`task|bug|feature|chore`, always written; missing reads as `task`; pure metadata, no effect on ready/status/order/deps), `blocked-by` (list of ids), `children` (ordered list of ids) and `created`, followed by a markdown body (`# Title`, text, timestamped notes). The hierarchy is stored **only** in `children` lists and `ROOT.md`; a child does not store its parent, it is derived by scanning.

**Order**: siblings are ordered by position in the parent's `children` list (roots by `ROOT.md`). Global order is DFS preorder over the roots. A leaf is a ticket with no children; an epic is any ticket that has children (not a type).

**ID Generation**: `internal/store/id.go` generates short pronounceable ids (consonant-vowel-consonant-vowel-consonant plus a trailing digit, e.g. `fanir7`) using crypto/rand. No directory-derived prefix.

**Partial ID Matching**: users pass full or partial ids; `Tree.Resolve()` tries an exact match, then substring match, and errors on zero or multiple matches. Positions like `1.2` are display only and are never accepted as arguments.

**Ready semantics** (`internal/tree/ready.go`): if any leaf is `in_progress`, take the first in DFS order; otherwise the first non-closed leaf in DFS order (a non-leaf whose descendants are all closed counts as a leaf). `ready <epic>` scopes to a subtree. A ticket is blocked if it or any ancestor has an unclosed `blocked-by` entry; if the top leaf is blocked, `ready` stops and prints the blocker. Exit codes: `0` printed `<id> <position> [<type>] <title>`, `1` nothing left to do, `2` top leaf blocked (`blocked: ...` on stdout). Any other error exits `3` (`cmd.ExitGeneric`), so 1 never means failure for `ready`; `fsck` exits `1` when it finds problems (`cmd.ExitFsck`).

**Status rules** (`internal/tree/status.go`): leaves hold real status. `start` fails on a non-leaf with unclosed descendants. `close` on a non-leaf fails while any descendant is unclosed; `--force` closes all descendants. Adding a child under a `closed` or `in_progress` ticket resets that parent to `open`; start/reopen/add/move of unclosed work also resets closed and `in_progress` ancestors (`fsck` flags an `in_progress` or `closed` ticket with unclosed descendants). Mutations (`Tree.apply`) refuse with `ErrCorrupt` while any ticket or `ROOT.md` failed to load; ticket frontmatter is parsed strictly, so unknown keys make the ticket unreadable instead of being dropped on save. `undep` fails unless the blocker is currently listed (an exact dangling id is allowed). `rm` refuses if the ticket has children or is anyone's blocker; `--force` deletes the subtree and detaches deps.

**Dependencies** (`internal/tree/dep.go`): rejected when they form a cycle (counting `blocked-by` edges and parent links), or link a ticket with its own ancestor or descendant. `mv` re-checks these.

### Package Structure

**`main.go`**: calls `cmd.Execute()`.

**`cmd/`**: thin Cobra commands, one file per concern; each registers itself via `register()` in `init()`.
- `root.go`: root command, `--dir` flag, exit-code handling (`ExitError`)
- `helpers.go`: `App` helpers (`LoadTree`, `LoadResolved`), status markers
- `new.go`, `ls.go`, `show.go`, `edit.go`, `note.go`: create, view, edit, annotate
- `status.go`: `start`, `close`, `reopen`
- `type.go`: `type` (calls `Tree.SetType`)
- `ready.go`: `ready` with exit codes 0/1/2 (generic errors exit 3)
- `move.go`: `mv`, `up`, `down`, `top`, `bottom`
- `dep.go`: `dep`, `undep`
- `rm.go`, fsck command
- `tui.go`: `tui` (loads the tree and starts `internal/tui`)
- `textinput.go`: free-form text contract (inline, `-` for stdin, `-F` file)

**`internal/store/`**: file persistence only.
- `ticket.go`: `Ticket` struct and `Status` enum
- `parser.go`: markdown + frontmatter read/write
- `store.go`: load/save/delete tickets and `ROOT.md`, atomic writes (temp file then rename), id listing
- `id.go`: id generation

**`internal/tree/`**: domain logic over an in-memory `Tree` loaded from the store.
- `tree.go`: ordering, positions, ancestors/descendants, id resolution, `apply`/`commit`
- `ready.go`: ready selection and block description
- `status.go`: start/close/reopen rules
- `mutate.go`: add, move, up/down/top/bottom, indent/outdent, remove
- `dep.go`: dependency validation and add/remove
- `fsck.go`: integrity checks (orphans, ticket in two parents, dangling ids, cycles, dep rule violations)

**`internal/tui/`**: Bubble Tea reorder UI. Holds no ordering rules: every key calls a `Tree` method (Up/Down/Top/Bottom/Indent/Outdent/Move) and the view is rebuilt from the tree. It never edits ticket contents (no create, delete, retitle, status, type or dep changes). Tests drive `Model.Update` in-process.

### Key Design Patterns

**Locking**: `internal/store/lock.go` provides `Store.Lock(exclusive)`, an advisory `flock` on `.tickets/.lock` (unix; no-op on Windows) that waits 5s. `cmd/helpers.go` `lockCommand` wraps every command's `RunE` so the lock spans load to save: exclusive by default, shared for commands annotated `lockShared` (`ls`, `show`, `ready`, `fsck`), none for `lockNone` (`edit` and `tui` lock themselves). The TUI takes the lock per reorder and calls `Tree.Reload()` first so it never writes from a stale tree. New commands are exclusive by default.

**Atomic updates**: mutating `Tree` methods run via `apply()`, which changes the in-memory state, persists only dirty tickets and `ROOT.md` through the store (each write is temp file + rename), and reloads from disk on any failure to roll back.

**Thin commands**: `cmd/` only parses flags, resolves ids and prints; all rules live in `internal/tree`.

**Testing**: unit tests in `internal/store` and `internal/tree`; `cmd/*_test.go` run commands in-process; `integration_test.go` at the repo root builds the binary and runs an end-to-end scenario.

## Implementation Notes

- Go modules (`go 1.25.5` in `go.mod`)
- Dependencies: `spf13/cobra` (CLI), `gopkg.in/yaml.v3` (YAML)
- Tickets directory defaults to `.tickets`, override with `--dir`
- Errors use `fmt.Errorf()` with `%w`
