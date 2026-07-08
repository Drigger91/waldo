package waldo

import (
	"sync"

	"github.com/Drigger91/waldo/eviction"
)

// entry is what the store keeps per key. Currently just the value; a cached
// byte cost will return here when the byte budget comes back.
type entry[V any] struct {
	value V
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

	// config (immutable after New)
	maxItems int
	onEvict  func(key K)
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
	entry, exists := s.items[key]
	if exists {
		// bump up
		s.policy.Touch(key)
	}
	return entry.value, exists
}

// Set inserts or updates key, then evicts until back within the entry-count budget.
//
// TODO(you):
//  1. Lock. If key already exists: overwrite items[key] and s.policy.Touch(key).
//     If it's new: write items[key] and s.policy.Add(key).
//  2. Call s.evictToFit() (below) to bring the store back within budget.
func (s *store[K, V]) Set(key K, value V) {
	panic("TODO: implement Set")
}

// Delete removes key if present.
//
// TODO(you): Lock. If items[key] exists, delete it and s.policy.Remove(key).
func (s *store[K, V]) Delete(key K) {
	panic("TODO: implement Delete")
}

// Len returns the number of entries.
func (s *store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// evictToFit evicts victims until the entry-count budget is satisfied. Caller
// MUST hold s.mu (write lock).
//
// TODO(you): loop while over budget:
//
//	for s.overCapacity() {
//	    victim, ok := s.policy.Evict()
//	    if !ok { break } // policy empty — can't evict further
//	    delete(s.items, victim)
//	    if s.onEvict != nil { s.onEvict(victim) }
//	}
func (s *store[K, V]) evictToFit() {
	panic("TODO: implement evictToFit")
}

// overCapacity reports whether the entry-count budget is exceeded.
func (s *store[K, V]) overCapacity() bool {
	return s.maxItems > 0 && len(s.items) > s.maxItems
}
