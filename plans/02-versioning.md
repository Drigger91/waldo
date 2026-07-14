# 02 — Versioning (keep-last-N per key)

**Status:** ⬜ next
**Goal:** keep the last N versions of each key — the incremental on-ramp to full
MVCC (04), and the feature behind prompt-versioning in the AI use case.

## Design sketch (firm up before coding)

- Each key maps to a **bounded chain of versions**, newest-first, capped at N:
  `[]version{ seq uint64, value V, ts int64 }`.
- A monotonic `seq` (atomic counter) is stamped on every write — this is also the
  seed for MVCC later.
- API additions (shape TBD — decide in the design pass):
  - `Set(k, v)` → append a new version, drop the oldest beyond N.
  - `Get(k)` → latest version's value (unchanged behaviour).
  - `History(k)` / `GetVersion(k, i)` → older versions.
- Eviction (LRU) still keys on the **logical key**, not per-version: one key = one
  cache entry regardless of how many versions it holds.

## Open questions (resolve in the design pass)

- N as a per-store `MaxVersions` option (start here) vs per-key override?
- Read-at-`seq` now, or defer snapshot reads to MVCC (04)? (Lean: latest + explicit
  version index now; snapshot isolation arrives with MVCC.)
- `Delete` semantics: drop the whole chain, or push a tombstone version?
- Does versioning interact with the count budget? (Lean: no — versions are internal
  to one entry.)

## Tasks

- [ ] design pass → whiteboard entry (lock the API + the open questions above)
- [ ] version-chain data structure + unit tests
- [ ] monotonic `seq` counter
- [ ] `History` / `GetVersion` API
- [ ] `go test -race` + a small benchmark (append cost, memory per version)

## Exit criteria

Store and retrieve the last N versions of a key; versions older than N are dropped;
green under `-race`.

## Why this generalizes to MVCC (04)

Fixed-N → snapshot-visible: **"keep the last N"** becomes **"keep every version a
live snapshot can still see,"** and **"drop the oldest"** becomes **GC by the oldest
live read sequence.** Same version chain, richer visibility rule. Building the
bounded version now makes the unbounded, isolation-aware version tractable later.
