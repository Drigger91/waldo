# 001 — Phase 1: core store, eviction & concurrency

*Session date: 2026-07-09 · Status: design settled, ready for skeletons*

Phase 1 is ~200 lines of code sitting on top of a handful of design questions.
This entry is those questions and their answers. No code yet — the map, not the
territory.

---

## The store / eviction split — "who owns the bytes?"

Three shapes were on the table:

- **A — `Policy[K]` tracks keys only; store owns values.** Store keeps `map[K]V` +
  the budgets. Policy keeps its own recency structure (`map[K]*list.Element` +
  `list.List`) and hands back a victim key. Policy never sees `V`.
  - *Cost:* two maps keyed by `K` (key stored twice, two hash lookups per op).
  - *Payoff:* the policy is tiny and obviously correct; swapping LRU→FIFO→LFU is
    swapping one small object. The cleanest expression of "eviction is an interface."
- **B — Policy owns the whole entry (K+V).** One map, one lookup, no duplication —
  but the policy *is* the storage engine, is generic over `K` **and** `V`, and every
  new policy re-implements the map plumbing. This is what `hashicorp/golang-lru`
  actually is: "an LRU," not "pluggable policies."
- **C — one map, intrusive list (the eventual optimization).** Store owns
  `map[K]*entry` where `entry` carries an opaque `policyData any` handle; the policy
  operates on that handle. One lookup, still swappable, at the cost of a little
  coupling + a type assertion.

**Decision: start with A.** It's the honest teaching version and runs clean under
`-race` on the first try. Implement **C later** and *benchmark the delta* — that
before/after is the same species of lesson as RWMutex→sharded locks, which is the
whole project thesis.

### Why the dual budget (count + bytes) settles A vs B

To evict by bytes, *something* must know each entry's size. In B the policy holds
`V` and could measure. In A the policy only knows keys — so the store **computes a
cost and passes it in as an `int64`.** The policy tracks a running total and evicts
until *both* limits are satisfied. Policy stays `V`-agnostic; the byte budget is
just an integer threaded through. So the dual-budget requirement doesn't push us to
B at all — it's satisfied cleanly inside A.

```go
type Policy[K comparable] interface {
    Add(k K, cost int64)   // record a new key with its weight
    Touch(k K)             // mark recently used (LRU: move to front; FIFO: no-op)
    Remove(k K)            // explicit Delete
    Evict() (K, bool)      // pick a victim; false if empty
}

type Options[V any] struct {
    MaxEntries int           // count cap, set at init (0 = unlimited)
    MaxBytes   int64         // global byte cap        (0 = unlimited)
    SizeOf     func(V) int64 // how to weigh a value
    // + eviction hook & oversized policy, below
}
```

Eviction after a `Set`: *while entries > MaxEntries OR bytes > MaxBytes → evict one
victim, subtract its cost.* Two independent high-water marks, one victim-selection
policy.

## Oversized values

If a single new value is bigger than `MaxBytes`, evicting everyone else still leaves
us over budget. Decision:

- **Evict to make room, then store-and-exceed** (blow the budget for the one giant
  value) — configurable via `RejectOversized bool`, default **false**.
- **Don't hardcode the log line.** Expose `OnEvict func(key K, reason EvictReason)`.
  Logging is one use of the hook; metrics, cascading deletes, and (Phase 2) WAL
  tombstones want the same seam.

## The concurrency crux — `Get` is not a read

Under LRU, `Get` moves the touched key to the front of the recency list — a
**mutation** of a structure shared by every key. So the intuitive `RLock` is a lie;
two concurrent `Get`s corrupt the list and `-race` screams.

We walked the escape hatches:

1. **Full `Lock` in `Get`.** Correct, trivial — but now `RWMutex` buys nothing;
   every read serializes. **This is the baseline, named honestly.**
2. **Sharded locks.** `shard = hash(key) % N`; each shard owns its **own mutex, own
   LRU list, and own slice of the budget**. Key A and key B on different shards run
   truly parallel. This is the correct, *bounded* form of the "lock per key" idea —
   a per-key lock can't work because it protects the value but **not** the shared
   list/counters, and a `map[K]*Mutex` grows unboundedly (when do you delete a key's
   mutex?) and needs a lock to protect the lock map. Sharding = fixed N locks, each
   guarding its own partitioned eviction structure.
   - *Trade-offs bought:* per-shard (not global) LRU; the byte budget is split
     `MaxBytes/N`; cross-shard ops (`Len`, `Scan`, future txns) must touch every
     shard. Redis/Ristretto accept exactly these.
3. **Kill the shared structure — timestamp + sampling.** Put an `atime` (a cheap
   monotonic *logical counter*, not `time.Now()` — the wall clock is a per-`Get`
   syscall we don't need) on each entry. `Get` = one `atomic.Store`, lock-free. No
   list at all. Eviction can't pop a tail, so **sample K random entries and evict the
   oldest** (K≈5, Redis's `allkeys-lru`). Trade exact LRU for a lock-free read.
   Bonus: the timestamp lives *in* the entry, so `Delete` can't strand it — no
   lifecycle edge cases.
4. **Actor / buffered touches.** Funnel touches down to **one** background goroutine
   (single-writer principle → list needs no lock). Key refinements we landed on:
   - `Get` should **fire-and-forget**, not wait for an ack — the caller already has
     its value; recency is best-effort. Waiting reintroduces the sync point we're
     removing.
   - Make the buffer **lossy** (drop touches when full) so the reader *never* blocks.
   - **Honest catch:** a Go `chan` is itself a mutex-backed structure; a channel send
     *per Get* can cost *more* than the shard lock. Ristretto uses a lossy striped
     ring buffer + batching, not a channel. Only `-bench` reveals which wins.

**This actor is Phase 2 in disguise.** Group commit is the same pattern — many
writers, one background goroutine — except group commit *does* wait for the ack
(durability is a promise), while the recency-touch actor does *not* (recency is
best-effort). Knowing which side of that line you're on is the core instinct.

**Build order:** baseline global lock → sharding → timestamp+sampling, benchmarking
each step so the deltas are the lesson.

---

## Decided ✅

- Eviction **Design A**: store owns values + budgets; `Policy[K]` tracks keys +
  per-key `cost` and returns a victim. `V`-agnostic.
- **Dual budget**: `MaxEntries` (init) + `MaxBytes` (global); evict-while-over-either.
- **Oversized**: evict-to-make-room + store-and-exceed; `RejectOversized` (default
  false); `OnEvict` hook instead of a hardcoded log.
- **Generics**: `K comparable`, `V any`.
- **Concurrency arc** agreed: global lock → shard → timestamp+sampling, benchmark each.

## Decided for the *first cut* ✅

- **Single `sync.RWMutex` with a full `Lock` in `Get`** — not sharded, not timestamped
  yet. Dumb-but-correct baseline to measure against.
- **v0 API surface**: `Get`, `Set`, `Delete`, `Len`. **`Scan` deferred** (iteration
  under concurrency is a distraction right now).
- **Exact LRU** via `container/list` + `map[K]*list.Element` — the thing the
  timestamp+sampling variant will later be compared against.

## Open — decide when we get there 💤

- Shard count `N`; logical-counter vs `time.Now()`; sample size `K`.
- TTL policy; exact `OnEvict` / `EvictReason` shape; `Scan` semantics.

## Next step

Skeletons: `Store[K,V]` + `Policy[K]` interfaces + `Options` struct handed over;
fill in the LRU bodies; get `go test -race` green; benchmark → that's the baseline
before sharding.
