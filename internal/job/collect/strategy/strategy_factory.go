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

package strategy

import (
	"fmt"
	"sync"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// Collector interface defines the contract for all collectors
type Collector interface {
	// Collect performs the actual metrics collection
	Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData

	// Protocol returns the protocol this collector supports
	Protocol() string
}

// CollectorFactory is a function that creates a collector with a logger
type CollectorFactory func(logger.Logger) Collector

// CollectorRegistry manages all registered collector factories
type CollectorRegistry struct {
	factories   map[string]CollectorFactory
	collectors  map[string]Collector
	mu          sync.RWMutex
	initialized bool
}

// Global collector registry instance
var globalRegistry = &CollectorRegistry{
	factories:  make(map[string]CollectorFactory),
	collectors: make(map[string]Collector),
}

// RegisterFactory registers a collector factory function
// This is called during init() to register collector factories
func RegisterFactory(protocol string, factory CollectorFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.factories[protocol] = factory
}

// InitializeCollectors creates actual collector instances using the registered factories
// This should be called once when logger is available
func InitializeCollectors(logger logger.Logger) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if globalRegistry.initialized {
		return // Already initialized
	}

	for protocol, factory := range globalRegistry.factories {
		collector := factory(logger)
		globalRegistry.collectors[protocol] = collector
	}

	globalRegistry.initialized = true
}

// Register a collector directly (legacy method, kept for compatibility)
func Register(collector Collector) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	protocol := collector.Protocol()
	globalRegistry.collectors[protocol] = collector
}

// CollectorFor returns the collector for a protocol
func CollectorFor(protocol string) (Collector, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	collector, exists := globalRegistry.collectors[protocol]
	if !exists {
		return nil, fmt.Errorf("no collector found for protocol: %s", protocol)
	}
	return collector, nil
}

// SupportedProtocols returns all supported protocols
func SupportedProtocols() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	protocols := make([]string, 0, len(globalRegistry.collectors))
	for protocol := range globalRegistry.collectors {
		protocols = append(protocols, protocol)
	}
	return protocols
}

// CollectorCount returns the number of registered collectors
func CollectorCount() int {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	return len(globalRegistry.collectors)
}
