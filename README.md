# tk

`tk` is a minimal graph-based issue tracker for long-horizon AI agent tasks.

`tk` is similar to [beads](https://github.com/steveyegge/beads), but stores everything as simple markdown files with YAML frontmatter — no database or daemon to manage. `tk` started out as a Go port of the [ticket](https://github.com/wedow/ticket) single-file bash script, inspired by Joe Armstrong's [Minimal Viable Program](https://joearms.github.io/published/2014-06-25-minimal-viable-program.html).

`tk` has no TUI, and will never have one. It is intentionally minimal, and intended to be used in conjunction with `grep`/`rg`, `more`/`less`, and `yazi`/`ranger`. It's trivial to browse and edit issues directly in your editor (I personally use [yazi](https://github.com/mikavilpas/yazi.nvim) and [fzf-lua](https://github.com/ibhagwan/fzf-lua) inside [nvim](https://github.com/neovim/neovim)).

## Status

`tk` is a work in progress.

## Workflow

[Install](#installation) `tk`, then append [AGENT_INSTRUCTIONS.md](AGENT_INSTRUCTIONS.md) to your `CLAUDE.md` or `AGENTS.md`. Customize as needed.

### When to use tk

Reach for `tk` when the work won't fit in a single context window.

### The loop

1. **Plan** with your agent until you have a concrete approach.

2. **File** it as an epic plus ordered child tickets, chained so exactly one is ready at a time. Reference the plan with `--external-ref`; there is no fixed plan directory.

   ```bash
   epic=$(tk new "Rewrite parser" --type=epic)                          # fanir7
   token=$(tk new "Tokenizer" --parent="$epic" --external-ref=plan.md)  # pivot3
   parser=$(tk new "Parser" --parent="$epic" --external-ref=plan.md)    # miver8
   tk chain "$epic" "$token" "$parser"
   ```

3. **Clear context** and start a fresh session, so only the tickets drive the work.

4. **Claim** the next ready ticket in the epic: `tk ready "$epic"`, then `tk show pivo` (partial IDs match: `pivo` → `pivot3`) and `tk start pivo`.

5. **Close** it with `tk close pivo`, then repeat from step 4.

### Core commands

```bash
# Create and wire tickets (`tk new` prints the new ID)
epic=$(tk new "Rewrite parser" --type=epic)     # fanir7
tk new "Tokenizer" --parent="$epic"             # pivot3
tk new "Parser" --parent="$epic"                # miver8
tk dep <id> <dep-id>          # <id> depends on <dep-id>
tk chain "$epic" <id> ...     # sequence an epic: one ready at a time
tk new "Edge cases" --parent="$epic" -b - <<'EOF'
## Acceptance Criteria
- Handles `code`, $VARS and "quotes".
EOF

# Find, claim, finish
tk ready                      # all ready tickets, priority order
tk ready "$epic"              # ready tickets inside one epic
tk ready --tree               # ready tickets as a parent tree
tk ready --watch [-n 5]       # live tree, refreshed every N seconds
tk blocked                    # open tickets waiting on dependencies
tk show fan                   # partial IDs match: fan -> fanir7
tk start <id>                 # status -> in_progress
tk note <id> "..."            # timestamped progress note
tk ls --status in_progress
tk close <id>
```

Everything else — `link`, `clean`, `prune`, `query`, `edit` — lives under [All Commands](#all-commands).

## Key Features

- **AI-friendly**: Designed to be easily traced by AI agents following dependency graphs and context
- **File-based storage**: Tickets are `.md` files in `.tickets/`, editable in any text editor
- **Git-friendly**: Store `.tickets/` in git (like `git-bug`) or `.gitignore` it and use as a local todo list
- **Dependency tracking**: Define dependencies between tickets and visualize them as trees
- **Cross-linking**: Link related tickets together (star by default, full mesh with `--all-pairs`)
- **Partial ID matching**: Refer to tickets by any substring of their ID (e.g., `fan` matches `fanir7`)
- **jq-style queries**: Filter tickets with `jq` expressions


## Installation

```bash
go install github.com/h2oai/tk@latest
```

This installs `tk` to `$GOPATH/bin` (or `$HOME/go/bin` by default). Ensure this directory is in your PATH:

```bash
export PATH=$PATH:$HOME/go/bin
```

## Cleaning Closed Tickets

`tk clean` deletes closed tickets that nothing surviving references. Dependants
(`deps`) and children (`parent`) are always hard blockers. The `--links` flag
selects whether *links* (bidirectional "related tickets") also block deletion:

| `--links` | Behavior |
| --- | --- |
| `ignore` (default) | Links are informational; a link to any ticket, including a surviving one, never blocks deletion. |
| `block` | A link to a ticket that is not itself being deleted blocks deletion (the historical behavior). |

`--links` does not affect dependant or child blocking. A link to a missing ID
(a dangling link) follows the active policy: ignored under `--links=ignore` and
blocking under `--links=block`. `clean` never rewrites dangling references; use
`tk prune` to remove them.

## All Commands

```bash

./tk help
tk - minimal ticket system with dependency tracking

Tickets are stored as markdown files with YAML frontmatter in .tickets/
Supports partial ID matching (e.g., 'tk show fan' matches 'fanir7')

Usage:
  tk [command]

Available Commands:
  blocked     List blocked tickets
  chain       Sequence tickets as children of an epic
  clean       Delete all closed tickets
  close       Set ticket status to closed
  closed      List recently closed tickets
  completion  Generate the autocompletion script for the specified shell
  dep         Add a dependency
  edit        Open ticket in $EDITOR
  help        Help about any command
  link        Link tickets together
  ls          List tickets
  new         Create a new ticket
  note        Append timestamped note to ticket
  prune       Remove dangling references from tickets
  query       Output tickets as JSON
  ready       List ready tickets
  reopen      Set ticket status to open
  rm          Delete a ticket
  show        Display a ticket
  start       Set ticket status to in_progress
  status      Update ticket status
  undep       Remove a dependency
  unlink      Remove link between tickets

Flags:
      --dir string   tickets directory (default ".tickets")
  -h, --help         help for tk

Use "tk [command] --help" for more information about a command.
```

## Ticket Format

Tickets are markdown files stored in `.tickets/{id}.md` with YAML frontmatter:

```yaml
---
id: fanir7
status: open
created: 2025-01-12T10:30:00Z
deps: []
links: []
priority: 1
type: feature
assignee: Jack B. Nimble
---

# Ticket Title

Description of the ticket goes here. You can use markdown formatting.
More details, context, and notes can be added.
```

