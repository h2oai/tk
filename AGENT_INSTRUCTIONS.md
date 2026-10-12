## Tickets with tk

Tickets live in `.tickets/` as an ordered tree; `tk ready` returns the next leaf. Ids may be
partial.

```bash
tk ready [epic]                  # next: "<id> <pos> [<type>/<effort>] <title>"
tk show <id>                     # read it
tk start <id>                    # claim (in_progress)
tk note <id> "..."               # record progress / decisions
tk close <id>                    # finish, then tk ready again
tk new "Title" [--type bug|feature|chore] [--effort easy|normal|hard] [--under P] [--before|--after <o> | --at N]   # prints id
tk effort <id> easy|normal|hard  # re-estimate
tk ls [--all] [--archived] [id]  # outline with positions
tk dep <id> <blocker> / undep <id> <blocker>   # blocked-by
tk mv <id> --before|--after <o> / --under P [--at N] / --root; up/down/top/bottom
```

Epic = ticket with children, worked in order:

```bash
epic=$(tk new "Rewrite parser")
tk new "Tokenizer" --under "$epic"
tk new "Parser" --under "$epic" -b - <<'EOF'
Body with `code`, $VARS and "quotes".
EOF
```

### Gotchas

- `tk ready` exits `0` ticket printed, `1` nothing left, `2` top leaf blocked, `3` error (any other
  `tk` error too). `fsck` exits `1` on problems.
- On `2`, don't skip ahead: `tk start` the named blocker, or fix with `tk undep` / `tk mv`.
- Text (body, notes): inline, `-` for stdin, or `-F file`. Use a quoted heredoc for backticks, `$`
  or quotes.
- `close <epic>` fails while descendants are open; `--force` closes them all.
- Effort (`easy|normal|hard`, default `normal`) is a hint for choosing a model or effort level; it
  never changes what `tk ready` returns.
- Archived tickets (`[a]`, position `-`) are hidden from `tk ls` unless `--archived`, and can't
  be started or reopened; `tk unarchive <id>` first.
- Never run `tk tui` (human-only). Positions are display only; pass ids.

### Critical Rules

- `tk start` before working, `tk close` when done.
- Anything left undone (follow-ups, discovered or multi-session work) gets a `tk new` before you
  finish: `tk ls` first and place it with `--under` / `--before` / `--after` rather than at the end
  (fix with `tk mv`); add `--effort` when it is clearly easy or hard.
- `tk note` context for the next session.
- If `.tickets/` is gitignored, don't cite ticket ids in commits or PRs; they mean nothing to anyone
  else.
