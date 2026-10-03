# tk

`tk` is a minimal graph-based issue tracker for long-horizon AI agents tasks.

`tk` is similar to [beads](https://github.com/steveyegge/beads), but stores everything as simple markdown files with YAML frontmatter — no database or daemon to manage. `tk` started out as a Go port of the [ticket](https://github.com/wedow/ticket) single-file bash script, inspired by Joe Armstrong's [Minimal Viable Program](https://joearms.github.io/published/2014-06-25-minimal-viable-program.html).

`tk` has no TUI, and will never have one. It is intentionally minimal, and intended to be used in conjunction with `grep`/`rg`, `more`/`less`, and `yazi`/`ranger`. It's trivial to browse and edit issues directly in your editor (I personally use [yazi](https://github.com/mikavilpas/yazi.nvim) and [fzf-lua](https://github.com/ibhagwan/fzf-lua) inside [nvim](https://github.com/neovim/neovim)).

## Status

`tk` is a work in progress.

## Workflow

Use `tk` only for long-horizon tasks that cannot be completed in one shot (say 200K context sans-compaction).

To get started, [install](#installation) `tk` and append [AGENT_INSTRUCTIONS.md](AGENT_INSTRUCTIONS.md) to your `CLAUDE.md` or `AGENTS.md`. Customize as necessary to adapt to your workflow - there are no hard rules here.

Enter plan mode with your AI agent, make a plan, ask the agent to *save the plan in `./.plans`, break it into 5-10 issues, each with a link to the original plan and file it in `tk`*.
From this point on, you can simply clear the context and direct the agent to *fix issue x-h42g* or *run `tk ready` and fix the next available issue*. Or, run multiple agents on multiple worktrees if you're feeling lucky.


## Key Features

- **AI-friendly**: Designed to be easily traced by AI agents following dependency graphs and context
- **File-based storage**: Tickets are `.md` files in `.tickets/`, editable in any text editor
- **Git-friendly**: Store `.tickets/` in git (like `git-bug`) or `.gitignore` it and use as a local todo list
- **Dependency tracking**: Define dependencies between tickets and visualize them as trees
- **Cross-linking**: Link related tickets together (star by default, full mesh with `--all-pairs`)
- **Partial ID matching**: Refer to tickets by any substring of their ID (e.g., `h42` matches `x-h42g`)
- **jq-style queries**: Filter tickets with `jq` expressions


## Installation

```bash
go install github.com/lo5/tk@latest
```

This installs `tk` to `$GOPATH/bin` (or `$HOME/go/bin` by default). Ensure this directory is in your PATH:

```bash
export PATH=$PATH:$HOME/go/bin
```

## Quick Start

```bash
# Create a new ticket
tk new "Fix login page"

# List all tickets
tk ls

# Show a ticket (partial ID matching)
tk show h42

# Add a dependency
tk dep h42 8a2

# View dependency tree
tk dep tree h42

# Update status
tk start h42      # Mark as in_progress
tk close h42      # Mark as closed

# Append notes
tk note h42 "Made progress on authentication"

# Query tickets
tk ls --status in_progress
tk query '.status == "in_progress"'

# Find ready work (open/in-progress with all deps closed)
tk ready

# Find the next ready ticket inside an epic (children of the given ticket)
tk ready h42

# Sequence tickets inside an epic: make them children and chain them so that
# exactly one is ready at a time (ticket[i] depends on ticket[i-1])
tk chain h42 8a2 3f1 5c4

# Link related tickets (first ticket is the hub; others are linked to it)
tk link h42 8a2 3f1

# Link every pair of tickets (full mesh)
tk link --all-pairs h42 8a2 3f1

# Remove a link (symmetric)
tk unlink h42 8a2

# Clean up closed tickets
# By default links do not block deletion; non-closed deps/children still do.
tk clean              # Dry-run: show what would be deleted
tk clean --fix        # Actually delete closed tickets
tk clean --links=block  # Historical behavior: links also block deletion

# Clean up dangling references
tk prune              # Dry-run: show what would be cleaned
tk prune --fix        # Actually remove dangling references
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
Supports partial ID matching (e.g., 'tk show h42' matches 'x-h42g')

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
  ready       List ready tickets (optionally scoped to a ticket's children)
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
id: x-h42g
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

