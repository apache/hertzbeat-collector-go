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

package worker

import (
	"fmt"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// MetricsData represents collected metrics data
type MetricsData struct {
	ID       int64             `json:"id"`
	TenantID int64             `json:"tenantId"`
	App      string            `json:"app"`
	Metrics  string            `json:"metrics"`
	Priority int               `json:"priority"`
	Time     int64             `json:"time"`
	Code     int               `json:"code"`
	Msg      string            `json:"msg"`
	Fields   []job.Field       `json:"fields"`
	Values   [][]string        `json:"values"`
	Metadata map[string]string `json:"metadata"`
	Labels   map[string]string `json:"labels"`
}

// CollectDataDispatcher defines the interface for dispatching collected data
type CollectDataDispatcher interface {
	DispatchCollectData(timeout *job.Timeout, metrics *job.Metrics, data []*MetricsData) error
}

// CollectService defines the interface for metrics collection service
type CollectService interface {
	Collect(metrics *job.Metrics) *job.CollectRepMetricsData
}

// MetricsDataBuilder helps build metrics data
type MetricsDataBuilder struct {
	data *MetricsData
}

// NewMetricsDataBuilder creates a new metrics data builder
func NewMetricsDataBuilder() *MetricsDataBuilder {
	return &MetricsDataBuilder{
		data: &MetricsData{
			Time:     time.Now().UnixMilli(),
			Fields:   make([]job.Field, 0),
			Values:   make([][]string, 0),
			Metadata: make(map[string]string),
			Labels:   make(map[string]string),
		},
	}
}

// SetApp sets the app name
func (b *MetricsDataBuilder) SetApp(app string) *MetricsDataBuilder {
	b.data.App = app
	return b
}

// SetID sets the monitor ID
func (b *MetricsDataBuilder) SetID(id int64) *MetricsDataBuilder {
	b.data.ID = id
	return b
}

// SetTenantID sets the tenant ID
func (b *MetricsDataBuilder) SetTenantID(tenantID int64) *MetricsDataBuilder {
	b.data.TenantID = tenantID
	return b
}

// SetMetrics sets the metrics name
func (b *MetricsDataBuilder) SetMetrics(metrics string) *MetricsDataBuilder {
	b.data.Metrics = metrics
	return b
}

// SetCode sets the result code
func (b *MetricsDataBuilder) SetCode(code int) *MetricsDataBuilder {
	b.data.Code = code
	return b
}

// SetMsg sets the result message
func (b *MetricsDataBuilder) SetMsg(msg string) *MetricsDataBuilder {
	b.data.Msg = msg
	return b
}

// SetFields sets the field definitions
func (b *MetricsDataBuilder) SetFields(fields []job.Field) *MetricsDataBuilder {
	b.data.Fields = fields
	return b
}

// AddValue adds a row of values
func (b *MetricsDataBuilder) AddValue(values []string) *MetricsDataBuilder {
	b.data.Values = append(b.data.Values, values)
	return b
}

// SetMetadata sets the metadata
func (b *MetricsDataBuilder) SetMetadata(metadata map[string]string) *MetricsDataBuilder {
	if metadata != nil {
		for k, v := range metadata {
			b.data.Metadata[k] = v
		}
	}
	return b
}

// SetLabels sets the labels
func (b *MetricsDataBuilder) SetLabels(labels map[string]string) *MetricsDataBuilder {
	if labels != nil {
		for k, v := range labels {
			b.data.Labels[k] = v
		}
	}
	return b
}

// Build returns the built metrics data
func (b *MetricsDataBuilder) Build() *MetricsData {
	return b.data
}

// MetricsCollect represents a collection task
type MetricsCollect struct {
	// Basic info
	collectorIdentity string
	tenantID          int64
	id                int64
	app               string
	metrics           *job.Metrics
	metadata          map[string]string
	labels            map[string]string
	annotations       map[string]string

	// Task control
	timeout           *job.Timeout
	collectDispatcher CollectDataDispatcher
	priority          int
	isCyclic          bool
	isSd              bool
	prometheusProxy   bool

	// Timing
	newTime   time.Time
	startTime time.Time

	// Dependencies
	collectService CollectService
	logger         logger.Logger
}

// NewMetricsCollect creates a new metrics collection task
func NewMetricsCollect(
	metrics *job.Metrics,
	timeout *job.Timeout,
	dispatcher CollectDataDispatcher,
	collectorIdentity string,
	collectService CollectService,
	logger logger.Logger) *MetricsCollect {

	// Extract job information from timeout if available
	if timeout != nil {
		task := timeout.Task()
		if task == nil {
			logger.Error(fmt.Errorf("timeout task is nil"), "failed to extract job from timeout")
			return nil
		}
	}

	// For now, we'll create a basic task structure
	// In a full implementation, you'd extract this from the WheelTimerTask
	mc := &MetricsCollect{
		collectorIdentity: collectorIdentity,
		metrics:           metrics,
		timeout:           timeout,
		collectDispatcher: dispatcher,
		collectService:    collectService,
		logger:            logger.WithName("metrics-collect"),
		newTime:           time.Now(),
		metadata:          make(map[string]string),
		labels:            make(map[string]string),
		annotations:       make(map[string]string),
	}

	// Set default values
	mc.tenantID = 1 // default tenant
	mc.id = 1       // default monitor id
	mc.app = "test" // default app

	// Set priority: one-time tasks have higher priority
	if mc.isCyclic {
		mc.priority = -1 // lower priority for periodic tasks
	} else {
		mc.priority = 1 // higher priority for one-time tasks
	}

	return mc
}

// Execute implements the Task interface
func (mc *MetricsCollect) Execute() error {
	mc.startTime = time.Now()

	mc.logger.Info("starting metrics collection",
		"id", mc.id,
		"app", mc.app,
		"metrics", mc.metrics.Name,
		"protocol", mc.metrics.Protocol)

	// Create response builder
	builder := NewMetricsDataBuilder()
	builder.SetApp(mc.app).
		SetID(mc.id).
		SetTenantID(mc.tenantID).
		SetMetrics(mc.metrics.Name).
		SetMetadata(mc.metadata).
		SetLabels(mc.labels)

	var metricsDataList []*MetricsData

	// Handle different protocols
	switch mc.metrics.Protocol {
	case "prometheus":
		// Handle Prometheus collection
		mc.logger.Info("collecting prometheus metrics", "metrics", mc.metrics.Name)
		metricsDataList = mc.collectPrometheus(builder)
	default:
		// Handle standard protocol collection
		if mc.collectService == nil {
			builder.SetCode(1).SetMsg(fmt.Sprintf("no collect service available for protocol: %s", mc.metrics.Protocol))
			metricsDataList = []*MetricsData{builder.Build()}
		} else {
			metricsDataList = mc.collectStandard(builder)
		}
	}

	// Dispatch collected data
	if mc.collectDispatcher != nil {
		if err := mc.collectDispatcher.DispatchCollectData(mc.timeout, mc.metrics, metricsDataList); err != nil {
			mc.logger.Error(err, "failed to dispatch collected data")
			return err
		}
	}

	duration := time.Since(mc.startTime)
	mc.logger.Info("metrics collection completed",
		"id", mc.id,
		"app", mc.app,
		"metrics", mc.metrics.Name,
		"duration", duration,
		"dataCount", len(metricsDataList))

	return nil
}

// collectPrometheus handles Prometheus metrics collection
func (mc *MetricsCollect) collectPrometheus(builder *MetricsDataBuilder) []*MetricsData {
	// TODO: Implement actual Prometheus collection logic
	// For now, return a dummy success response
	builder.SetCode(0).SetMsg("prometheus collection not implemented yet")
	return []*MetricsData{builder.Build()}
}

// collectStandard handles standard protocol collection
func (mc *MetricsCollect) collectStandard(builder *MetricsDataBuilder) []*MetricsData {
	// Use the collect service to perform collection
	collectedData := mc.collectService.Collect(mc.metrics)
	if collectedData == nil {
		mc.logger.Error(nil, "metrics collection returned nil")
		builder.SetCode(1).SetMsg("collection failed: received nil data")
		return []*MetricsData{builder.Build()}
	}

	// Convert CollectRepMetricsData to MetricsData
	metricsData := mc.convertToMetricsData(collectedData, builder)

	// Validate response
	if err := mc.validateResponse(metricsData); err != nil {
		mc.logger.Error(err, "metrics validation failed")
		builder.SetCode(1).SetMsg(fmt.Sprintf("validation failed: %v", err))
		return []*MetricsData{builder.Build()}
	}

	return []*MetricsData{metricsData}
}

// convertToMetricsData converts CollectRepMetricsData to MetricsData
func (mc *MetricsCollect) convertToMetricsData(collectedData *job.CollectRepMetricsData, builder *MetricsDataBuilder) *MetricsData {
	// Set basic fields from builder (which has ID, App, TenantID, etc.)
	builder.SetCode(collectedData.Code).
		SetMsg(collectedData.Msg).
		SetFields(collectedData.Fields)

	// Convert ValueRow to [][]string
	values := make([][]string, 0, len(collectedData.Values))
	for _, valueRow := range collectedData.Values {
		values = append(values, valueRow.Columns)
	}

	// Set values directly on the builder's data
	metricsData := builder.Build()
	metricsData.Values = values

	// Set metadata and labels if present
	if collectedData.Labels != nil {
		metricsData.Labels = collectedData.Labels
	}
	if collectedData.Metadata != nil {
		metricsData.Metadata = collectedData.Metadata
	}

	// Set time from collected data if available
	if collectedData.Time > 0 {
		metricsData.Time = collectedData.Time
	}

	return metricsData
}

// validateResponse validates the collected metrics data
func (mc *MetricsCollect) validateResponse(data *MetricsData) error {
	if data == nil {
		return fmt.Errorf("metrics data is nil")
	}

	// Check if we have fields and values
	if len(data.Fields) == 0 {
		mc.logger.Info("no fields defined for metrics", "metrics", mc.metrics.Name)
		return nil
	}

	// Validate that all rows have the correct number of columns
	expectedCols := len(data.Fields)
	for i, row := range data.Values {
		if len(row) != expectedCols {
			return fmt.Errorf("row %d has %d columns, expected %d", i, len(row), expectedCols)
		}
	}

	mc.logger.Info("metrics validation passed",
		"fields", len(data.Fields),
		"rows", len(data.Values))

	return nil
}

// Priority implements the Task interface
func (mc *MetricsCollect) Priority() int {
	return mc.priority
}

// Timeout implements the Task interface
func (mc *MetricsCollect) Timeout() time.Duration {
	// Parse timeout from metrics configuration
	if mc.metrics.Timeout != "" {
		if duration, err := time.ParseDuration(mc.metrics.Timeout); err == nil {
			return duration
		}
	}

	// Default timeout based on task type
	if mc.isCyclic {
		return 120 * time.Second // 2 minutes for periodic tasks
	} else {
		return 240 * time.Second // 4 minutes for one-time tasks
	}
}

// GetID returns the task ID
func (mc *MetricsCollect) GetID() int64 {
	return mc.id
}

// GetApp returns the app name
func (mc *MetricsCollect) GetApp() string {
	return mc.app
}

// GetMetrics returns the metrics name
func (mc *MetricsCollect) GetMetrics() string {
	return mc.metrics.Name
}

// GetStartTime returns the start time
func (mc *MetricsCollect) GetStartTime() time.Time {
	return mc.startTime
}

// GetNewTime returns the creation time
func (mc *MetricsCollect) GetNewTime() time.Time {
	return mc.newTime
}

// IsCyclic returns whether this is a cyclic task
func (mc *MetricsCollect) IsCyclic() bool {
	return mc.isCyclic
}
