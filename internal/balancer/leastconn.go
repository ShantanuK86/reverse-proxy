package balancer

import (
	"github.com/shantanuk/reverse-proxy/internal/backend"
)

// LeastConn selects the healthy backend with the fewest active connections.
type LeastConn struct{}

// NewLeastConn creates a new least-connections balancer.
func NewLeastConn() *LeastConn {
	return &LeastConn{}
}

// Next returns the healthy backend with the lowest active connection count.
// Ties are broken by selecting the first backend found with the minimum.
func (lc *LeastConn) Next(pool *backend.Pool) (*backend.Backend, error) {
	healthy := pool.Healthy()
	if len(healthy) == 0 {
		return nil, ErrNoBackends
	}

	var best *backend.Backend
	var bestConns int64 = -1

	for _, b := range healthy {
		conns := b.GetActiveConns()
		if bestConns == -1 || conns < bestConns {
			best = b
			bestConns = conns
		}
	}

	return best, nil
}
