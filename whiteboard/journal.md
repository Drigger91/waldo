# Project journal

A dated, running log of the *interesting* parts of building waldo: key decisions,
the design discussions behind them, the gotchas that cost me time, and the mental
models that finally clicked. Distilled on purpose — this is the takeaway, not the
conversation that produced it. Newest entries at the bottom; append-only-ish (if the
thinking changes, add a superseding entry rather than rewriting history).

---

## 2026-07-09 — Phase 1 design: store, eviction & concurrency (the plan)

Phase 1 is ~200 lines sitting on a handful of design questions. Here's where they
landed and why.

### Store / eviction split — "who owns the bytes?"

- **A — policy tracks keys only; store owns values.** Two maps keyed by `K` (key
  stored twice), but the policy is tiny, `V`-agnostic, and swapping LRU→FIFO→LFU is
  swapping one small object. The cleanest "eviction is an interface."
- **B — policy owns the whole entry (K+V).** One map, one lookup — but the policy
  *is* the storage engine, generic over `K` and `V`, and every policy re-implements
  the map plumbing. This is what `hashicorp/golang-lru` actually is.
- **C — one map, intrusive list.** Store owns `map[K]*entry` with an opaque handle;
  one lookup, still swappable, at the cost of coupling. The eventual optimization.

**Decision: start with A**; build **C later and benchmark the delta** (same species
of lesson as RWMutex→sharded locks, the whole project thesis).

**Refinement while writing skeletons:** the policy ended up **cost-free**
(`Add(key)`, no `cost`). The store already tracks `curBytes` via each entry's cached
cost and drives eviction, so the policy only answers "which key is next?" → clean
split: **policy = ordering, store = accounting**. A future *size-aware* policy would
thread cost back in.

### Dual capacity budget

`MaxEntries` (init) **and** `MaxBytes` (global), evict-while-over-**either**. The
store computes per-entry cost via `SizeOf func(V) int64` and tracks the running
total; the byte budget never leaks into the policy.

### Oversized values

One value alone exceeds `MaxBytes` → **evict to fit, then store-and-exceed**,
configurable via `RejectOversized` (default false). Don't hardcode the log — expose
`OnEvict(key, reason)` (Phase 2 WAL tombstones will want the same seam).

### The concurrency arc (each step benchmarked against the last)

1. **Single `RWMutex`, full `Lock` in `Get`** — honest baseline; the RWMutex buys
   nothing yet, and that's the point.
2. **Sharded locks** — `shard = hash(key) % N`; each shard owns its own mutex, LRU
   list, and slice of the budget. The *bounded* form of "lock per key." Trade-offs:
   per-shard (not global) LRU, split byte budget, cross-shard `Len`/`Scan`.
3. **Timestamp + sampling** — kill the shared list entirely (see next entry).

### Decided

- Eviction **Design A**, policy **cost-free**; store owns values + budgets + accounting.
- **Dual budget** (`MaxEntries` + `MaxBytes`), evict-while-over-either.
- **Oversized**: evict-to-fit + store-and-exceed; `RejectOversized` default false;
  `OnEvict` hook, not a hardcoded log.
- **Generics**: `K comparable`, `V any`.
- **First cut**: single `RWMutex`, full `Lock` in `Get`, exact LRU via
  `container/list` + `map[K]*list.Element`; v0 API `Get`/`Set`/`Delete`/`Len`.
- **Open**: shard count `N`; logical-counter vs `time.Now()`; sample size `K`; TTL
  policy; `OnEvict`/`EvictReason` shape; `Scan` semantics.

---

## 2026-07-09 — Phase 1: the store, and why `Get` can't use `RLock`

Started implementing the store, things worth remembering.

### The core model: `RLock` fails on integrity, not accuracy

I kept wanting `Get` to take `RLock` (readers shouldn't block each other). Wrong,
and understanding *why* is the whole Phase 1 concurrency lesson. There are **two
separate problems**, and I was reasoning about the wrong one:

- **Problem A — eviction accuracy.** If concurrent recency bumps race and one is
  lost, eviction is slightly less accurate. This is *tolerant* — approximate LRU is
  fine, and real caches embrace it.
- **Problem B — data-structure integrity.** `Touch` calls `list.MoveToFront`, which
  rewires ~6 pointers. `RLock` lets *many goroutines run concurrently*, so two
  `Touch`es splice the same linked list at once → orphaned nodes, cycles, or the
  list desyncing from the map. A later `Evict` then walks a corrupt list and
  hangs or panics. This is *not* tolerant.

**The lock exists for B, not A.** Key realisation: this corruption happens even
with a huge cache and **zero evictions** — a read-only, all-`Get` workload still
has concurrent `MoveToFront` calls. Capacity is irrelevant; the damage is to the
*structure*, not the eviction *decision*.

Rule with no exceptions: concurrent writes to the same non-atomic memory = UB. A
multi-pointer list splice needs *exclusive* access, so a mutating `Get` needs
`Lock`, not `RLock`.

**When `RLock` (or lock-free) *is* allowed:** when the per-`Get` update is a single
*atomic word* write instead of a splice — i.e. a timestamp: `atomic.StoreInt64(&e.atime, clock)`.
Concurrent atomic writes to one word are defined and safe because there's no
multi-step structure to corrupt. That's a *different data structure* (timestamps +
sampling), and it's where "approximate eviction is fine" (Problem A) finally
becomes the justification — for the sampling, not for the lock.

### Design space: can the recency work move off the read path? (backpressure)

Idea: instead of splicing under a lock, have `Get` *fire* a "touched K" signal and
return; a background goroutine applies it. The reader stops paying for the write.
The real question this raises is **where backpressure lands** when signals arrive
faster than they're applied:

| Strategy | Backpressure lands on | Verdict |
|---|---|---|
| Synchronous `Lock` (baseline) | the reader (blocks on mutex) | correct, simple |
| Bounded queue, **blocking** send | the reader (blocks when full) | reinvents the lock, but worse (channel cost + still blocks) |
| Bounded queue, **lossy** drop | nobody (touch discarded) | legit *iff* recency is droppable |
| Unbounded queue | nobody… until OOM | trap — never |

**Deciding principle (the transferable one):** *can the result be dropped, or is it
a promise?*
- Recency is **best-effort** → dropping a touch is fine → the **lossy** buffer is
  legitimate and pushes backpressure to "nobody"; the reader never waits and
  integrity (B) is preserved because a *single* applier goroutine owns the list.
- Contrast Phase 2 **group commit**: the result is *durability*, a **promise** →
  you must NOT drop → backpressure must block or reject the writer. Same machinery,
  opposite policy, because one result is droppable and the other is a contract.

**Reality check:** a Go `chan` is itself mutex-backed; a send *per Get* can cost
more than the lock you're avoiding. Ristretto's win isn't "streaming" — it's
**lossy** (never block the reader, via a striped ring buffer) **+ batching** (apply
~64 touches under one lock acquisition). You amortise the lock, not remove it.

**Decision:** keep the synchronous `Lock` for the exact-LRU baseline (correct,
simple, benchmarkable). The lossy-buffer / timestamp variant belongs to the
approximate-LRU step, built and then **benchmarked head-to-head** against the
baseline — because it is genuinely *not guaranteed* to win. Measure, don't guess.

**State:** `Get` implemented (post-fix). Next: `LRU.Evict` → `Set` + `evictToFit`,
then capture the `BenchmarkStore_ParallelGet` baseline before touching sharding.
