# Ticket Management with tk

This project uses **tk** for ticket tracking. Tickets are stored as markdown files with YAML frontmatter in the `.tickets/` directory.

## Quick Reference

```bash
tk ready              # Find available work (no blockers)
tk ready <epic-id>    # Find the next available work inside an epic
tk chain <epic-id> <id> <id> ...   # Sequence epic children so one is ready at a time
tk show <id>          # View ticket details
tk start <id>         # Claim work (set status to in_progress)
tk close <id>         # Complete work (set status to closed)
tk ls --status=open   # List all open tickets
```

## Essential Commands

### Finding Work

- `tk ready` - Show open/in-progress tickets with all dependencies resolved (sorted by priority ascending, 0=highest)
- `tk ready --sort date` - Same, sorted by creation date (newest first)
- `tk show <id>` - Detailed ticket view with metadata and relationships
- `tk start <id>` - Set status to in_progress (claim work)
- `tk ls` - List all tickets
- `tk ls --status=open` - All open tickets
- `tk ls --status=in_progress` - Your active work
- `tk ls --status=closed` - Recently closed tickets
- `tk blocked` - Show open/in-progress tickets with unresolved dependencies
- `tk dep tree <id>` - Show dependency tree (deduplicates by default)
- `tk dep tree --full <id>` - Show full tree (all occurrences, no deduplication)

### Creating & Updating

- `tk new "Ticket title"` - Create a new ticket (defaults to status: open, type: task, priority: 2)
  - `--type=bug|feature|task|epic|chore` - Ticket type
  - `-p, --priority 0-4` - Priority (0=critical, 2=medium, 4=backlog)
  - `-b, --body "..."` - Body text, stored verbatim (`-b -` reads stdin)
  - `-F, --file <path>` - Read body from a file
  - `-a, --assignee username` - Assign to someone
  - `--parent <id>` - Parent ticket ID
  - `--external-ref "..."` - External reference (e.g., gh-123)
- `tk close <id>` - Set status to closed (mark complete)
- `tk reopen <id>` - Set status to open
- `tk note <id> "..."` - Append timestamped note to ticket (`-` reads stdin, `-F <path>` reads a file)
- `tk dep <id> <dependency-id>` - Add dependency (first ticket depends on second)
- `tk chain <epic-id> <ticket-id>...` - Make each ticket a child of the epic and chain them sequentially (ticket[i] depends on ticket[i-1]) so `tk ready <epic-id>` yields exactly one runnable ticket at a time. Idempotent: re-running adds nothing.
- `tk undep <id> <dependency-id>` - Remove dependency
- `tk link <hub-id> <id> [id...]` - Create symmetric links (bidirectional). By default the first ticket is the hub, linked to each of the rest (a star); the remaining tickets are not linked to each other
- `tk link --all-pairs <id> <id> [id...]` - Link every pair of the supplied tickets (full mesh)
- `tk unlink <id> <target-id>` - Remove the symmetric link between two tickets

### Querying & Filtering
- `tk query` - Output all tickets as JSON, one per line
- `tk query '.priority == "0"'` - Query with jq-style filters
- `tk query '.status == "open"'` - Find open tickets
- `tk query '.type == "bug"'` - Find bugs
- `tk query -` / `tk query -F filter.jq` - Read the filter from stdin / a file

### Passing Free-Form Text

`tk new` (body), `tk note` (note) and `tk query` (filter) all take text **inline**, from **stdin** with `-`, or from a **file** with `-F/--file <path>`. For anything containing backticks, `$`, quotes, backslashes or newlines, use a quoted heredoc; nothing inside it needs escaping:

```bash
tk new "Fix parser" -b - <<'EOF'
Handle `code spans`, $VARS, "quotes" and \backslashes.

## Acceptance Criteria

- Parser accepts all of the above.
EOF

tk note <id> - <<'EOF'
Root cause is `parse()` at L42 -- see $HOME handling.
EOF

tk query - <<'EOF'
.status == "open" and .priority == "0"
EOF
```

One trailing newline is trimmed, so heredoc, `printf` and file input store identical text. Stdin is read only when `-` is given. Write `## Design` / `## Acceptance Criteria` sections directly in the body.

### Maintenance
- `tk prune` - Dry-run: show dangling references (refs to deleted tickets)
- `tk prune --fix` - Actually remove dangling references from deps, links, and parent fields
  - Use case: After manually deleting ticket files (e.g., `rm .tickets/x-abc1.md`)
  - Ensures store consistency by cleaning up orphaned references

## Common Workflows

### Starting work:
```bash
tk ready              # Find available work
tk ready <epic-id>    # Next available work inside an epic
tk chain <epic> <id> <id> ...   # Chain epic children into a sequential queue
tk show <id>          # Review ticket details
tk start <id>         # Claim it
```

### Completing work:
```bash
tk close <id>         # Mark complete (can provide full or partial ID)
```

### Creating dependent tickets:
```bash
# Capture the generated IDs and reuse them
feature=$(tk new "Implement feature X" --type=feature)
test=$(tk new "Write tests for X" --type=task --parent="$feature")
tk dep "$test" "$feature"  # tests depend on feature
```

### Working with blocked tickets:
```bash
tk blocked            # See what's blocking progress
tk show <id>          # View dependencies preventing work
tk close <blocker-id> # Close the blocking ticket
```

## 🚨 CRITICAL 🚨

- **File issues for remaining work** - Create tickets with `tk new` for anything that needs follow-up
  - Create tickets for tracking strategic and/or discovered work (multi-session, dependencies, discovered work)
- **Update ticket status** - Close finished work with `tk close <id>`
  - Work is NOT complete until tickets are properly closed
  - NEVER leave work in ambiguous state (e.g., started but unclear if done)
  - Ticket state is the source of truth for project progress

