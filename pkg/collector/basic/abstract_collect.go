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

package basic

import (
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// AbstractCollector defines the interface for all collectors
type AbstractCollector interface {
	// PreCheck validates metrics configuration
	PreCheck(metrics *jobtypes.Metrics) error

	// Collect performs the actual metrics collection
	Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData

	// SupportProtocol returns the protocol this collector supports
	SupportProtocol() string
}

// BaseCollector provides common functionality for all collectors
type BaseCollector struct {
	Logger logger.Logger
}

// NewBaseCollector creates a new base collector
func NewBaseCollector(logger logger.Logger) *BaseCollector {
	return &BaseCollector{
		Logger: logger.WithName("base-collector"),
	}
}

// CreateSuccessResponse creates a successful metrics data response
func (bc *BaseCollector) CreateSuccessResponse(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	return &jobtypes.CollectRepMetricsData{
		ID:        0,  // Will be set by the calling context
		MonitorID: 0,  // Will be set by the calling context
		App:       "", // Will be set by the calling context
		Metrics:   metrics.Name,
		Priority:  0,
		Time:      getCurrentTimeMillis(),
		Code:      200, // Success
		Msg:       "success",
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}
}

// CreateFailResponse creates a failed metrics data response
func (bc *BaseCollector) CreateFailResponse(metrics *jobtypes.Metrics, code int, message string) *jobtypes.CollectRepMetricsData {
	return &jobtypes.CollectRepMetricsData{
		ID:        0,  // Will be set by the calling context
		MonitorID: 0,  // Will be set by the calling context
		App:       "", // Will be set by the calling context
		Metrics:   metrics.Name,
		Priority:  0,
		Time:      getCurrentTimeMillis(),
		Code:      code,
		Msg:       message,
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}
}

// CollectorRegistry manages registered collectors
type CollectorRegistry struct {
	collectors map[string]AbstractCollector
	logger     logger.Logger
}

// NewCollectorRegistry creates a new collector registry
func NewCollectorRegistry(logger logger.Logger) *CollectorRegistry {
	return &CollectorRegistry{
		collectors: make(map[string]AbstractCollector),
		logger:     logger.WithName("collector-registry"),
	}
}

// Register registers a collector for a specific protocol
func (cr *CollectorRegistry) Register(protocol string, collector AbstractCollector) {
	cr.collectors[protocol] = collector
	cr.logger.Info("registered collector", "protocol", protocol)
}

// GetCollector gets a collector for the specified protocol
func (cr *CollectorRegistry) GetCollector(protocol string) (AbstractCollector, bool) {
	collector, exists := cr.collectors[protocol]
	return collector, exists
}

// GetSupportedProtocols returns all supported protocols
func (cr *CollectorRegistry) GetSupportedProtocols() []string {
	protocols := make([]string, 0, len(cr.collectors))
	for protocol := range cr.collectors {
		protocols = append(protocols, protocol)
	}
	return protocols
}

// getCurrentTimeMillis returns current time in milliseconds
func getCurrentTimeMillis() int64 {
	return time.Now().UnixMilli()
}
