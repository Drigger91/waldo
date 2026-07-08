// Package eviction defines the pluggable victim-selection policy for a waldo
// store, plus the built-in policies (LRU to start; FIFO/LFU/TTL later).
//
// Design (see the whiteboard journal): a Policy is purely about ORDERING — "given that
// something must go, which key?" It deliberately does NOT know about values or
// bytes. Byte/entry accounting and the decision to evict live in the store; the
// policy just tracks recency/insertion order over keys. That keeps the interface
// tiny and V-agnostic, and makes LRU/FIFO/LFU swappable without touching storage.
//
// (Size-aware policies — evict-the-biggest, cost/benefit — would need a per-key
// cost threaded in here later. Out of scope for Phase 1; noted so we remember.)
//
// Thread-safety: implementations are NOT expected to be safe for concurrent use.
// The store owns a lock and serializes every Policy call under it. Keeping the
// policy single-threaded is intentional — the concurrency lesson lives in the
// store, not scattered across every policy.
package eviction

// Policy decides which key to evict when the store is over budget.
//
// The store guarantees the following call discipline:
//   - Add(k)    is called exactly once when a brand-new key is inserted.
//   - Touch(k)  is called on access (Get) and on updating an existing key.
//   - Remove(k) is called on explicit Delete.
//   - Evict()   is called repeatedly while the store is over budget; it returns
//               the next victim (the store then deletes it from storage).
//
// A key passed to Touch/Remove is always one previously Add-ed and not yet
// evicted/removed, so implementations may assume it is present.
type Policy[K comparable] interface {
	// Add records a newly inserted key as most-recently-used.
	Add(key K)

	// Touch marks an existing key as most-recently-used.
	Touch(key K)

	// Remove drops a key from the policy's bookkeeping (explicit delete).
	Remove(key K)

	// Evict returns the next key that should be evicted and removes it from the
	// policy's bookkeeping. ok is false when the policy is empty.
	Evict() (key K, ok bool)

	// Len reports how many keys the policy is currently tracking. The store uses
	// this only in tests/asserts; it should equal the store's own entry count.
	Len() int
}
