# tk v2 specification

A minimal ticket tracker built around one idea: **tickets form an ordered tree, and `tk ready` returns the highest leaf.** All the user does is create a hierarchy and reorder it.

## Concepts

- **Ticket**: a unit of work with a title, body, status, type and optional blockers. The type (`task|bug|feature|chore`, default `task`) is pure metadata: it never affects `ready`, status rules, ordering or deps. `epic` is not a type.
- **Tree**: every ticket has at most one parent. Tickets with no parent are *roots*. Roots are ordered, and each ticket's children are ordered.
- **Epic**: not a type. A ticket is an epic if and only if it has children. Any ticket can have children, at any depth.
- **Order**: siblings are ordered by their position in the parent's `children` list (roots by `ROOT.md`). Earlier means picked up first. The global order is DFS preorder over roots.
- **Leaf**: a ticket with no children.

There is no priority, assignee, links or external-ref. Order and hierarchy replace them.

## On-disk layout

```
.tickets/
  ROOT.md        # ordered root ids
  <id>.md        # one file per ticket
  archive/       # archived tickets, flat; no ROOT.md
```

`<id>.md`:

```markdown
---
id: fanir7
status: open            # open | in_progress | closed
type: task              # task | bug | feature | chore (pure metadata; missing reads as task)
blocked-by: [lovet2]
children: [kamop3, ritus9]   # ordered; first is picked up first
created: 2026-10-05T12:00:00Z
---
# Title

Body text, notes appended as timestamped sections.
```

- The hierarchy is stored **only** in `children` lists and `ROOT.md`. A child does not store its parent; the parent is derived by scanning.
- Archived tickets live in `.tickets/archive/` (see [Archival](#archival)): flat `<id>.md` files, no `ROOT.md`, created lazily by `tk archive`.
- IDs are pronounceable five-letter stems followed by a digit 1–9 (e.g. `fanir7`, `igoro8`). The stem is either CVCVC (consonant-vowel-consonant-vowel-consonant) or VCVCV (vowel-consonant-vowel-consonant-vowel), chosen randomly; Q and X are excluded from consonants, and 0 is excluded from digits to avoid confusion with letters. Partial matching: exact match first, then substring; error on zero or multiple matches.
- Concurrency: every command takes an advisory `flock` on `.tickets/.lock` (exclusive for commands that write, shared for `ls`, `show`, `ready`, `fsck`) for its whole load-modify-save cycle, waiting up to 5 seconds. The lock is released when the process exits, so crashes leave nothing stale. `edit` locks only while reading and saving (and refuses to save if the ticket changed meanwhile); `tui` locks and reloads from disk on each reorder. The `.lock` file can be gitignored. This serializes writers but does not claim tickets: two workers asking `tk ready` still get the same ticket.
- Writes are atomic (temp file then rename). `mv` edits up to three files (old parent, new parent, and the moved ticket is untouched).
- `tk` directory defaults to `.tickets`, override with `--dir`.

## Status rules

- Statuses: `open`, `in_progress`, `closed`.
- **Leaves** hold real status.
- **Non-leaves** may be closed manually, but `close` fails while any descendant is not closed (`--force` cascades closing to all descendants).
- Adding a child under a ticket that is `closed` or `in_progress` resets that parent to `open` (it is now an epic and waits for its children).
- A parent whose children are all closed becomes a ready leaf again (a wrap-up task) until closed by hand. Childless placeholder containers surface in `ready` too; this is accepted. Reorder to the bottom or add children first.

## `tk ready`

Definition of "highest leaf":

1. If any leaf is `in_progress`, take the first one in DFS order (resume work).
2. Otherwise take the first leaf in DFS order whose status is not `closed`. A non-leaf whose descendants are all closed counts as a leaf for this purpose, since it is the next actionable item.

`tk ready <epic>` scopes the search to that ticket's subtree.

Blocking: a ticket is blocked if it, or any ancestor, has an unclosed ticket in `blocked-by`. If the selected top leaf is blocked, `ready` **stops** (it does not skip to the next leaf) and prints the blocker and its position, for example:

```
blocked: fanir7 "Add parser" waits on lovet2 "Pick schema" (2.4.1, open)
```

Output on success is `<id> <position> [<type>] <title>`, e.g. `fanir7 1.2 [bug] Fix crash`. Exit codes: `0` a ready ticket was printed, `1` nothing left to do, `2` top leaf is blocked.

## Dependencies (`blocked-by`)

- `blocked-by` is a list of ticket ids, on any ticket. A blocker is satisfied only when it is `closed`. Epics may be blockers (satisfied once closed manually).
- Dependencies **may disagree with order**. That is the situation `ready` exit code 2 reports. `tk dep` and the move commands (`mv`, `up`, `down`, `top`, `bottom`) warn on stderr (`warning: <id> "<title>" (<pos>) is above its blocker ...`) when they newly create such a pair involving the changed ticket or its descendants; the change still succeeds. `tk fsck` lists every current pair as a `warn:` line without affecting its exit code. Closed tickets and closed blockers never warn.
- Rejected by `tk dep` and re-checked by `tk mv`:
  - cycles, counting both `blocked-by` edges and parent links
  - a dep between a ticket and its own ancestor or descendant (it could never be satisfied)
- Descendants inherit blocking from ancestors.

## Commands

| Command | Behaviour |
|---|---|
| `tk new "Title" [--type T] [--under P] [--at N] [-b body \| -b - \| -F file]` | Create a ticket; `--type` is `task` (default), `bug`, `feature` or `chore`. Default: append as last child of `P`, or last root. |
| `tk ls [--all] [<id>]` | Render the tree as an outline with positions (e.g. `2.1.3`). Each line shows `[type]` before the title. Closed subtrees hidden unless `--all`. Positions are display only. Walks from `ROOT.md`, so unreachable tickets (orphans) are not shown; when any exist, a one-line warning goes to stderr pointing at `tk fsck`. |
| `tk show <id>` | Title, body, status, type, blockers, children, position. |
| `tk type <id> <type>` | Set the type (case-insensitive, stored lowercase). Works on any status; setting the current type is a silent no-op; no status or ancestor effects. |
| `tk edit <id>` | Open in `$EDITOR`. |
| `tk note <id> [text \| - \| -F file]` | Append a timestamped note. |
| `tk start <id>` / `tk close <id>` / `tk reopen <id>` | Status transitions, with the rules above. |
| `tk ready [<epic>]` | See above. |
| `tk mv <id> --under P --at N` (also `--before/--after <id>`) | Reparent and/or reposition. |
| `tk up/down/top/bottom <id>` | Move within siblings. |
| `tk tui` | Interactive reorder: `j/k` move the cursor, `J/K` down/up among siblings, `H/L` outdent/indent, `g/G` top/bottom among siblings, `enter` open the selected ticket in a read-only, scrollable detail view rendered with glamour (`j/k`, `pgup/pgdn`, `g/G` scroll; `J/K` or `→/←` next/previous ticket in list order; `esc` or `q` back), `q` quit. Changes apply immediately through the same checks as `tk mv`; errors show in the status line. It never edits ticket contents. |
| `tk dep <id> <blocker>` / `tk undep <id> <blocker>` | Manage `blocked-by`. |
| `tk rm <id>` | Refuses if the ticket has children or is anyone's blocker. `--force` deletes the subtree and detaches deps. |
| `tk archive <id> [--force] [-n]` | Move the closed subtree into `.tickets/archive/` (see [Archival](#archival)). |
| `tk archive --all [-n]` | Move every fully closed subtree into `.tickets/archive/`, skipping those linked to live tickets (see [Archival](#archival)). |
| `tk fsck` | Verify integrity: orphans, ticket in two parents, dangling ids, cycles, dep rule violations (exit 1), plus `warn:` lines for tickets above their unclosed blockers (exit unaffected). Orphan lines hint at the repair: `tk mv <id> --root` (or `--under`) re-attaches the ticket with its subtree. |

Addressing is by id only (partial matching). Positional paths are never accepted as arguments, because they shift on reorder.

Free-form text input contract: inline, stdin with `-`, or file with `-F`.

## Non-goals

- Claims or assignees for multiple workers: single worker assumed (concurrent writes are serialized by the lock, but `ready` does not reserve a ticket). Use `tk ready <epic>` to scope parallel streams by hand.
- Priority, assignee, links, jq `query`, live `--watch` tree, `prune`, `clean`.
- Positional addressing and a single outline file.
- Archive retrieval: there is no `unarchive`, no way to list the archive, and no
  command that reads an archived ticket. Inspect or delete `.tickets/archive/` by
  hand.

## Archival

Archiving is a **one-way move**. It relocates a finished subtree out of the live
tree into a sibling directory, and nothing reads that directory back: `tk archive`
is the only command that touches it. To inspect or delete archived tickets, work
with `.tickets/archive/` by hand.

- A ticket is archived **if and only if** it lives in `.tickets/archive/`.
- `archive/` holds files flat, regardless of depth, and has no `ROOT.md`. It is
  created lazily by the first successful archive; with `--dir D` it is `D/archive/`.
- Files move **byte for byte**: frontmatter, status and body are not rewritten. An
  archived ticket keeps its own `children` and `blocked-by` lists, so an archived
  subtree stays self-consistent inside the archive.

`tk archive <id> [--force]` moves the subtree rooted at a live `<id>` into
`archive/`, printing one line per moved ticket: `<id> archived "<title>"`.

- Every ticket in the subtree must be `closed`; `--force` first closes the whole
  subtree (as `close --force`).
- The live tree keeps **no reference** to an archived ticket: its id is removed
  from its parent's `children` list, or from `ROOT.md` when it was a root.
- The archive is refused while any `blocked-by` edge crosses the subtree boundary
  — a live ticket listing a subtree member, or a subtree member listing a live
  ticket. `--force` never overrides this. Edges with both ends inside the subtree
  move with it.
- Filenames do not collide: if `archive/<id>.md` already exists, the incoming file
  gets the shortest free numeric suffix (`<id>-2.md`, `<id>-3.md`, …). Only files
  whose exact name is taken are suffixed; the frontmatter id is unchanged.
- The move is best-effort: files are renamed and the live list rewritten without a
  journal, so a crash can leave a half-moved subtree. The live tree's normal `fsck`
  checks report the damage — a dangling child id or an orphaned file.

`tk archive --all` archives every fully closed subtree in one batch, printing the
same lines in DFS preorder. It takes no id, and `--force` is rejected with it.

- The candidates are the **maximal** subtrees whose tickets are all closed: a closed
  leaf under an epic that is still open is archived and the epic stays.
- A `blocked-by` edge crosses only when its other end **stays live**; edges between
  tickets archived in the same run move with them.
- A candidate with a crossing edge is **skipped** instead of refusing the run: it
  stays live, and so do its ancestors. Its closed siblings and their subtrees are
  still archived. Skipping can make another candidate cross (it now waits on, or
  blocks, a live ticket), so selection repeats until nothing changes.
- Each skipped ticket that crosses gets one line on stderr, e.g.
  `skipped <id>: waits on <id>, blocks <id>`. Ancestors kept only to hold a skipped
  ticket are not reported. Skips do not fail the command: it exits 0.
- With nothing to archive it prints nothing and exits 0.
- The whole batch is one move: lists are rewritten once, then the files renamed,
  with the same filename suffixing and best-effort guarantees as above.

`-n` / `--dry-run`, with either form, runs the same checks and prints the same
output with `would archive` in place of `archived`, but changes nothing: no
ticket is closed (even with `--force`), moved or rewritten. A run that would be
refused fails the same way.

Every other command addresses the live tree only. Id resolution never scans
`archive/`, there is no `archive:` prefix, and an id that lives only in the archive
reports `not found`. `tk new` does not consult the archive, and `fsck` has no
archive-specific checks.

## Implementation notes

- Language and libraries: Go, `spf13/cobra`, `gopkg.in/yaml.v3`.
- Packages: `internal/store` (load/save, ID resolution, atomic writes), `internal/tree` (DFS, ready, dep validation, mv), `cmd/` (thin cobra commands).
