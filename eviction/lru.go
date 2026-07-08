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
	nodes map[K]*list.Element // key - its node in ll
}

// NewLRU returns an empty LRU policy.
func NewLRU[K comparable]() *LRU[K] {
	return &LRU[K]{
		ll:    list.New(),
		nodes: make(map[K]*list.Element),
	}
}

// Add records key as most-recently-used.
func (p *LRU[K]) Add(key K) {
	el := p.ll.PushFront(key) // most-recently-used goes to the front
	p.nodes[key] = el
}

// Len reports the number of tracked keys.
func (p *LRU[K]) Len() int {
	return p.ll.Len()
}

// Touch moves an existing key to the front (most-recently-used).
func (p *LRU[K]) Touch(key K) {
	if el, ok := p.nodes[key]; ok {
		p.ll.MoveToFront(el)
	}
}

// Remove drops a key from the policy (explicit delete).
func (p *LRU[K]) Remove(key K) {
	if el, ok := p.nodes[key]; ok {
		p.ll.Remove(el)
		delete(p.nodes, key)
	}
}

// Evict removes and returns the least-recently-used key (the back of the list).
func (p *LRU[K]) Evict() (K, bool) {
	back := p.ll.Back()
	if back == nil {
		var zero K
		return zero, false
	}
	key := back.Value.(K)
	p.ll.Remove(back)
	delete(p.nodes, key)
	return key, true
}
