/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package dispatch

import (
	"fmt"
	"net/http"
	"time"

	// Import basic package with blank identifier to trigger its init() function
	// This ensures all collector factories are registered automatically
	_ "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/basic"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/collect/strategy"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// MetricsCollector handles metrics collection using goroutines and channels
type MetricsCollector struct {
	logger logger.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger logger.Logger) *MetricsCollector {
	return &MetricsCollector{
		logger: logger.WithName("metrics-collector"),
	}
}

// CollectMetrics collects metrics using the appropriate collector and returns results via channel
// This method uses Go's channel-based concurrency instead of Java's thread pools
func (mc *MetricsCollector) CollectMetrics(metrics *jobtypes.Metrics, job *jobtypes.Job, timeout *jobtypes.Timeout) chan *jobtypes.CollectRepMetricsData {
	resultChan := make(chan *jobtypes.CollectRepMetricsData, 1)

	// Start collection in a goroutine (Go's lightweight thread)
	go func() {
		defer close(resultChan)

		mc.logger.Info("starting metrics collection",
			"jobID", job.ID,
			"metricsName", metrics.Name,
			"protocol", metrics.Protocol)

		startTime := time.Now()

		// Get the appropriate collector based on protocol
		collector, err := strategy.CollectorFor(metrics.Protocol)
		if err != nil {
			mc.logger.Error(err, "failed to get collector",
				"protocol", metrics.Protocol,
				"metricsName", metrics.Name)

			result := mc.createErrorResponse(metrics, job, http.StatusInternalServerError, fmt.Sprintf("Collector not found: %v", err))
			resultChan <- result
			return
		}

		// Perform the actual collection
		result := collector.Collect(metrics)

		// Enrich result with job information
		if result != nil {
			result.ID = job.ID
			result.MonitorID = job.MonitorID
			result.App = job.App
			result.TenantID = job.TenantID

			// Add labels from job
			if result.Labels == nil {
				result.Labels = make(map[string]string)
			}
			for k, v := range job.Labels {
				result.Labels[k] = v
			}

			// Add metadata from job
			if result.Metadata == nil {
				result.Metadata = make(map[string]string)
			}
			for k, v := range job.Metadata {
				result.Metadata[k] = v
			}
		}

		duration := time.Since(startTime)

		if result != nil && result.Code == http.StatusOK {
			mc.logger.Info("metrics collection completed successfully",
				"jobID", job.ID,
				"metricsName", metrics.Name,
				"protocol", metrics.Protocol,
				"duration", duration,
				"valuesCount", len(result.Values))
		} else {
			mc.logger.Info("metrics collection failed",
				"jobID", job.ID,
				"metricsName", metrics.Name,
				"protocol", metrics.Protocol,
				"duration", duration,
				"code", result.Code,
				"message", result.Msg)
		}

		resultChan <- result
	}()

	return resultChan
}

// createErrorResponse creates an error response for failed collections
func (mc *MetricsCollector) createErrorResponse(metrics *jobtypes.Metrics, job *jobtypes.Job, code int, message string) *jobtypes.CollectRepMetricsData {
	return &jobtypes.CollectRepMetricsData{
		ID:        job.ID,
		MonitorID: job.MonitorID,
		TenantID:  job.TenantID,
		App:       job.App,
		Metrics:   metrics.Name,
		Priority:  metrics.Priority,
		Time:      time.Now().UnixMilli(),
		Code:      code,
		Msg:       message,
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}
}
