# Waldo

An embeddable, concurrent key-value store written in Go — built from scratch as a
learning project. Think of it like Node's `lru-cache` or `hashicorp/golang-lru`:
a **library you import**, not a server you connect to.

The name is a nod to the **WAL** (Write-Ahead Log) that gives it durability —
"WAL-do."

> **Status:** learning project, pre-implementation. Correctness and understanding
> over practicality. If a feature exists mainly to teach a concept, that's the point.

> **Design decisions live in [`whiteboard/`](whiteboard/)** — the reasoning behind
> every non-obvious choice, recorded as we go. This README is the map; the whiteboard
> is how we drew it.

**The two things I most want to walk away understanding:**
1. **How WAL files recover state** — append, `fsync`, replay on startup, survive a
   crash mid-write (Phase 2).
2. **How transactions actually work** — isolation levels, MVCC/snapshot isolation,
   and nested transactions / savepoints (Phase 4).

Everything before those exists to make them reachable.

---

## Why this exists

I read [DDIA](https://dataintensive.net/) and wanted to *actually build* the
storage-engine concepts instead of just reading about them. The goals, in order:

1. **Deep-dive on concurrency in Go** — goroutines, channels, `sync` primitives,
   the actor pattern, the race detector. This is the main event.
2. **Understand durability for real** — WALs, `fsync`, group commit, crash recovery.
3. **Understand storage engines** — log-structured storage, eviction policies.
4. **Understand transactions** — MVCC, snapshot isolation, isolation levels.

It does not need to make practical sense. It needs to make me a better engineer.

## Design constraints (decided so far)

- It's a **package/library**, imported in-process — like `lru-cache`. No network
  layer required (a server could be a later, optional wrapper).
- **Thread-safe by default.** Concurrent access is a first-class concern, not an
  afterthought.
- **Pluggable eviction** — LRU to start, but the eviction policy should be an
  interface so LFU / FIFO / TTL can slot in.
- **Optional persistence** — a pure in-memory cache should work, and durability
  (WAL) should be an opt-in layer on top.
- Use **generics** for typed keys/values where it makes the API nicer.
- Every phase gets **benchmarks** (`go test -bench`) and runs clean under
  `go test -race`.

Module path: `github.com/Drigger91/waldo` (adjust if you fork/rename).

---

## Roadmap

Each phase teaches one hard thing and builds on the last. Adapted from our
original storage-engine plan to fit the "embeddable package with eviction" shape.

### Phase 1 — Core store + concurrency + eviction  ← start here
The in-memory heart of the package.

- A `Store[K, V]` interface: `Get`, `Set`, `Delete`, `Len`, maybe `Scan`.
- Thread-safety: start with a single `sync.RWMutex`, then evolve to **sharded
  locks** (N buckets, each with its own mutex) to reduce contention. Benchmark
  the difference — that delta is the whole lesson.
- **Eviction as an interface** so policies are swappable:
  - LRU (doubly-linked list + map — the classic).
  - Then FIFO, LFU, TTL-based expiry as separate implementations.
- **Go concepts:** `sync.RWMutex`, `container/list`, generics, table-driven tests,
  `go test -race`, `testing.B` benchmarks.

### Phase 2 — Write-Ahead Log (the durability deep-dive)
Make writes survive a crash. Append each mutation to a log and `fsync` *before*
acknowledging the `Set`.

- Record format: `[keylen | key | vallen | val | crc | timestamp]`.
- Segment rotation, CRC checksums, recovery that detects a torn tail write.
- **Group commit** — the money concept and a perfect channels problem: writers
  send entries down a channel; one background goroutine drains, writes, `fsync`s
  once for the whole batch, then signals every writer back. The **actor pattern**.
- On startup, **replay the log** to rebuild in-memory state — this *is* recovery.
  The headline test: `kill -9` mid-write, restart, assert no lost acked writes and
  no corruption (the torn tail is caught by CRC and discarded).
- **Go concepts:** `chan`, dedicated writer goroutine, `sync.WaitGroup`,
  `context.Context` for shutdown, `select`, `encoding/binary`, `os.File.Sync()`.

### Phase 3 — Log-structured storage (optional, DDIA's core)
Graduate from "cache with a log" to a real storage engine.

- Memtable (sorted in-memory) → flush to immutable, sorted **SSTables** on disk.
- Background **compaction** goroutine merges SSTables and drops tombstones.
- Sparse index + bloom filter per SSTable.
- **Go concepts:** background workers, `sync/atomic`, benchmarking, generics for
  a skiplist.

### Phase 4 — Transactions & MVCC (headline goal; can come right after Phase 2)
Phase 3 is optional, so this can follow the WAL directly — MVCC doesn't need the
log-structured storage first.

- Multiple versions per key, keyed by a monotonic sequence number.
- Read at a snapshot → **snapshot isolation** for free.
- **Nested transactions / savepoints** — a nested `BEGIN` marks a savepoint you can
  roll back *to*; only the outermost commit becomes durable. Maps cleanly onto the
  sequence-number model.
- Atomicity via the WAL's **commit record**: replay only committed txns on recovery,
  discard any half-finished one → a crash mid-transaction rolls back automatically.
- Optimistic conflict detection for writes.
- **Go concepts:** atomic counters, the race detector, designing an API that's
  safe under concurrent transactions. Feel the difference between read-committed,
  snapshot isolation, and **write skew** (the anomaly SI does *not* prevent).

### Phase 5 — Capstone (someday)
Network wrapper (RESP protocol so `redis-cli` works), or replication / Raft.
Explicitly season 2.

---

## Suggested package layout

```
waldo/
├── go.mod
├── README.md
├── whiteboard/         # design discussions & decision records
├── waldo.go            # public API: Store interface, New(), options
├── store.go            # core concurrent map implementation
├── eviction/
│   ├── eviction.go     # Policy interface
│   ├── lru.go
│   ├── fifo.go
│   └── ttl.go
├── wal/                # phase 2
│   ├── wal.go
│   ├── record.go
│   └── recovery.go
└── *_test.go           # tests + benchmarks alongside each package
```

Nothing here is implemented yet — this is the map, not the territory.

## Working agreement

- **I write code too.** Don't scaffold entire implementations end-to-end;
  explain the design, hand me the tricky primitive or a skeleton, let me fill in
  the rest, then review.
- Prefer teaching the *why* (especially concurrency trade-offs) over just the *what*.

## Ground rules / habits

- `go test -race ./...` on every change.
- Benchmark before/after concurrency changes; keep the numbers.
- Write the **crash test early** in Phase 2: kill the process mid-write, restart,
  assert no data loss / no corruption. That one test forces real understanding of
  durability.

---

## Resume here

**Phase 1 design is settled** — see [`whiteboard/001`](whiteboard/001-phase1-core-store.md)
for the full reasoning (eviction split, dual count+byte budget, the "`Get` is not a
read" concurrency crux, and the global-lock → shard → timestamp+sampling arc).

**Next step:** hand over `Store[K,V]` + `Policy[K]` interface skeletons + the
`Options` struct, then fill in the **first-cut** implementation:

- single `sync.RWMutex`, full `Lock` in `Get` (dumb-but-correct baseline)
- exact LRU via `container/list` + `map[K]*list.Element`
- v0 API: `Get`, `Set`, `Delete`, `Len` (`Scan` deferred)

Get `go test -race` green, benchmark it — that number is the baseline before
sharding.
