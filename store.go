package waldo

import (
	"slices"
	"sync"
	"time"

	"github.com/Drigger91/waldo/eviction"
)

// entry is what the store keeps per key: a bounded chain of versions, oldest
// first (newest at the end), capped at store.maxVersions.
type entry[V any] struct {
	versions []Version[V]
}

// latest returns the newest version's value, or (zero, false) if the chain is empty.
func (e entry[V]) latest() (V, bool) {
	if len(e.versions) == 0 {
		var zero V
		return zero, false
	}
	return e.versions[len(e.versions)-1].Value, true
}

// store is the Phase 1 first-cut implementation: a single RWMutex over one map.
//
// Deliberate choice (see the whiteboard journal): Get takes a FULL write lock, because
// under LRU it mutates recency. That means the RWMutex buys us nothing yet —
// every op serializes. That's the point: this is the baseline we benchmark, then
// beat with sharded locks. Name the weakness, don't hide it.
type store[K comparable, V any] struct {
	mu     sync.RWMutex
	items  map[K]entry[V]
	policy eviction.Policy[K]

	seq uint64 // monotonic version counter; bumped under mu

	// config (immutable after New)
	maxItems    int
	maxVersions int
	onEvict     func(key K)
}

// Get returns the value for key.
func (s *store[K, V]) Get(key K) (V, bool) {
	// lock
	// Full Lock, not RLock: Touch mutates the LRU list, so Get is a writer despite
	// reading. Sharding will REDUCE this contention (per-shard lock); dropping the
	// write lock entirely needs approximate LRU (timestamp+sampling), not shards.
	s.mu.Lock()
	defer s.mu.Unlock()

	// critical section
	e, exists := s.items[key]
	if !exists {
		var zero V
		return zero, false
	}
	s.policy.Touch(key) // bump recency
	return e.latest()
}

// Set appends a new version of key, then evicts until back within the entry-count
// budget. An existing key gains a version but is not a new entry, so no eviction.
func (s *store[K, V]) Set(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pushVersionLocked(key, value) {
		// brand-new key: register with the policy, then trim to the entry budget.
		s.policy.Add(key)
		s.evict()
	} else {
		// existing key, new version, same entry — just bump recency.
		s.policy.Touch(key)
	}
}

// pushVersionLocked appends value as a new version of key, trimming the chain to
// maxVersions by dropping the oldest. Reports whether key was newly created.
//
// The "Locked" suffix is the convention for helpers that assume s.mu is already
// held. The compiler cannot enforce it and RWMutex is not reentrant, so the name
// is the only warning the call site gets.
func (s *store[K, V]) pushVersionLocked(key K, value V) (isNew bool) {
	s.seq++

	currTime := time.Now().UnixNano()

	versionEntry := Version[V]{
		Value: value,
		Ts:    currTime,
		Seq:   s.seq,
	}

	// Map values are COPIES — vals is a local, not a reference into the map.
	// Nothing here sticks until the write-back at the end.
	vals, exists := s.items[key]
	vals.versions = append(vals.versions, versionEntry)

	for len(vals.versions) > s.maxVersions {
		n := copy(vals.versions, vals.versions[1:]) // shift everything down one
		clear(vals.versions[n:])                    // drop the duplicated tail ref
		vals.versions = vals.versions[:n]
	}

	s.items[key] = vals
	return exists == false // need to pass whether the key was new/old
}

// Delete removes key and all its versions if present.
func (s *store[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[key]; exists {
		delete(s.items, key)
		s.policy.Remove(key)
	}
}

// Len returns the number of entries.
func (s *store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// History returns key's kept versions, newest-first (nil if key is absent).
func (s *store[K, V]) History(key K) []Version[V] {
	s.mu.Lock() // Touch mutates recency → write lock, like Get
	defer s.mu.Unlock()

	e, exists := s.items[key]
	if !exists {
		return nil
	}
	s.policy.Touch(key)

	// FIX(3): `versions := e.versions` copies the slice HEADER, not the elements —
	// so slices.Reverse was reordering the store's own chain in place. Because
	// latest() reads the last element, one History call left Get returning the
	// OLDEST value, and a second History call undid the first. Clone, then
	// reverse the clone: the caller gets newest-first and can never touch our
	// state.
	out := slices.Clone(e.versions)
	slices.Reverse(out)
	return out
}

// evict removes victims until the entry-count budget is satisfied.
//
// Caller MUST hold s.mu (write lock). It does NOT lock itself: Set already holds
// the lock, and Go's RWMutex is not reentrant — re-locking here would deadlock
// the goroutine against itself. This is why locking lives in the public methods
// and helpers assume it.
func (s *store[K, V]) evict() {
	for s.overCapacity() {
		victim, ok := s.policy.Evict()
		if !ok {
			return // policy empty — nothing left to evict
		}
		delete(s.items, victim)
		if s.onEvict != nil {
			s.onEvict(victim)
		}
	}
}

// overCapacity reports whether the entry-count budget is exceeded.
func (s *store[K, V]) overCapacity() bool {
	return s.maxItems > 0 && len(s.items) > s.maxItems
}
