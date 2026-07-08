# Whiteboard

waldo's **project journal** — key decisions, design discussions, and the
gotchas/issues hit along the way, kept so future-me knows *why*, not just *what*.
Distilled notes, not transcripts.

**One file per day**, named `YYYY-MM-DD.md`, holding that day's entries: decisions
made, problems encountered, and models that clicked. Newest first below.

## Entries

- **[2026-07-09](2026-07-09.md)** — Phase 1 kickoff: store/eviction design & the
  concurrency arc; why `Get` can't use `RLock` (integrity vs accuracy); the
  non-reentrant-mutex **deadlock**; the evict-before-add off-by-one; and the
  single-`RWMutex` `ParallelGet` baseline.
