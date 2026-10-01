package backend

import (
	"sync"
	"testing"
)

func TestNewPool(t *testing.T) {
	urls := []string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	}
	pool, err := NewPool(urls)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	if pool.Len() != 3 {
		t.Errorf("Len() = %d, want 3", pool.Len())
	}
}

func TestHealthyFiltering(t *testing.T) {
	pool, _ := NewPool([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	})

	all := pool.All()
	pool.MarkDead(all[1])

	healthy := pool.Healthy()
	if len(healthy) != 2 {
		t.Errorf("Healthy() = %d backends, want 2", len(healthy))
	}

	// Verify the dead one is not in healthy
	for _, b := range healthy {
		if b.URL.String() == "http://localhost:9002" {
			t.Error("dead backend found in healthy list")
		}
	}
}

func TestMarkAliveRecovery(t *testing.T) {
	pool, _ := NewPool([]string{"http://localhost:9001"})
	b := pool.All()[0]

	pool.MarkDead(b)
	if b.IsAlive() {
		t.Error("backend should be dead")
	}

	pool.MarkAlive(b)
	if !b.IsAlive() {
		t.Error("backend should be alive after recovery")
	}
}

func TestConcurrentAccess(t *testing.T) {
	pool, _ := NewPool([]string{
		"http://localhost:9001",
		"http://localhost:9002",
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			all := pool.All()
			for _, b := range all {
				b.IncrConns()
				_ = b.GetActiveConns()
				b.DecrConns()
			}
			_ = pool.Healthy()
		}()
	}
	wg.Wait()

	for _, b := range pool.All() {
		if conns := b.GetActiveConns(); conns != 0 {
			t.Errorf("backend %s has %d active conns after concurrent test, want 0",
				b.URL, conns)
		}
	}
}

func TestActiveConns(t *testing.T) {
	b, _ := NewBackend("http://localhost:9001")
	b.IncrConns()
	b.IncrConns()
	b.IncrConns()
	if got := b.GetActiveConns(); got != 3 {
		t.Errorf("ActiveConns = %d, want 3", got)
	}
	b.DecrConns()
	if got := b.GetActiveConns(); got != 2 {
		t.Errorf("ActiveConns = %d, want 2", got)
	}
}
