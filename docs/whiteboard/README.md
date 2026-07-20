# Whiteboard

waldo's **project journal** — key decisions, design discussions, and the
gotchas/issues hit along the way, kept so future-me knows *why*, not just *what*.
Distilled notes, not transcripts.

**One file per day**, named `YYYY-MM-DD.md`, holding that day's entries: decisions
made, problems encountered, and models that clicked. Newest first below.

## Entries

- **[2026-07-20](2026-07-20.md)** — **waldo is a database, not a cache** (eviction and
  snapshot isolation are incompatible); **MVCC moved ahead of the WAL**, since
  atomicity needs a transaction boundary to mean anything; SSTables deferred; global
  `seq`, never per-shard; inline-vs-background rule. Then 02 landed green, and the
  six bugs it took: **Go value semantics** (map values, slice headers, and reslicing
  all share or copy in ways that surprise), `History` silently breaking `Get` through
  a shared backing array, and the 07-09 **deadlock recurring** — a convention that
  lives only in a doc comment is not a safeguard.
- **[2026-07-14](2026-07-14.md)** — direction crystallised (core-first, then thin AI
  faces); versioning (02) design decided — unified `Store` + `MaxVersions`,
  `History` returning `[]Version[V]`, `Delete` drops the chain.
- **[2026-07-09](2026-07-09.md)** — Phase 1 kickoff: store/eviction design & the
  concurrency arc; why `Get` can't use `RLock` (integrity vs accuracy); the
  non-reentrant-mutex **deadlock**; the evict-before-add off-by-one; and the
  single-`RWMutex` `ParallelGet` baseline.
