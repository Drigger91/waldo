package eviction

import "container/list"

// LRU is the classic least-recently-used policy: a doubly-linked list ordered
// most-recently-used (front) → least-recently-used (back), plus a map from key
// to its list node for O(1) Touch/Remove.
//
// The list node's Value holds the key K. Because container/list stores `any`,
// reading a key back requires a type assertion `el.Value.(K)`.
//
// This is the EXACT-LRU baseline. Later we'll benchmark it against a
// timestamp+sampling variant (see the whiteboard journal) — so keep it honest and simple.
type LRU[K comparable] struct {
	ll    *list.List          // front = most recently used, back = least
	nodes map[K]*list.Element // key → its node in ll
}

// NewLRU returns an empty LRU policy.
func NewLRU[K comparable]() *LRU[K] {
	return &LRU[K]{
		ll:    list.New(),
		nodes: make(map[K]*list.Element),
	}
}

// Add records key as most-recently-used.
//
// Worked example so you can see the container/list + map dance; the rest are
// yours to fill in following this shape.
func (p *LRU[K]) Add(key K) {
	el := p.ll.PushFront(key) // most-recently-used goes to the front
	p.nodes[key] = el
}

// Len reports the number of tracked keys.
func (p *LRU[K]) Len() int {
	return p.ll.Len()
}

// Touch moves an existing key to the front (most-recently-used).
//
// TODO(you): look the node up in p.nodes and move it to the front of p.ll.
// Hint: container/list has exactly the method you want for "move this element
// to the front" — no need to remove + re-add.
func (p *LRU[K]) Touch(key K) {
	panic("TODO: implement LRU.Touch")
}

// Remove drops a key from the policy (explicit delete).
//
// TODO(you): find the node, remove it from p.ll, and delete it from p.nodes.
// Keep the two structures in lockstep — a node in the list but not the map (or
// vice-versa) is the classic LRU bug the race detector won't catch for you.
func (p *LRU[K]) Remove(key K) {
	panic("TODO: implement LRU.Remove")
}

// Evict removes and returns the least-recently-used key (the back of the list).
//
// TODO(you): grab p.ll.Back(); if it's nil the policy is empty (return zero, false).
// Otherwise recover the key with a type assertion on el.Value, remove it from
// BOTH p.ll and p.nodes, and return (key, true).
func (p *LRU[K]) Evict() (K, bool) {
	panic("TODO: implement LRU.Evict")
}
