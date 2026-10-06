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
| `tk fsck` | Verify integrity: orphans, ticket in two parents, dangling ids, cycles, dep rule violations (exit 1), plus `warn:` lines for tickets above their unclosed blockers (exit unaffected). Orphan lines hint at the repair: `tk mv <id> --root` (or `--under`) re-attaches the ticket with its subtree. |

Addressing is by id only (partial matching). Positional paths are never accepted as arguments, because they shift on reorder.

Free-form text input contract: inline, stdin with `-`, or file with `-F`.

## Non-goals

- Claims or assignees for multiple workers: single worker assumed (concurrent writes are serialized by the lock, but `ready` does not reserve a ticket). Use `tk ready <epic>` to scope parallel streams by hand.
- Priority, assignee, links, jq `query`, live `--watch` tree, `prune`, `clean`.
- Positional addressing and a single outline file.

Archival is specified in [Appendix: Archival](#appendix-archival). An alternative
design that relocates archived subtrees is specified in
[Appendix B: Archival by relocation](#appendix-b-archival-by-relocation).

## Implementation notes

- Language and libraries: Go, `spf13/cobra`, `gopkg.in/yaml.v3`.
- Packages: `internal/store` (load/save, ID resolution, atomic writes), `internal/tree` (DFS, ready, dep validation, mv), `cmd/` (thin cobra commands).

## Appendix: Archival

Archiving is a reversible hiding state layered *on top of* status. It never changes
a status: it only removes a ticket and its whole subtree from listings, selection
and warnings, while keeping every file on disk so `unarchive` can restore it
exactly where it was.

### On-disk representation

An archived ticket carries `archived: true` in its frontmatter. The key is omitted
whenever it is false, so tickets written before this feature read as unarchived.

```markdown
---
id: fanir7
status: closed
type: task
archived: true
blocked-by: [lovet2]
children: [kamop3]
created: 2026-10-05T12:00:00Z
---
# Title
```

Frontmatter parsing is strict, so an older `tk` binary rejects a ticket once it
contains the unknown `archived` key. This is accepted: archival is a v2 feature and
the tickets directory is expected to move as one.

### Invariants

- An archived ticket is always `closed`.
- Every descendant of an archived ticket is archived.
- The converse is allowed: an unarchived ticket may have archived children.

`tk fsck` reports a new **`archive`** problem (exit 1) when either invariant is
broken, i.e. an archived ticket whose status is not closed, or an archived ticket
with any unarchived descendant. Blockers and deps that point at an archived ticket
are not problems.

### Commands

| Command | Behaviour |
|---|---|
| `tk archive <id> [--force]` | Archive the ticket and its whole subtree. Requires every ticket in the subtree to be closed; `--force` closes the subtree first (as if by `close --force`). Fails while anything in the subtree is unclosed. |
| `tk unarchive <id>` | Unarchive the ticket's subtree and every archived ancestor (required so the ticket is reachable again). Statuses are unchanged (they are already closed). A no-op on a ticket that is not archived. |

- `archive` prints each affected ticket as `<id> archived "<title>"`; `unarchive`
  prints `<id> unarchived "<title>"`.
- `rm` may still permanently delete an archived ticket (`--force` for a subtree).
  Every other mutating command — `start`, `close`, `reopen`, `note`, `edit`, `type`,
  `dep`, `mv`, `up`, `down`, `top`, `bottom` — refuses an archived ticket.
  `show` still reads one.
- `new` or `mv` under an archived parent is refused, and so is `dep` when either the
  holder or the blocker is archived. Unarchive first.
- Partial-id resolution treats archived tickets like any other: a partial id may
  resolve to an archived ticket, and the command then reports why it refuses.

### Visibility

- `ls`, `ready`, `tui`, `fsck`'s misorder warnings and the `dep`/`mv` warnings skip
  archived tickets and their subtrees. `ready` never selects an archived ticket and
  never stops on one.
- `ls --archived` reveals archived subtrees; `ls --all` reveals closed-only
  subtrees. Archived hiding dominates: an archived node appears only with
  `--archived`, a non-archived but closed node only with `--all`, and a subtree that
  is both needs both flags. Passing both shows everything.
- `ls <archived-id>` shows the whole archived subtree, as if `--archived` were
  scoped to it.
- Archived lines use the `[a]` marker; `show` prints `archived: true`.

### Positions

Positions shown for the visible tree skip archived nodes, so `ls` shows no gaps.
A parallel *raw* position — a DFS that counts every ticket, archived included — is
used only when displaying an archived ticket, in `show`, `ls --archived` and
`unarchive`. Positions shift when a ticket is archived or unarchived.

### Blocking

- An archived blocker counts as **satisfied**: a ticket that waits on it is no
  longer blocked, and `ready` never reports it as an unclosed blocker.
- An archived ticket's own `blocked-by` entries are ignored while it is archived.
- Positions in `ready` and in blocker warnings are the visible ones.

### Example

```console
$ tk ls
1 [ ] rasot4 [task] Ship it

$ tk new "Release 1.0"                    # fanir7
fanir7
$ tk new "Write docs" --under fanir7       # kamop3
kamop3
$ tk close fanir7 --force
kamop3 closed "Write docs"
fanir7 closed "Release 1.0"

$ tk archive fanir7
kamop3 archived "Write docs"
fanir7 archived "Release 1.0"

$ tk ls
1 [ ] rasot4 [task] Ship it

$ tk ls --archived
1 [ ] rasot4 [task] Ship it
2 [a] fanir7 [task] Release 1.0
  2.1 [a] kamop3 [task] Write docs

$ tk show kamop3
# Write docs
id:       kamop3
status:   closed
archived: true
position: 2.1
...

$ tk start kamop3
kamop3: archived

$ tk unarchive kamop3
kamop3 unarchived "Write docs"
fanir7 unarchived "Release 1.0"

$ tk ls --all
1 [ ] rasot4 [task] Ship it
2 [x] fanir7 [task] Release 1.0
  2.1 [x] kamop3 [task] Write docs
```

## Appendix B: Archival by relocation

An alternative to [Appendix: Archival](#appendix-archival), designed from
scratch: it shares no state or rules with that design. Where Appendix A hides a
ticket in place with an `archived` flag, this design *moves* a finished subtree
out of the tree into a sibling directory. The move is **one-way** and the
directory is the only record that a ticket was archived. There is no `archived`
frontmatter key and no `unarchive`.

### On-disk layout

```
.tickets/
  ROOT.md
  <id>.md            # live tickets
  archive/
    <id>.md          # archived tickets, flat regardless of depth
```

- A ticket is archived **if and only if it lives in `.tickets/archive/`**.
- `archive/` is created lazily by the first successful `archive`; it has no
  `ROOT.md`. With `--dir D`, the archive is `D/archive/`.
- Files are moved **byte for byte**: frontmatter, status and body are not
  rewritten. An archived ticket keeps its own `children` and `blocked-by` lists,
  so an archived subtree stays self-consistent inside the archive.

### Invariants

- An archived ticket is always `closed`.
- The live tree keeps **no reference** to an archived ticket: its id is removed
  from its parent's `children` list, or from `ROOT.md` when it was a root. The
  archive is a detached side-store.
- No `blocked-by` edge may cross the boundary. An archive is refused while any
  live ticket lists a subtree member, or any subtree member lists a live ticket.
  Edges with both ends inside the subtree are ignored and move with it. This
  refusal is absolute: `--force` never overrides it.

### Commands

| Command | Behaviour |
|---|---|
| `tk archive <id> [--force]` | Move the closed subtree rooted at a live `<id>` into `archive/`; `--force` closes the subtree first. Refused on a crossing `blocked-by` edge. |
| `tk rm archive:<id> [--force]` | Permanently delete an archived ticket; `--force` deletes its archived subtree. |
| `tk show archive:<id>` | Read an archived ticket. The only command that reads one. |

### `tk archive <id> [--force]`

- Accepts exactly one **live** id. `archive:<id>` fails with `already archived`,
  and an id that exists only in the archive fails with
  `not found (try show archive:<id>)`.
- Requires every ticket in the subtree to be closed. `--force` first closes the
  whole subtree (as `close --force`) and does nothing else.
- Refuses while any `blocked-by` edge crosses the subtree boundary (see
  invariants); `undep` first. Internal edges are ignored.
- Prints one line per moved ticket: `<id> archived "<title>"`.
- The move is best-effort: files are renamed and the live list rewritten without
  a journal, so a crash can leave a half-moved subtree. `fsck` reports it.

### Addressing

- Archived tickets are addressed as `archive:<id>` or `archive:<partial>`, and
  resolve only against `.tickets/archive/` (exact match, then unique substring).
- Archived tickets are **not listable**: `ls`, `ready`, `tui`, fsck's misorder
  warnings and the `dep`/`mv` warnings never see them, and no flag reveals them.
  The normal loader scans `.tickets/` only; `show archive:`, `rm archive:` and
  `fsck` read the archive directory on demand.
- Only `show` reads an archived ticket. Every mutating command recognises the
  `archive:` prefix and refuses with `<id> is archived`.
- `show archive:<live-id>` fails with `<id> is not archived`; a bare id that
  exists only in the archive fails with `not found (try show archive:<id>)`.
- `tk new` checks both directories before issuing an id, so an archived id is
  never reused.

### `tk show archive:<id>`

Prints the normal `show` layout plus an `archived: true` field line. It shows no
position (the archive has no ordered root list) and lists children by id, so an
archived epic stays navigable.

### Deletion

`tk rm archive:<id>` permanently deletes an archived ticket; `--force` deletes
its whole archived subtree, walking the archived `children` lists. `archive` and
`rm archive:` are the only commands that write to `archive/`.

### `tk fsck`

Reports the following as **`archive`** problems (exit 1), without repair:

- an archived ticket whose status is not `closed`;
- an archived ticket whose listed child is missing from the archive;
- an id that exists in both `.tickets/` and `.tickets/archive/`;
- a half-moved subtree, or a live reference left pointing at an id that now
  lives only in the archive.

`archive:` addressing needs no separate fsck check: a live ticket that was moved
while still referenced is one of the half-move cases above.

### Concurrency

Archive operations reuse the existing `.tickets/.lock`: exclusive for `archive`
and `rm archive:`, shared for `show archive:` and `fsck`.

### Compatibility

`archived: true` is not part of this design. Frontmatter parsing is strict, so a
ticket carrying that key is unreadable and `fsck` reports it, exactly like any
other unknown key. Nothing else changes: the live tree, `ls`, `ready` and `tui`
are unaffected because they never scan `archive/`.

### Example

```console
$ tk ls
1 [ ] rasot4 [task] Ship it

$ tk new "Release 1.0"                    # fanir7
fanir7
$ tk new "Write docs" --under fanir7       # kamop3
kamop3
$ tk close fanir7 --force
kamop3 closed "Write docs"
fanir7 closed "Release 1.0"

$ tk archive fanir7
kamop3 archived "Write docs"
fanir7 archived "Release 1.0"

$ tk ls
1 [ ] rasot4 [task] Ship it

$ tk show archive:fanir7
# Release 1.0
id:       fanir7
status:   closed
type:     task
archived: true
children: [kamop3]
...

$ tk show archive:kamop3
# Write docs
id:       kamop3
status:   closed
type:     task
archived: true
...

$ tk start fanir7
fanir7: not found

$ tk start archive:fanir7
fanir7 is archived; only show can read it

$ ls .tickets/archive
fanir7.md  kamop3.md

$ tk rm archive:fanir7 --force
kamop3 removed
fanir7 removed
```
