package backend

import (
	"net/url"
	"sync"
	"sync/atomic"
)

// Backend represents a single upstream server.
type Backend struct {
	URL         *url.URL
	alive       atomic.Bool
	ActiveConns atomic.Int64
	mu          sync.RWMutex

	// Health check state (managed by health.Checker).
	ConsecutiveFails    int
	ConsecutiveSucceses int
}

// NewBackend creates a backend from a raw URL string.
func NewBackend(rawURL string) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	b := &Backend{URL: u}
	b.alive.Store(true)
	return b, nil
}

// IsAlive reports whether the backend is considered healthy.
func (b *Backend) IsAlive() bool {
	return b.alive.Load()
}

// SetAlive sets the backend's health status.
func (b *Backend) SetAlive(alive bool) {
	b.alive.Store(alive)
}

// IncrConns atomically increments the active connection count.
func (b *Backend) IncrConns() {
	b.ActiveConns.Add(1)
}

// DecrConns atomically decrements the active connection count.
func (b *Backend) DecrConns() {
	b.ActiveConns.Add(-1)
}

// GetActiveConns returns the current active connection count.
func (b *Backend) GetActiveConns() int64 {
	return b.ActiveConns.Load()
}

// Pool is a thread-safe collection of backends.
type Pool struct {
	backends []*Backend
	mu       sync.RWMutex
}

// NewPool creates a pool from a list of backend URL strings.
func NewPool(urls []string) (*Pool, error) {
	backends := make([]*Backend, 0, len(urls))
	for _, rawURL := range urls {
		b, err := NewBackend(rawURL)
		if err != nil {
			return nil, err
		}
		backends = append(backends, b)
	}
	return &Pool{backends: backends}, nil
}

// All returns all backends in the pool.
func (p *Pool) All() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*Backend, len(p.backends))
	copy(result, p.backends)
	return result
}

// Healthy returns only backends that are currently alive.
func (p *Pool) Healthy() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var healthy []*Backend
	for _, b := range p.backends {
		if b.IsAlive() {
			healthy = append(healthy, b)
		}
	}
	return healthy
}

// MarkDead marks a backend as unhealthy.
func (p *Pool) MarkDead(b *Backend) {
	b.SetAlive(false)
}

// MarkAlive marks a backend as healthy.
func (p *Pool) MarkAlive(b *Backend) {
	b.SetAlive(true)
}

// Len returns the total number of backends.
func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.backends)
}
