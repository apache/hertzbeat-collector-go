// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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