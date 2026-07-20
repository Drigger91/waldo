package waldo_test

import (
	"sync"
	"testing"

	"github.com/Drigger91/waldo"
)

// --- basic behaviour -------------------------------------------------------

func TestStore_BasicOps(t *testing.T) {
	s := waldo.New[string, int](waldo.Options[string, int]{})

	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get on empty store should report ok=false")
	}

	s.Set("a", 1)
	s.Set("b", 2)
	if got, ok := s.Get("a"); !ok || got != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", got, ok)
	}
	if s.Len() != 2 {
		t.Fatalf("Len() = %d; want 2", s.Len())
	}

	s.Set("a", 10) // update
	if got, _ := s.Get("a"); got != 10 {
		t.Fatalf("Get(a) after update = %d; want 10", got)
	}
	if s.Len() != 2 {
		t.Fatalf("Len() after update = %d; want 2 (update, not insert)", s.Len())
	}

	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("Get(a) after Delete should report ok=false")
	}
	if s.Len() != 1 {
		t.Fatalf("Len() after delete = %d; want 1", s.Len())
	}
}

// --- eviction by entry count ----------------------------------------------

func TestStore_EvictByCount(t *testing.T) {
	s := waldo.New[int, int](waldo.Options[int, int]{MaxEntries: 3})

	for i := 0; i < 3; i++ {
		s.Set(i, i)
	}
	// Access 0 so it becomes most-recently-used; under LRU the next insert
	// should then evict key 1 (the least-recently-used), not key 0.
	s.Get(0)
	s.Set(3, 3)

	if s.Len() != 3 {
		t.Fatalf("Len() = %d; want 3 (capped at MaxEntries)", s.Len())
	}
	if _, ok := s.Get(1); ok {
		t.Error("key 1 should have been evicted as least-recently-used")
	}
	if _, ok := s.Get(0); !ok {
		t.Error("key 0 was touched and should have survived")
	}
}

// --- versioning ------------------------------------------------------------

func TestStore_Versioning(t *testing.T) {
	s := waldo.New[string, int](waldo.Options[string, int]{MaxVersions: 3})

	s.Set("k", 1)
	s.Set("k", 2)
	s.Set("k", 3)

	if v, ok := s.Get("k"); !ok || v != 3 {
		t.Fatalf("Get(k) = %v, %v; want 3, true (latest)", v, ok)
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d; want 1 (one key, three versions)", s.Len())
	}

	h := s.History("k")
	if len(h) != 3 {
		t.Fatalf("len(History) = %d; want 3", len(h))
	}
	if h[0].Value != 3 || h[1].Value != 2 || h[2].Value != 1 {
		t.Errorf("History values = %d,%d,%d; want 3,2,1 (newest-first)", h[0].Value, h[1].Value, h[2].Value)
	}
	if !(h[0].Seq > h[1].Seq && h[1].Seq > h[2].Seq) {
		t.Errorf("History seqs = %d,%d,%d; want strictly descending (newest-first)", h[0].Seq, h[1].Seq, h[2].Seq)
	}

	// A 4th write drops the oldest (value 1).
	s.Set("k", 4)
	h = s.History("k")
	if len(h) != 3 {
		t.Fatalf("after 4th Set: len(History) = %d; want 3 (bounded)", len(h))
	}
	if h[0].Value != 4 || h[2].Value != 2 {
		t.Errorf("after 4th Set: newest=%d oldest=%d; want 4 and 2", h[0].Value, h[2].Value)
	}

	// MaxVersions: 1 is the plain latest-only cache — no history kept.
	one := waldo.New[string, int](waldo.Options[string, int]{MaxVersions: 1})
	one.Set("x", 10)
	one.Set("x", 20)
	oh := one.History("x")
	if len(oh) != 1 {
		t.Fatalf("MaxVersions=1: len(History) = %d; want 1 (latest only)", len(oh))
	}
	if oh[0].Value != 20 {
		t.Errorf("MaxVersions=1: newest = %d; want 20", oh[0].Value)
	}

	// An unset MaxVersions falls back to DefaultMaxVersions (5), not to 1:
	// waldo is a versioned store by default. There is deliberately no upper
	// clamp — a fixed ceiling is exactly what MVCC (04) would have to remove.
	d := waldo.New[string, int](waldo.Options[string, int]{})
	for i := 1; i <= 7; i++ {
		d.Set("x", i)
	}
	dh := d.History("x")
	if len(dh) != waldo.DefaultMaxVersions {
		t.Fatalf("default MaxVersions: len(History) = %d; want %d", len(dh), waldo.DefaultMaxVersions)
	}
	if dh[0].Value != 7 || dh[len(dh)-1].Value != 3 {
		t.Errorf("default MaxVersions: newest=%d oldest=%d; want 7 and 3", dh[0].Value, dh[len(dh)-1].Value)
	}

	// Delete drops the whole chain.
	s.Delete("k")
	if h := s.History("k"); h != nil {
		t.Errorf("History after Delete = %v; want nil", h)
	}
}

// --- concurrency (run with -race) -----------------------------------------

// TestStore_ConcurrentAccess is the one that justifies the whole Phase 1 design.
// It MUST be run with the race detector: `go test -race`.
func TestStore_ConcurrentAccess(t *testing.T) {
	s := waldo.New[int, int](waldo.Options[int, int]{MaxEntries: 128})

	const goroutines = 16
	const opsPer = 2000

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < opsPer; i++ {
				k := (g*opsPer + i) % 256
				switch i % 4 {
				case 0, 1:
					s.Set(k, i)
				case 2:
					s.Get(k)
				case 3:
					s.Delete(k)
				}
			}
		}(g)
	}
	wg.Wait()
	// No assertion on contents — the point is that -race stays silent and the
	// LRU list never corrupts under concurrent Touch/Add/Remove/Evict.
}

// --- benchmarks (the baseline before sharding) ----------------------------

func BenchmarkStore_Set(b *testing.B) {
	s := waldo.New[int, int](waldo.Options[int, int]{MaxEntries: 1024})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(i&1023, i)
	}
}

// BenchmarkStore_ParallelGet is the number to remember. Under the single-RWMutex
// baseline with a full Lock in Get, this should scale POORLY with -cpu — that
// flat/declining curve is exactly the contention sharding will fix. Capture it:
//
//	go test -bench=ParallelGet -cpu=1,2,4,8 -benchmem
func BenchmarkStore_ParallelGet(b *testing.B) {
	s := waldo.New[int, int](waldo.Options[int, int]{MaxEntries: 4096})
	for i := 0; i < 4096; i++ {
		s.Set(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Get(i & 4095)
			i++
		}
	})
}
