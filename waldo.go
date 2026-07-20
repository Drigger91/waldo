// Package waldo is an embeddable, concurrent, generic key-value database.
//
// It is a library you import, not a server you connect to. The in-memory heart
// is a thread-safe store keeping per-key version history; transactions with
// snapshot isolation and a write-ahead log follow (see plans/).
//
// Eviction is an opt-in policy, not the point: MaxEntries defaults to unlimited,
// and bounded eviction is incompatible with snapshot isolation — a snapshot
// cannot read a key LRU already dropped. Under transactions, version retention
// is bounded by the oldest live reader instead.
//
// (A byte-size budget is deliberately parked for a later increment — sizing an
// arbitrary/heterogeneous value is its own interesting problem. See the
// whiteboard journal.)
package waldo

import "github.com/Drigger91/waldo/eviction"

// Version is one stored revision of a key's value.
type Version[V any] struct {
	// Seq is monotonic and store-wide (never per-shard: that would make the
	// durable format depend on the hash function and shard count). Snapshot
	// reads will select the newest version with Seq <= the reader's seq.
	Seq   uint64
	Value V
	Ts    int64 // unix nanoseconds at write time
}

// Store is the public key-value interface.
//
// Note on Get: under an LRU policy, Get updates recency — so it is NOT a
// read-only operation internally, even though it reads to the caller. This is
// the central Phase 1 concurrency lesson (see the whiteboard journal).
type Store[K comparable, V any] interface {
	// Get returns the latest value for key and whether it was present.
	Get(key K) (value V, ok bool)

	// Set inserts a new version of key, evicting as needed to stay within budget.
	Set(key K, value V)

	// Delete removes key and all its versions if present (no-op otherwise).
	Delete(key K)

	// History returns key's kept versions, newest-first (empty if absent). The
	// number retained is Options.MaxVersions.
	History(key K) []Version[V]

	// Len returns the current number of entries (keys, not versions).
	Len() int
}

// DefaultMaxVersions is the version-history depth used when Options.MaxVersions
// is unset (< 1). Callers can raise or lower it explicitly.
const DefaultMaxVersions = 5

// Options configures a Store. The zero value is a valid, unbounded, in-memory
// store keeping up to DefaultMaxVersions revisions per key — eviction never
// triggers, since MaxEntries is 0.
type Options[K comparable, V any] struct {
	// MaxEntries caps the number of entries (keys). 0 (the default) means
	// unlimited. Setting it opts into eviction, which is incompatible with the
	// snapshot isolation coming in the MVCC milestone.
	MaxEntries int

	// MaxVersions is how many revisions of each key to retain for History.
	// Unset (0) uses DefaultMaxVersions. Set it to 1 for latest-only, or higher
	// for deeper history. There is deliberately no upper clamp.
	MaxVersions int

	// Policy is the eviction policy. Defaults to LRU when nil.
	Policy eviction.Policy[K]

	// OnEvict, if set, is called for each evicted key. Runs while the store lock
	// is held — keep it cheap and non-blocking, and do NOT call back into the
	// store from it (that would deadlock).
	OnEvict func(key K)
}

// New builds a Store from opts. This is plumbing — the interesting code is in
// store.go's Get/Set/Delete/History.
func New[K comparable, V any](opts Options[K, V]) Store[K, V] {
	p := opts.Policy
	if p == nil {
		p = eviction.NewLRU[K]()
	}
	maxVersions := opts.MaxVersions
	if maxVersions < 1 {
		maxVersions = DefaultMaxVersions
	}
	return &store[K, V]{
		items:       make(map[K]entry[V]),
		policy:      p,
		maxItems:    opts.MaxEntries,
		maxVersions: maxVersions,
		onEvict:     opts.OnEvict,
	}
}
