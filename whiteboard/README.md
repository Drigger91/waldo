# Whiteboard

Design discussions and decision records for waldo, before (and alongside) the code.

Each entry captures a session: the problem we chewed on, the options we weighed,
the trade-offs, and — most importantly — **what we decided and why**. When future-me
wonders "why is `Get` allowed to take a write lock?" the answer lives here, not in a
buried code comment.

Read these in order; later entries assume the decisions in earlier ones.

## Entries

- [001 — Phase 1: core store, eviction & concurrency](001-phase1-core-store.md)

## Convention

- Numbered, dated, append-only-ish. Don't rewrite history to look smart in
  hindsight — if we change our mind, add a new entry that supersedes the old one
  and link back. The point is to see the *reasoning evolve* (very on-brand for a
  project about write-ahead logs).
- Each entry ends with a **Decided / Open** ledger so it's obvious what's locked
  and what's still up for grabs.
