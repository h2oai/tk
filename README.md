# tk

`tk` is a minimal ticket tracker for long-horizon AI agent tasks, built around one idea: **tickets form an ordered tree, and `tk ready` returns the highest leaf.** You create a hierarchy and reorder it; `tk` tells you what to do next.

Tickets are plain markdown files with YAML frontmatter in `.tickets/`. There is no database, daemon or TUI. Browse them with `grep`/`rg`, `less`, or your editor. `tk` started as a Go port of the [ticket](https://github.com/wedow/ticket) bash script.

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

## `tk ready`

1. If any leaf is `in_progress`, take the first one in DFS order (resume work).
2. Otherwise take the first leaf in DFS order that is not closed. A non-leaf whose descendants are all closed counts as a leaf (a wrap-up task) until you close it.

`tk ready <epic>` restricts the search to that ticket's subtree.

If the chosen ticket (or any ancestor) waits on an unclosed blocker, `ready` stops rather than skipping ahead, and prints the blocker:

```
blocked: fanir7 "Add parser" waits on lovet2 "Pick schema" (2.4.1, open)
```

Exit codes: `0` a ticket was printed (`<id> <position> <title>`), `1` nothing left to do, `2` the top leaf is blocked. Fix a block by closing the blocker, `tk undep`, or reordering.

## Status rules

- `close` on a ticket with unclosed descendants fails; `tk close --force` closes all descendants too.
- Adding a child under a `closed` or `in_progress` ticket resets that parent to `open`.
- A childless placeholder container shows up in `ready`; give it children or move it to the bottom.

## Dependencies

`tk dep <id> <blocker>` makes `<id>` wait on `<blocker>`; a blocker is satisfied only when closed. Dependencies may disagree with order (that is what exit code 2 reports). `tk dep` and `tk mv` reject cycles (counting parent links) and deps between a ticket and its own ancestor or descendant.

## Commands

| Command | Behaviour |
|---|---|
| `tk new <title> [--under P] [--at N \| --before X \| --after X] [-b body \| -b - \| -F file]` | Create a ticket, print its id. Default: last child of `P`, or last root. |
| `tk ls [--all] [id]` | Outline with positions (e.g. `2.1.3`). Closed subtrees hidden unless `--all`. |
| `tk show <id>` | Title, body, status, blockers, children, position. |
| `tk edit <id>` | Open in `$EDITOR`. |
| `tk note <id> [text \| - \| -F file]` | Append a timestamped note. |
| `tk start <id>` / `tk close <id> [--force]` / `tk reopen <id>` | Status transitions. |
| `tk ready [epic]` | Highest ready leaf; exit 0/1/2. |
| `tk mv <id> [--under P \| --root] [--at N \| --before X \| --after X]` | Reparent and/or reposition. |
| `tk up` / `down` / `top` / `bottom <id>` | Move among siblings. |
| `tk dep <id> <blocker>` / `tk undep <id> <blocker>` | Manage blockers. |
| `tk rm <id> [--force]` | Refuses if the ticket has children or blocks others; `--force` deletes the subtree and detaches deps. |
| `tk fsck` | Verify integrity (orphans, duplicate parents, dangling ids, cycles, dep rule violations); exit 1 on problems. |

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
```

See [CLAUDE.md](CLAUDE.md) for the architecture and [docs/spec.md](docs/spec.md) for the full specification.
