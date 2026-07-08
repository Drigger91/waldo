// Package waldo is an embeddable, concurrent, generic key-value store.
//
// It is a library you import, not a server you connect to — think lru-cache or
// hashicorp/golang-lru. Phase 1 is the in-memory heart: a thread-safe store with
// a pluggable eviction policy and an entry-count capacity budget.
//
// (A byte-size budget is deliberately parked for a later increment — sizing an
// arbitrary/heterogeneous value is its own interesting problem. See the
// whiteboard journal.)
package waldo

import "github.com/Drigger91/waldo/eviction"

// Store is the public key-value interface.
//
// Note on Get: under an LRU policy, Get updates recency — so it is NOT a
// read-only operation internally, even though it reads to the caller. This is
// the central Phase 1 concurrency lesson (see the whiteboard journal).
type Store[K comparable, V any] interface {
	// Get returns the value for key and whether it was present.
	Get(key K) (value V, ok bool)

	// Set inserts or updates key, evicting as needed to stay within budget.
	Set(key K, value V)

	// Delete removes key if present (no-op otherwise).
	Delete(key K)

	// Len returns the current number of entries.
	Len() int
}

// Options configures a Store. The zero value is a valid, unbounded, in-memory
// cache with default LRU eviction (which never triggers, since MaxEntries is 0).
type Options[K comparable, V any] struct {
	// MaxEntries caps the number of entries. 0 means unlimited.
	MaxEntries int

	// Policy is the eviction policy. Defaults to LRU when nil.
	Policy eviction.Policy[K]

	// OnEvict, if set, is called for each evicted key. Runs while the store lock
	// is held — keep it cheap and non-blocking, and do NOT call back into the
	// store from it (that would deadlock).
	OnEvict func(key K)
}

// New builds a Store from opts. This is plumbing — the interesting code is in
// store.go's Get/Set/Delete.
func New[K comparable, V any](opts Options[K, V]) Store[K, V] {
	p := opts.Policy
	if p == nil {
		p = eviction.NewLRU[K]()
	}
	return &store[K, V]{
		items:    make(map[K]entry[V]),
		policy:   p,
		maxItems: opts.MaxEntries,
		onEvict:  opts.OnEvict,
	}
}
