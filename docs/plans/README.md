# Plans

Precise, **living** execution plans — the "what to do next," updated as work lands
(checkboxes flip, status changes). This is the project's persistent memory: **to
resume, read this file, then the current milestone's plan.**

How this differs from the other docs:
- **`/README.md`** — *why* + high-level roadmap (rarely changes).
- **`docs/plans/`** — *what & how*, with live status (this dir).
- **`docs/whiteboard/YYYY-MM-DD.md`** — dated log of decisions & gotchas (append-only
  history).

## Direction

**waldo is a database, not a cache** ([2026-07-20](../whiteboard/2026-07-20.md)) — an
embeddable, versioned, transactional KV engine. The learning goals that settle this:
**transactions, isolation levels, and WAL recovery.** Eviction is demoted to an
opt-in policy for non-transactional use; it is *incompatible* with snapshot
isolation, since a snapshot at seq N cannot read a key LRU already dropped. Real
engines bound version retention by the **oldest live reader**, never by recency.

Faces depend on the core; the core never imports a face. The smart layer
(fuzzy/semantic search, context management) is a **separate project** for later —
deferred, not designed, until there's a durable engine worth exposing.

## Sequence & status

| # | Milestone | Goal (the hard thing) | Status |
|---|---|---|---|
| 01 | [Core store, concurrency, eviction](01-core-store.md) | thread-safe generic KV + pluggable LRU; *feel* lock contention | 🟡 first cut green; sharding next |
| 02 | [Versioning (keep-last-N)](02-versioning.md) | version chains per key — the MVCC on-ramp | 🟡 in progress — skeleton + tests in; `pushVersion`/`History` open |
| 03 | [MVCC & transactions](03-mvcc.md) | snapshot reads; atomic commit; GC by oldest live reader | ⬜ next |
| 04 | Isolation levels | read-committed / SI / serializable — **proven by anomaly tests** | ⬜ planned |
| 05 | WAL + recovery | committed txns replay, incomplete ones don't; crash-tested | ⬜ planned |
| 06 | Sharded locks | close out 01's benchmark — pure in-memory opt under a fixed format | ⬜ planned |
| 07 | Network faces (MCP/gRPC/REST) | thin protocol layer over the core | ⬜ planned |

Detailed task lists exist for the **current and next** milestones; later rows stay
one-liners until we reach them — no point planning far-out work that will change.

**Current focus → 02 Versioning.**

## Why MVCC comes before the WAL (swapped 2026-07-20)

The hard part of a WAL is **atomicity**, and atomicity only means something once
there's a transaction boundary. A WAL over single-key writes is just a log —
append, replay, done — which is the easy 40% of the lesson. Transaction ids, commit
records, and "which transactions does recovery replay vs discard" only exist because
transactions do. Build the WAL first and it gets rewritten when they arrive.

MVCC is also a *continuation of 02* ("keep last N" → "keep what live snapshots
need"), whereas durability is genuinely orthogonal and layers cleanly on top of a
correct in-memory engine. And in-memory MVCC is far cheaper to iterate on: no fsync,
no crash harness, just logic.

## Standing decisions

- **No SSTables.** They solve dataset-exceeds-RAM and sorted range scans. Neither is
  a problem we have. Checkpointing (when 05 needs it, since WAL-only replay is O(the
  whole log)) is a periodic **snapshot**. Revisit only if unbounded volume shows up.
- **Global `seq`, never per-shard** — `atomic.AddUint64`. Per-shard counters make the
  *durable format* depend on the hash function and shard count, so N stops being a
  tuning knob and recovery breaks if either changes (note: `hash/maphash` is randomly
  seeded per process — that failure would be silent). The storage format must not
  depend on the concurrency strategy.
- **Trimming stays inline; background work is for GC and TTL.** Inline when the work
  is proportional to the write, background when it's proportional to the data. A
  sweeper would make `MaxVersions` an average rather than an invariant. Version GC in
  03 *is* the legitimate background case.
- **Eviction is opt-in and non-transactional.** `MaxEntries: 0` (unlimited) is
  already the zero value, so the default store is already a database.

## Open questions

- **Does WAL replay restore full version history or only the latest value?** ←
  blocks 05; changes the log record format. Lean: log the *version*
  (`seq, key, value, ts`) — costs nothing extra, and discarding history at replay is
  easy where inventing it later is impossible.
- **Does eviction stay in the API at all** once transactions land, or become a
  separate non-transactional mode? ← revisit at the end of 03.
