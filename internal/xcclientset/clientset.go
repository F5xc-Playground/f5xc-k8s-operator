package xcclientset

import (
	"sync"

	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient"
)

// ClientSet wraps an XCClient behind a sync.RWMutex so credentials can be
// rotated without restarting the operator.
type ClientSet struct {
	mu     sync.RWMutex
	client xcclient.XCClient
}

// New creates a ClientSet with the given initial client.
func New(client xcclient.XCClient) *ClientSet {
	return &ClientSet{client: client}
}

// Get returns the current XCClient under a read lock.
func (cs *ClientSet) Get() xcclient.XCClient {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.client
}

// Swap replaces the current XCClient under a write lock.
func (cs *ClientSet) Swap(client xcclient.XCClient) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.client = client
}
