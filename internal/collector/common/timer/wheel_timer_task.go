/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package timer

import (
	"encoding/json"
	"fmt"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// MetricsTaskDispatcher defines the interface for dispatching metrics collection tasks
type MetricsTaskDispatcher interface {
	DispatchMetricsTask(timeout *job.Timeout) error
}

// WheelTimerTask represents a task that wraps a job for execution in the timer wheel
type WheelTimerTask struct {
	job        *job.Job
	dispatcher MetricsTaskDispatcher
	logger     logger.Logger
}

// NewWheelTimerTask creates a new wheel timer task
func NewWheelTimerTask(job *job.Job, dispatcher MetricsTaskDispatcher, logger logger.Logger) *WheelTimerTask {
	task := &WheelTimerTask{
		job:        job.Clone(), // Clone to avoid data races
		dispatcher: dispatcher,
		logger:     logger.WithName("wheel-timer-task"),
	}

	// Initialize job metrics configuration
	task.initJobMetrics()

	return task
}

// Run executes the timer task
func (wtt *WheelTimerTask) Run(timeout *job.Timeout) error {
	if wtt.job == nil {
		return fmt.Errorf("job is nil")
	}

	// Set dispatch time
	wtt.job.DispatchTime = time.Now().UnixMilli()

	wtt.logger.Info("executing wheel timer task",
		"jobId", wtt.job.ID,
		"monitorId", wtt.job.MonitorID,
		"app", wtt.job.App,
		"dispatchTime", wtt.job.DispatchTime)

	// Dispatch the metrics collection task
	if wtt.dispatcher != nil {
		return wtt.dispatcher.DispatchMetricsTask(timeout)
	}

	return fmt.Errorf("metrics task dispatcher is not set")
}

// GetJob returns the job associated with this task
func (wtt *WheelTimerTask) GetJob() *job.Job {
	return wtt.job
}

// initJobMetrics initializes job metrics with parameter substitution
func (wtt *WheelTimerTask) initJobMetrics() {
	if wtt.job == nil {
		return
	}

	// Build configmap for parameter substitution
	configMap := make(map[string]*job.Configmap)
	for i := range wtt.job.Configmap {
		configMap[wtt.job.Configmap[i].Key] = &wtt.job.Configmap[i]

		// Decode password fields (simplified - in real implementation, use proper decryption)
		if wtt.job.Configmap[i].Type == 1 && wtt.job.Configmap[i].Value != nil { // PASSWORD type
			if strValue, ok := wtt.job.Configmap[i].Value.(string); ok {
				// TODO: Implement proper AES decoding here
				wtt.job.Configmap[i].Value = strValue
			}
		}

		// Trim string values
		if strValue, ok := wtt.job.Configmap[i].Value.(string); ok {
			wtt.job.Configmap[i].Value = strValue
		}
	}

	// Process metrics with parameter substitution
	processedMetrics := make([]job.Metrics, 0, len(wtt.job.Metrics))

	for _, metric := range wtt.job.Metrics {
		// Serialize to JSON for placeholder replacement
		metricJSON, err := json.Marshal(metric)
		if err != nil {
			wtt.logger.Error(err, "failed to marshal metric", "metric", metric.Name)
			continue
		}

		// Replace placeholders (simplified implementation)
		processedJSON := wtt.replacePlaceholders(string(metricJSON), configMap)

		// Deserialize back to metric
		var processedMetric job.Metrics
		if err := json.Unmarshal([]byte(processedJSON), &processedMetric); err != nil {
			wtt.logger.Error(err, "failed to unmarshal processed metric", "metric", metric.Name)
			continue
		}

		// Special handling for push-style monitors
		if wtt.job.App == "push" {
			wtt.replaceFieldsForPushStyleMonitor(&processedMetric, configMap)
		}

		processedMetrics = append(processedMetrics, processedMetric)
	}

	wtt.job.Metrics = processedMetrics
	wtt.initIntervals()
}

// replacePlaceholders replaces ${param} placeholders in the JSON string
func (wtt *WheelTimerTask) replacePlaceholders(jsonStr string, configMap map[string]*job.Configmap) string {
	// This is a simplified implementation
	// In the real implementation, you would use a proper template engine
	// or regex-based replacement similar to the Java version

	result := jsonStr
	for key, config := range configMap {
		placeholder := fmt.Sprintf("${%s}", key)
		if config.Value != nil {
			if strValue, ok := config.Value.(string); ok {
				result = replaceAll(result, placeholder, strValue)
			}
		}
	}

	return result
}

// replaceFieldsForPushStyleMonitor handles special field replacement for push-style monitors
func (wtt *WheelTimerTask) replaceFieldsForPushStyleMonitor(metric *job.Metrics, configMap map[string]*job.Configmap) {
	// Implementation for push-style monitor field replacement
	// This would involve updating metric fields based on configMap values
	wtt.logger.Info("processing push-style monitor fields", "metric", metric.Name)
}

// initIntervals initializes the intervals for the job
func (wtt *WheelTimerTask) initIntervals() {
	if wtt.job == nil {
		return
	}

	// Initialize intervals based on metric priorities and configurations
	intervalSet := make(map[int64]bool)

	// Add default interval
	if wtt.job.DefaultInterval > 0 {
		intervalSet[wtt.job.DefaultInterval] = true
	}

	// Add metric-specific intervals
	for _, metric := range wtt.job.Metrics {
		if metric.Interval > 0 {
			intervalSet[metric.Interval] = true
		}
	}

	// Convert to sorted slice
	intervals := make([]int64, 0, len(intervalSet))
	for interval := range intervalSet {
		intervals = append(intervals, interval)
	}

	// Sort intervals (simple bubble sort for small arrays)
	for i := 0; i < len(intervals); i++ {
		for j := i + 1; j < len(intervals); j++ {
			if intervals[i] > intervals[j] {
				intervals[i], intervals[j] = intervals[j], intervals[i]
			}
		}
	}

	wtt.job.Intervals = intervals

	wtt.logger.Info("initialized job intervals",
		"jobId", wtt.job.ID,
		"intervals", intervals)
}

// Helper function to replace all occurrences of old with new in str
func replaceAll(str, old, new string) string {
	result := ""
	for i := 0; i < len(str); {
		if i+len(old) <= len(str) && str[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(str[i])
			i++
		}
	}
	return result
}
