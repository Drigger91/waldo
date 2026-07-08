package waldo

import (
	"sync"

	"github.com/Drigger91/waldo/eviction"
)

// entry is what the store keeps per key. Currently just the value; a cached
// byte cost will return here when the byte budget is introduced
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
func (s *store[K, V]) Set(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[key]; exists {
		// add to the storage
		s.policy.Touch(key)
		s.items[key] = entry[V]{value}
		return
	}
	// new key: add first, then trim back to budget (add-then-evict keeps the
	// store at exactly maxItems, instead of floating at maxItems+1).
	s.items[key] = entry[V]{value}
	s.policy.Add(key)
	s.evict()
}

// Delete removes key if present.
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
