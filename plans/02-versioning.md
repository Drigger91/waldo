# 02 — Versioning (keep-last-N per key)

**Status:** 🟡 in progress — design settled ([whiteboard/2026-07-14](../whiteboard/2026-07-14.md)); skeletons next
**Goal:** keep the last N versions of each key — the incremental on-ramp to full
MVCC (04), and the feature behind prompt-versioning in the AI use case.

## Decided design

```go
type Version[V any] struct { Seq uint64; Value V; Ts int64 }
type entry[V any]   struct { versions []Version[V] } // oldest-first, len ≤ MaxVersions
```

- **Unified Store** with `Options.MaxVersions`: unset → `DefaultMaxVersions` (5),
  `1` → today's latest-only cache, any higher value accepted. **No upper clamp** —
  a hard ceiling is something MVCC (04) would only have to remove.
- Monotonic store-level `seq`, bumped under the write lock, stamped on every `Set`.
- `Set` appends a version, drops the oldest beyond N.
- `Get` returns the latest version's value (unchanged behaviour).
- `History(key) []Version[V]` — newest-first **copy**, exposes `Seq` + `Ts`.
- `Delete` drops the whole chain (map delete already does this — no change needed).
- LRU + count budget key on the **logical key**; one key = one entry regardless of
  version count.

## Tasks

- [ ] `Version[V]` type + `Options.MaxVersions` + `History` on the `Store` interface (waldo.go)
- [ ] `entry.versions` chain + `latest()` helper; update `Get` to read the latest
- [ ] `pushVersion` — append + trim to `MaxVersions` (the meaty bit)
- [ ] wire `Set` to `pushVersion` + policy/eviction
- [ ] `History` — newest-first copy
- [ ] `TestStore_Versioning` green + `-race`; small bench (append cost / mem per version)

## Exit criteria

Store and retrieve the last N versions of a key; versions older than N are dropped;
`Get` still returns latest; green under `-race`. `MaxVersions: 1` reproduces the
old latest-only cache exactly; an unset `MaxVersions` keeps 5.

## Deferred to later milestones

- Read-at-`seq` / snapshot reads → [MVCC (03)](03-mvcc.md).
- `Delete` as a tombstone version → [MVCC (03)](03-mvcc.md).
- `seq` → `atomic.AddUint64`: not needed while every mutation holds the write lock.
  Arrives with 03.
- Per-key version override, byte-size accounting of versions → later.

**Next after this:** [03 — MVCC & transactions](03-mvcc.md). The `Seq` field
`History` exposes is what snapshot reads walk; that's why it's in the API now.

## Why this generalises to MVCC (04)

Fixed-N → snapshot-visible: **"keep the last N"** becomes **"keep every version a
live snapshot can still see,"** and **"drop the oldest"** becomes **GC by the oldest
live read sequence.** Same version chain, richer visibility rule.
