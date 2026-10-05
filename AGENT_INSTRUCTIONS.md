# Ticket Management with tk

This project uses **tk** for ticket tracking. Tickets are markdown files in `.tickets/`. They form an ordered tree; `tk ready` returns the next leaf to work on. Ticket state is the source of truth for progress.

## Workflow

```bash
tk ready              # next ticket: "<id> <position> <title>"
tk show <id>          # read it (partial ids work)
tk start <id>         # claim it (in_progress)
tk note <id> "..."    # record progress / decisions
tk close <id>         # finish, then run tk ready again
```

`tk ready <epic-id>` limits the search to one epic's subtree.

### Exit codes of `tk ready`

- `0`: a ticket was printed; work on it.
- `1`: nothing left to do.
- `2`: the top leaf is blocked. The output names the blocker (`blocked: A "..." waits on B "..." (pos, status)`). Do not skip ahead. Either work on the blocker (`tk start <blocker>`; it may be elsewhere in the tree), or, if the dependency is wrong, remove it with `tk undep <id> <blocker>`, or reorder with `tk mv`.

### Building a hierarchy

An epic is just a ticket with children. Children are picked up in order.

```bash
epic=$(tk new "Rewrite parser")               # prints the new id
tk new "Tokenizer" --under "$epic"
tk new "Parser" --under "$epic" -b - <<'EOF'
Body text with `code`, $VARS and "quotes".
EOF
tk new "Urgent fix" --under "$epic" --at 1    # insert first
tk ls                                         # outline with positions
```

### Reordering

Order decides what `tk ready` returns. Positions shown by `tk ls` are display only; always pass ids.

```bash
tk up <id>; tk down <id>; tk top <id>; tk bottom <id>
tk mv <id> --before <other>                   # or --after <other>, --at N
tk mv <id> --under <parent> [--at N]          # reparent; --root for top level
```

### Dependencies

```bash
tk dep <id> <blocker>     # <id> waits until <blocker> is closed
tk undep <id> <blocker>
```

Descendants inherit an ancestor's blockers. Cycles and ancestor/descendant deps are rejected.

### Other

- `tk close <epic> --force` closes an epic and all its descendants; plain `close` fails while any descendant is unclosed.
- Adding a child to a closed or in_progress ticket resets it to open (it is now an epic).
- `tk rm <id>` refuses if it has children or blocks others; `--force` deletes the subtree.
- `tk reopen <id>` sets a ticket back to open. `tk fsck` checks integrity.
- Free-form text (body, notes) can be given inline, via `-` (stdin) or `-F file`. Use a quoted heredoc for anything with backticks, `$` or quotes.

## Rules

- File tickets (`tk new`) for follow-up or discovered work before you finish.
- Always `tk start` before working and `tk close` when done; never leave work in an ambiguous state.
- Use `tk note` to leave context for the next session.
