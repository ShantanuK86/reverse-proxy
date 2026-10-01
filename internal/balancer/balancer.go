package balancer

import (
	"errors"

	"github.com/shantanuk/reverse-proxy/internal/backend"
)

// ErrNoBackends is returned when no healthy backends are available.
var ErrNoBackends = errors.New("no healthy backends available")

// Balancer selects a backend from the pool for each request.
type Balancer interface {
	Next(pool *backend.Pool) (*backend.Backend, error)
}

// New creates a Balancer for the given strategy name.
func New(strategy string) (Balancer, error) {
	switch strategy {
	case "round-robin":
		return NewRoundRobin(), nil
	case "least-connections":
		return NewLeastConn(), nil
	default:
		return nil, errors.New("unsupported balancer strategy: " + strategy)
	}
}
