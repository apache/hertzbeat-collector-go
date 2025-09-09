package transport

import (
	"sync"
)

// ProcessorFunc defines the function signature for message processors
type ProcessorFunc func(msg interface{}) (interface{}, error)

// ProcessorRegistry manages message processors
type ProcessorRegistry struct {
	processors map[int32]ProcessorFunc
	mu         sync.RWMutex
}

// NewProcessorRegistry creates a new processor registry
func NewProcessorRegistry() *ProcessorRegistry {
	return &ProcessorRegistry{
		processors: make(map[int32]ProcessorFunc),
	}
}

// Register registers a processor for a specific message type
func (r *ProcessorRegistry) Register(msgType int32, processor ProcessorFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[msgType] = processor
}

// Get retrieves a processor for a specific message type
func (r *ProcessorRegistry) Get(msgType int32) (ProcessorFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	processor, exists := r.processors[msgType]
	return processor, exists
}

// TransportClient defines the interface for transport clients
type TransportClient interface {
	Start() error
	Shutdown() error
	IsStarted() bool
	SendMsg(msg interface{}) error
	SendMsgSync(msg interface{}, timeoutMillis int) (interface{}, error)
	RegisterProcessor(msgType int32, processor ProcessorFunc)
}