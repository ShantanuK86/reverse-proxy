package balancer

import (
	"sync/atomic"

	"github.com/shantanuk/reverse-proxy/internal/backend"
)

// RoundRobin distributes requests evenly across healthy backends.
type RoundRobin struct {
	counter atomic.Uint64
}

// NewRoundRobin creates a new round-robin balancer.
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

// Next returns the next healthy backend using round-robin selection.
func (rr *RoundRobin) Next(pool *backend.Pool) (*backend.Backend, error) {
	healthy := pool.Healthy()
	if len(healthy) == 0 {
		return nil, ErrNoBackends
	}

	idx := rr.counter.Add(1) - 1
	return healthy[idx%uint64(len(healthy))], nil
}
