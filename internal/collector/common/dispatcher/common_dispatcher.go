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

package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/collect"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// CommonDispatcherImpl is responsible for breaking down jobs into individual metrics collection tasks
// It manages collection timeouts, handles results, and decides on next round scheduling
type CommonDispatcherImpl struct {
	logger           logger.Logger
	metricsCollector MetricsCollector
	resultHandler    collect.ResultHandler
	mu               sync.RWMutex
}

// MetricsCollector interface for metrics collection
type MetricsCollector interface {
	CollectMetrics(metrics *jobtypes.Metrics, job *jobtypes.Job, timeout *jobtypes.Timeout) chan *jobtypes.CollectRepMetricsData
}

// NewCommonDispatcher creates a new common dispatcher
func NewCommonDispatcher(logger logger.Logger, metricsCollector MetricsCollector, resultHandler collect.ResultHandler) *CommonDispatcherImpl {
	return &CommonDispatcherImpl{
		logger:           logger.WithName("common-dispatcher"),
		metricsCollector: metricsCollector,
		resultHandler:    resultHandler,
	}
}

// DispatchMetricsTask dispatches a job by breaking it down into individual metrics collection tasks
func (cd *CommonDispatcherImpl) DispatchMetricsTask(ctx context.Context, job *jobtypes.Job, timeout *jobtypes.Timeout) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	cd.logger.Info("dispatching metrics task",
		"jobID", job.ID,
		"monitorID", job.MonitorID,
		"app", job.App,
		"metricsCount", len(job.Metrics))

	startTime := time.Now()

	// Get the next metrics to collect based on job priority and dependencies
	metricsToCollect := job.NextCollectMetrics()
	if len(metricsToCollect) == 0 {
		cd.logger.Info("no metrics to collect", "jobID", job.ID)
		return nil
	}

	// Create collection context with timeout
	collectCtx, collectCancel := context.WithTimeout(ctx, cd.getCollectionTimeout(job))
	defer collectCancel()

	// Collect all metrics concurrently using channels (Go's way of handling concurrency)
	resultChannels := make([]chan *jobtypes.CollectRepMetricsData, len(metricsToCollect))

	for i, metrics := range metricsToCollect {
		cd.logger.Info("starting metrics collection",
			"jobID", job.ID,
			"metricsName", metrics.Name,
			"protocol", metrics.Protocol)

		// Start metrics collection in goroutine
		resultChannels[i] = cd.metricsCollector.CollectMetrics(metrics, job, timeout)
	}

	// Collect results from all metrics collection tasks
	var results []*jobtypes.CollectRepMetricsData
	var errors []error

	for i, resultChan := range resultChannels {
		select {
		case result := <-resultChan:
			if result != nil {
				results = append(results, result)
				cd.logger.Info("received metrics result",
					"jobID", job.ID,
					"metricsName", metricsToCollect[i].Name,
					"code", result.Code)
			}
		case <-collectCtx.Done():
			errors = append(errors, fmt.Errorf("timeout collecting metrics: %s", metricsToCollect[i].Name))
			cd.logger.Info("metrics collection timeout",
				"jobID", job.ID,
				"metricsName", metricsToCollect[i].Name)
		}
	}

	duration := time.Since(startTime)

	// Handle collection results
	if err := cd.handleResults(results, job, errors); err != nil {
		cd.logger.Error(err, "failed to handle collection results",
			"jobID", job.ID,
			"duration", duration)
		return err
	}

	cd.logger.Info("successfully dispatched metrics task",
		"jobID", job.ID,
		"resultsCount", len(results),
		"errorsCount", len(errors),
		"duration", duration)

	return nil
}

// handleResults processes the collection results and decides on next actions
func (cd *CommonDispatcherImpl) handleResults(results []*jobtypes.CollectRepMetricsData, job *jobtypes.Job, errors []error) error {
	cd.logger.Info("handling collection results",
		"jobID", job.ID,
		"resultsCount", len(results),
		"errorsCount", len(errors))

	// Process successful results
	for _, result := range results {
		if err := cd.resultHandler.HandleCollectData(result, job); err != nil {
			cd.logger.Error(err, "failed to handle collect data",
				"jobID", job.ID,
				"metricsName", result.Metrics)
			// Continue processing other results even if one fails
		}
	}

	// Log errors but don't fail the entire job
	for _, err := range errors {
		cd.logger.Error(err, "metrics collection error", "jobID", job.ID)
	}

	// TODO: Implement logic to decide if we need to trigger next level metrics
	// or next round scheduling based on results
	cd.evaluateNextActions(job, results, errors)

	return nil
}

// evaluateNextActions decides what to do next based on collection results
func (cd *CommonDispatcherImpl) evaluateNextActions(job *jobtypes.Job, results []*jobtypes.CollectRepMetricsData, errors []error) {
	cd.logger.Info("evaluating next actions",
		"jobID", job.ID,
		"resultsCount", len(results),
		"errorsCount", len(errors))

	// Check if we have dependent metrics that need to be collected next
	hasNextLevel := cd.hasNextLevelMetrics(job, results)

	if hasNextLevel {
		cd.logger.Info("job has next level metrics to collect", "jobID", job.ID)
		// TODO: Schedule next level metrics collection
	}

	// For cyclic jobs, the rescheduling is handled by WheelTimerTask
	if job.IsCyclic {
		cd.logger.Info("cyclic job will be rescheduled by timer", "jobID", job.ID)
	}

	// TODO: Send results to data queue for further processing
	cd.sendToDataQueue(results, job)
}

// hasNextLevelMetrics checks if there are dependent metrics that should be collected next
func (cd *CommonDispatcherImpl) hasNextLevelMetrics(job *jobtypes.Job, results []*jobtypes.CollectRepMetricsData) bool {
	// This is a simplified check - in a full implementation, this would analyze
	// metric dependencies and priorities to determine if more metrics need collection
	return false
}

// sendToDataQueue sends collection results to the data processing queue
func (cd *CommonDispatcherImpl) sendToDataQueue(results []*jobtypes.CollectRepMetricsData, job *jobtypes.Job) {
	cd.logger.Info("sending results to data queue",
		"jobID", job.ID,
		"resultsCount", len(results))

	// TODO: Implement queue sending logic
	// For now, we'll just log that data should be sent
	for _, result := range results {
		cd.logger.Info("result ready for queue",
			"jobID", job.ID,
			"metricsName", result.Metrics,
			"code", result.Code,
			"valuesCount", len(result.Values))
	}
}

// getCollectionTimeout calculates the timeout for metrics collection
func (cd *CommonDispatcherImpl) getCollectionTimeout(job *jobtypes.Job) time.Duration {
	// Default timeout is 30 seconds
	defaultTimeout := 30 * time.Second

	// If job has a specific timeout, use it
	if job.DefaultInterval > 0 {
		// Use 80% of the interval as timeout to allow for rescheduling
		jobTimeout := time.Duration(job.DefaultInterval) * time.Second * 80 / 100
		if jobTimeout > defaultTimeout {
			return jobTimeout
		}
	}

	return defaultTimeout
}

// Stop stops the common dispatcher
func (cd *CommonDispatcherImpl) Stop() error {
	cd.logger.Info("stopping common dispatcher")
	return nil
}
