# 03 — MVCC & transactions

**Status:** ⬜ next (blocked on 02 landing)
**Goal:** snapshot reads and atomic multi-key commit — the milestone the whole
project is actually for. Generalises 02's version chains from "keep last N" to
"keep every version a live snapshot can still see."

## Shape (firm up in a design pass before coding)

```go
func (s *store[K,V]) Begin() *Txn[K,V]
func (t *Txn[K,V]) Get(key K) (V, bool)   // newest version with Seq <= t.readSeq
func (t *Txn[K,V]) Set(key K, value V)    // buffers into t.writes, invisible to others
func (t *Txn[K,V]) Commit() error         // validate, then apply atomically
func (t *Txn[K,V]) Rollback()             // discard t.writes
```

- **Read seq** stamped at `Begin`; reads walk the chain for the newest version
  `<= readSeq`. This is why `History` returns `Seq` — 02 already seeded it.
- **Write set** is transaction-private until commit. Uncommitted writes are invisible
  to everyone, including the store's own `Get`.
- **Commit** takes the write lock, validates (see below), stamps every buffered write
  with one `commitSeq`, applies them, releases. Atomic because it's one critical
  section — no WAL needed yet.
- **Conflict rule** to decide in the design pass: first-committer-wins — abort if any
  key in the write set gained a version with `Seq > t.readSeq` since `Begin`.

## Version GC — the legitimate background case

Retention flips from *count-bounded* to *reader-bounded*: keep every version any live
snapshot can still see. Concretely, track the **oldest live read seq**; a version is
collectable once a newer version exists with `Seq <= oldestLiveRead`.

This is the background work rejected in 02 and correct here — it's proportional to
the *data*, not to the write, and cannot be done inline because a `Set` doesn't know
what other transactions are open. See the inline-vs-background rule in
[plans/README](README.md).

**Consequence:** `MaxVersions` stops being the retention rule under transactions. A
long-running reader legitimately pins more than N versions. Decide whether
`MaxVersions` becomes a floor, a hint, or is ignored in transactional mode.

## Eviction interaction — resolve before coding

LRU can drop a key a live snapshot still needs, silently breaking isolation. Options:
pin keys with live readers, disable eviction when a txn is open, or split
transactional / non-transactional modes. **Cheapest correct answer first**; this is
the trade the "waldo is a database" call was made to avoid dodging.

## Tasks

- [ ] design pass → whiteboard entry (conflict rule, GC trigger, eviction answer)
- [ ] `Txn` type; `Begin`/`Rollback` + read-seq plumbing
- [ ] snapshot read: newest version `<= readSeq` (chain walk)
- [ ] buffered write set; `Commit` validate-then-apply under one lock
- [ ] `seq` → `atomic.AddUint64` (global, never per-shard — see README)
- [ ] oldest-live-reader tracking + GC pass
- [ ] **property test**: random op sequences against a trivial reference model
- [ ] `go test -race` with concurrent transactions

## Exit criteria

Concurrent transactions read a stable snapshot; a committed txn is all-or-nothing;
conflicting writes abort rather than silently interleave; GC reclaims versions no
live reader can see; green under `-race`.

## Deferred

- Durability of commits → 05 (WAL). Commits are in-memory-atomic only until then.
- Named isolation levels → 04. This milestone builds snapshot isolation; 04 adds the
  others and the anomaly suite that proves the differences.
