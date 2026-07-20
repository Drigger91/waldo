# 01 — Core store, concurrency, eviction

**Status:** 🟡 in progress — first cut green; sharding remaining
**Goal:** a thread-safe, generic `Store[K,V]` with pluggable eviction — and *feel*
the cost of lock contention before fixing it.

## Done

- [x] `Store[K,V]` interface + `Options` + `New` (waldo.go)
- [x] cost-free `Policy[K]` interface — ordering only; store does accounting (eviction/)
- [x] exact LRU via `container/list` + map: `Touch`/`Remove`/`Evict`
- [x] single-`RWMutex` store: `Get`/`Set`/`Delete`/`Len`, count-based eviction (`MaxEntries`)
- [x] green under `go test -race ./...`
- [x] baseline benchmark: `ParallelGet` 33.6 → 57.8 → 105.4 → 124.9 ns/op at 1/2/4/8
      cores (Apple M2) — **negative scaling** under the single mutex

## Next (the payoff)

- [ ] **Sharded locks** — N shards, each with its own mutex + LRU list + budget slice;
      route by `hash(key) % N`
- [ ] re-run `ParallelGet` across cores — expect it to *scale up* now, not down
- [ ] record before/after numbers in the whiteboard

## Later / optional

- [ ] TTL eviction policy
- [ ] FIFO / LFU policies
- [ ] (parked) byte-size budget — `SizeOf` / reflection-based `ApproxSize`

## Exit criteria

Sharded store beats the single-mutex `ParallelGet` baseline at ≥4 cores and stays
green under `-race`.

## Notes

Design reasoning & gotchas: [2026-07-09](../whiteboard/2026-07-09.md) — the `RLock`
integrity-vs-accuracy model, the non-reentrant-mutex deadlock, the evict-before-add
off-by-one.
