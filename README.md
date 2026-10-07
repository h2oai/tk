# tk

`tk` is a minimal, local ticket tracker for long-horizon AI agent tasks. Tickets form an ordered tree, and the highest leaf is the ticket ready to be picked up next. You create a hierarchy and reorder it; the agent works the tree continuously, top to bottom.

Tickets are plain markdown files with YAML frontmatter in `.tickets/`. There is no database or daemon, so you and your agent can edit or grep the files directly.

## Concepts

- **Tree**: every ticket has at most one parent. Roots are ordered, and each ticket's children are ordered. Earlier means picked up first.
- **Epic**: not a type. A ticket is an epic if and only if it has children.
- **Leaf**: a ticket with no children. Leaves hold real status (`open`, `in_progress`, `closed`).
- **Order**: the global order is DFS preorder over the roots. `tk ready` returns the first unfinished leaf in that order.
- **Blockers**: a ticket can wait on other tickets (`tk dep`). Descendants inherit blocking from ancestors.

## Installation

```bash
go install github.com/h2oai/tk@latest
```

Make sure `$GOPATH/bin` (default `$HOME/go/bin`) is on your `PATH`.

## Workflow

Append [AGENT_INSTRUCTIONS.md](AGENT_INSTRUCTIONS.md) to your `CLAUDE.md` or `AGENTS.md`.

Use this prompt to kick off your agent, substituting your epic's id for `<epic-id>`:

> Solve the child tickets of `<epic-id>` using subagents, sequentially. Have each
> subagent ask you for clarification when it needs it (questions with options and
> a recommendation), so you can relay them to me. If a subagent files new
> tickets, stop and let me know so I can triage them.

Then monitor the top-level agent and triage with `tk tui`.

## Quick example

```bash
epic=$(tk new "Rewrite parser")                     # fanir7
tk new "Tokenizer" --under "$epic"                  # pivot3
tk new "Parser" --under "$epic" -b - <<'EOF'
Handle `code`, $VARS and "quotes".
EOF

tk ls                  # outline with positions
tk ready               # -> pivot3 1.1 Tokenizer
tk start pivo          # partial ids work
tk note pivo "lexer done, starting on escapes"
tk close pivo
tk ready               # -> next leaf
```

Reorder with `tk mv`, `tk up`, `tk down`, `tk top`, `tk bottom`; the next `tk ready` follows the new order.

## Roadmap

`tk ls --all` displays an auto-numbered hierarchical list — effectively the roadmap the agent reads to know where it is, where it's going, and how to get there. Markers: `[ ]` open, `[~]` in progress, `[x]` closed; `[type]` is the ticket's type.

For example:

```text
1 [ ] inusa1  [feature] Phase 1 — Walking skeleton
  1.1 [x] wujif1  [chore] Scaffold package, build, and fixture isolation
  1.2 [x] epuwo6  [feature] Tracer bullet: lint() to disallowed-file to renderReport()
  1.3 [x] suzos5  [task] Node smoke test for the built package
  1.4 [x] bepin4  [feature] Walker and first rule 1 violations (debugger, var)
  1.5 [x] muwes8  [feature] Nested and overlapping violations (class)
  1.6 [x] jiben3  [feature] Render multi-line ranges, elision, and output cap
  1.7 [~] agovi4  [chore] Phase 1 coverage review
  1.8 [ ] fegez4  [chore] Phase 1 DRY review
2 [ ] fosin9  [feature] Phase 2 — Rule 1
  2.1 [ ] vagew8  [chore] Decide on destructured constructor and __proto__ keys
  2.2 [ ] topiv9  [bug] Treat bodiless overload signatures as type-level
  2.3 [ ] nefir9  [chore] Decide on import x = N.y entity aliases
  2.4 [ ] cujuf3  [feature] Decorators, generators/yield, and with
  2.5 [ ] ugolu5  [feature] Multi-declarator declarations and comma expressions
  2.6 [ ] necup3  [feature] enum and namespace-like declarations
  2.7 [ ] uvuso5  [feature] Object-literal accessors and this/arguments/new.target/import.meta
  2.8 [ ] dezuz5  [feature] Member access named constructor, __proto__, prototype
```

## Interactive reordering

`tk tui` opens a keyboard UI for reordering the tree: `j`/`k` move the cursor, `J`/`K` move a ticket down/up among its siblings, `H`/`L` outdent/indent, `g`/`G` move it first/last among its siblings, `Enter` opens the selected ticket in a read-only detail view (`j`/`k` scroll, `g`/`G` top/bottom, `Esc` or `q` back), and `q` quits. Every change is applied immediately through the same checks as `tk mv`, and errors appear in the status line. The TUI reorders and displays tickets; it never edits ticket contents.

## Ticket readiness

`tk ready` finds the next available ticket to work on.

1. If any leaf is `in_progress`, take the first one in DFS order (resume work).
2. Otherwise take the first leaf in DFS order that is not closed. A non-leaf whose descendants are all closed counts as a leaf (a wrap-up task) until you close it.

`tk ready <epic>` restricts the search to that ticket's subtree.

If the chosen ticket (or any ancestor) waits on an unclosed blocker, `ready` stops rather than skipping ahead, and prints the blocker:

```
blocked: fanir7 "Add parser" waits on lovet2 "Pick schema" (2.4.1, open)
```

Exit codes: `0` a ticket was printed (`<id> <position> [<type>] <title>`), `1` nothing left to do, `2` the top leaf is blocked. Fix a block by closing the blocker, `tk undep`, or reordering.

**Exit codes of every command**: `0` success, `1` `ready`: nothing left to do (also `fsck`: problems found), `2` `ready`: top leaf blocked, `3` any other error (unknown or ambiguous id, missing directory, refused operation, usage error, unreadable tickets). Only `0`, `1` and `2` from `ready` carry meaning; treat `3` as a failure, never as "done".

## Status rules

- `close` on a ticket with unclosed descendants fails; `tk close --force` closes all descendants too.
- Adding a child under a `closed` or `in_progress` ticket resets that parent to `open`. Starting, reopening, adding or moving unclosed work under an `in_progress` or `closed` ancestor resets those ancestors to `open` too; `tk fsck` reports an `in_progress` or `closed` ticket with unclosed descendants.
- A childless placeholder container shows up in `ready`; give it children or move it to the bottom.

## Dependencies

`tk dep <id> <blocker>` makes `<id>` wait on `<blocker>`; a blocker is satisfied only when closed. Dependencies may disagree with order (that is what exit code 2 reports). `tk dep` and `tk mv` reject cycles (counting parent links) and deps between a ticket and its own ancestor or descendant.

## Commands

| Command | Behaviour |
|---|---|
| `tk new <title> [--type T] [--under P] [--at N \| --before X \| --after X] [-b body \| -b - \| -F file]` | Create a ticket, print its id. `--type` is `task` (default), `bug`, `feature` or `chore`. Default: last child of `P`, or last root. |
| `tk ls [--all] [id]` | Outline with positions (e.g. `2.1.3`) and each ticket's `[type]`. Closed subtrees hidden unless `--all`. |
| `tk show <id>` | Title, body, status, type, blockers, children, position. |
| `tk type <id> <task\|bug\|feature\|chore>` | Set the ticket's type. |
| `tk edit <id>` | Open in `$EDITOR`. |
| `tk note <id> [text \| - \| -F file]` | Append a timestamped note. |
| `tk start <id>` / `tk close <id> [--force]` / `tk reopen <id>` | Status transitions. |
| `tk ready [epic]` | Highest ready leaf; exit 0/1/2 (3 on error). |
| `tk mv <id> [--under P \| --root] [--at N \| --before X \| --after X]` | Reparent and/or reposition. |
| `tk up` / `down` / `top` / `bottom <id>` | Move among siblings. |
| `tk tui` | Reorder tickets interactively: `j`/`k` move the cursor, `J`/`K` siblings, `H`/`L` outdent/indent, `g`/`G` first/last, `Enter` view ticket (`Esc` back), `q` quit. Never edits contents. |
| `tk dep <id> <blocker>` / `tk undep <id> <blocker>` | Manage blockers. `undep` fails (exit 3) if the blocker is not currently listed. |
| `tk rm <id> [--force]` | Refuses if the ticket has children or blocks others; `--force` deletes the subtree and detaches deps. |
| `tk archive <id> [--force]` | Move a fully closed subtree into `.tickets/archive/` (`--force` closes it first). Refused while a `blocked-by` edge crosses the subtree boundary. One-way: no command reads the archive back. |
| `tk archive --all` | Archive every fully closed subtree, skipping (and reporting on stderr) those still linked to live tickets by `blocked-by`. `-n`/`--dry-run` on either form prints what would move. |
| `tk fsck` | Verify integrity (orphans, duplicate parents, dangling ids, cycles, dep rule violations); exit 1 on problems, including unreadable tickets (unknown frontmatter keys, corrupt `ROOT.md`). Every command that modifies tickets refuses to run while any ticket or `ROOT.md` is unreadable. |

Ids are matched by substring (exact match first; error on zero or multiple matches). Positions such as `1.2` are display only and never accepted as arguments, because they shift on reorder. Free-form text (body, notes) can be given inline, on stdin with `-`, or from a file with `-F`. Use `--dir` to change the tickets directory (default `.tickets`).

## Ticket format

```
.tickets/
  ROOT.md        # ordered root ids
  <id>.md        # one file per ticket
```

```markdown
---
id: fanir7
status: open
type: task
blocked-by: [lovet2]
children: [kamop3, ritus9]
created: 2026-10-05T12:00:00Z
---
# Title

Body text; notes are appended as timestamped sections.
```

`ROOT.md` holds `roots: [id, ...]`. The hierarchy lives only in `children` lists and `ROOT.md`; a child does not store its parent. Ids are short pronounceable strings (e.g. `fanir7`).

## Development

```bash
make build   # go build
make test    # go test -race ./...
make check   # fmt, vet, test
```

See [CLAUDE.md](CLAUDE.md) for the architecture and [docs/spec.md](docs/spec.md) for the full specification.
