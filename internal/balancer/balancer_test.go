package balancer

import (
	"testing"

	"github.com/shantanuk/reverse-proxy/internal/backend"
)

func TestRoundRobin_Distribution(t *testing.T) {
	pool, _ := backend.NewPool([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	})

	rr := NewRoundRobin()
	counts := make(map[string]int)

	for i := 0; i < 300; i++ {
		b, err := rr.Next(pool)
		if err != nil {
			t.Fatalf("Next() error: %v", err)
		}
		counts[b.URL.String()]++
	}

	// Each backend should get exactly 100 requests with round-robin
	for url, count := range counts {
		if count != 100 {
			t.Errorf("backend %s got %d requests, want 100", url, count)
		}
	}
}

func TestRoundRobin_SkipsDeadBackends(t *testing.T) {
	pool, _ := backend.NewPool([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	})

	// Kill the second backend
	all := pool.All()
	pool.MarkDead(all[1])

	rr := NewRoundRobin()
	for i := 0; i < 100; i++ {
		b, err := rr.Next(pool)
		if err != nil {
			t.Fatalf("Next() error: %v", err)
		}
		if b.URL.String() == "http://localhost:9002" {
			t.Fatal("dead backend was selected")
		}
	}
}

func TestRoundRobin_NoBackends(t *testing.T) {
	pool, _ := backend.NewPool([]string{"http://localhost:9001"})
	pool.MarkDead(pool.All()[0])

	rr := NewRoundRobin()
	_, err := rr.Next(pool)
	if err != ErrNoBackends {
		t.Errorf("err = %v, want ErrNoBackends", err)
	}
}

func TestLeastConn_PicksMinimum(t *testing.T) {
	pool, _ := backend.NewPool([]string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	})

	all := pool.All()
	// Simulate different connection loads
	all[0].IncrConns() // 1 conn
	all[0].IncrConns() // 2 conns
	all[1].IncrConns() // 1 conn
	// all[2] has 0 conns

	lc := NewLeastConn()
	b, err := lc.Next(pool)
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}
	if b.URL.String() != "http://localhost:9003" {
		t.Errorf("selected %s, want http://localhost:9003 (0 conns)", b.URL)
	}
}

func TestLeastConn_SkipsDeadBackends(t *testing.T) {
	pool, _ := backend.NewPool([]string{
		"http://localhost:9001",
		"http://localhost:9002",
	})

	all := pool.All()
	// Backend 1 has 0 conns but is dead
	pool.MarkDead(all[0])
	// Backend 2 has 5 conns but is alive
	for i := 0; i < 5; i++ {
		all[1].IncrConns()
	}

	lc := NewLeastConn()
	b, err := lc.Next(pool)
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}
	if b.URL.String() != "http://localhost:9002" {
		t.Errorf("selected %s, want http://localhost:9002", b.URL)
	}
}

func TestNew_ValidStrategies(t *testing.T) {
	tests := []struct {
		strategy string
		wantErr  bool
	}{
		{"round-robin", false},
		{"least-connections", false},
		{"random", true},
		{"", true},
	}

	for _, tt := range tests {
		_, err := New(tt.strategy)
		if (err != nil) != tt.wantErr {
			t.Errorf("New(%q) error = %v, wantErr = %v", tt.strategy, err, tt.wantErr)
		}
	}
}
