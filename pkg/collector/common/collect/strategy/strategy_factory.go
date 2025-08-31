package strategy

import (
	"sync"
)

// AbstractCollect interface
type AbstractCollect interface {
	SupportProtocol() string
}

// strategy container
var (
	collectStrategy = make(map[string]AbstractCollect)
	mu              sync.RWMutex
)

// Register a collect strategy
// Example: registration in init() of each implementation file
//
//	func init() {
//	    Register(&XXXCollect{})
//	}
func Register(collect AbstractCollect) {

	mu.Lock()
	defer mu.Unlock()

	collectStrategy[collect.SupportProtocol()] = collect
}

// Invoke returns the collect strategy for a protocol
func Invoke(protocol string) AbstractCollect {

	mu.RLock()
	defer mu.RUnlock()

	return collectStrategy[protocol]
}
