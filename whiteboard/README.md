# Whiteboard

waldo's **project journal** — key decisions, design discussions, and the
gotchas/issues hit along the way, kept so future-me knows *why*, not just *what*.
Distilled notes, not transcripts.

**One file per day**, named `YYYY-MM-DD.md`, holding that day's entries: decisions
made, problems encountered, and models that clicked. Newest first below.

## Entries

- **[2026-07-20](2026-07-20.md)** — versioning defaults closed (5, no cap); `seq` is
  the shared primitive across versioning/WAL/MVCC/LSM; **SSTables deferred** (03 is
  WAL **+ snapshot**, since `MaxEntries` bounds memory by construction);
  inline-vs-background rule; product re-factored into **core engine + separate
  utility project**, making `OnEvict` load-bearing and possibly pulling the network
  face earlier.
- **[2026-07-14](2026-07-14.md)** — direction crystallised (core-first, then thin AI
  faces); versioning (02) design decided — unified `Store` + `MaxVersions`,
  `History` returning `[]Version[V]`, `Delete` drops the chain.
- **[2026-07-09](2026-07-09.md)** — Phase 1 kickoff: store/eviction design & the
  concurrency arc; why `Get` can't use `RLock` (integrity vs accuracy); the
  non-reentrant-mutex **deadlock**; the evict-before-add off-by-one; and the
  single-`RWMutex` `ParallelGet` baseline.
