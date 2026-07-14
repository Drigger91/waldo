# Plans

Precise, **living** execution plans — the "what to do next," updated as work lands
(checkboxes flip, status changes). This is the project's persistent memory: **to
resume, read this file, then the current milestone's plan.**

How this differs from the other docs:
- **`/README.md`** — *why* + high-level roadmap (rarely changes).
- **`plans/`** — *what & how*, with live status (this dir).
- **`whiteboard/YYYY-MM-DD.md`** — dated log of decisions & gotchas (append-only history).

## Direction

Build the **waldo core first** — a versioned, durable, embeddable KV engine — then
add thin **network faces** (MCP / gRPC / REST) for AI use cases: prompt versioning,
response caching, agent memory. Faces depend on the core; the core never imports a
face. The product *motivates* the roadmap; it does not reorder it around wrappers.

## Sequence & status

| # | Milestone | Goal (the hard thing) | Status |
|---|---|---|---|
| 01 | [Core store, concurrency, eviction](01-core-store.md) | thread-safe generic KV + pluggable LRU; *feel* lock contention | 🟡 first cut green; sharding next |
| 02 | [Versioning (keep-last-N)](02-versioning.md) | version chains per key — the MVCC on-ramp | ⬜ next |
| 03 | WAL — durability & recovery | writes survive a crash; replay rebuilds state | ⬜ planned |
| 04 | MVCC & transactions | snapshot isolation; generalize versioning | ⬜ planned |
| 05 | Network faces (MCP/gRPC/REST) | thin protocol layer over the core | ⬜ planned |

Detailed task lists exist for the **current and next** milestones; later rows stay
one-liners until we reach them — no point planning far-out work that will change.

**Current focus → 02 Versioning.**
