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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/worker"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// CollectResponseEventListener defines the interface for handling collect response events
type CollectResponseEventListener interface {
	Response(metricsData []interface{}) // TODO: Define proper MetricsData type
}

// TimerDispatcher manages job scheduling using a hashed wheel timer
type TimerDispatcher struct {
	wheelTimer *TimerWheel

	// Maps to track scheduled tasks
	cyclicTasks    sync.Map // map[int64]*jobtypes.Timeout
	tempTasks      sync.Map // map[int64]*jobtypes.Timeout
	eventListeners sync.Map // map[int64]CollectResponseEventListener

	// Timeout monitoring
	timeoutMonitor       sync.Map // map[string]*MetricsTimeoutInfo
	timeoutCheckInterval time.Duration
	timeoutCheckTicker   *time.Ticker
	stopTimeoutMonitor   chan struct{}

	// State management
	started atomic.Bool

	// Dependencies
	metricsDispatcher MetricsTaskDispatcher
	workerPool        *worker.PriorityWorkerPool
	collectDispatcher worker.CollectDataDispatcher
	collectService    worker.CollectService
	logger            logger.Logger
}

// MetricsTimeoutInfo tracks timeout information for metrics collection tasks
type MetricsTimeoutInfo struct {
	StartTime   time.Time
	Metrics     *job.Metrics
	Timeout     *job.Timeout
	MaxDuration time.Duration
	RetryCount  int
	MaxRetries  int
}

// NewTimerDispatcher creates a new timer dispatcher
func NewTimerDispatcher(logger logger.Logger) *TimerDispatcher {
	// Create worker pool with default configuration
	workerConfig := worker.DefaultWorkerPoolConfig()

	td := &TimerDispatcher{
		wheelTimer:           NewTimerWheel(logger.WithName("timer-wheel")),
		workerPool:           worker.NewPriorityWorkerPool(workerConfig, logger),
		timeoutCheckInterval: 10 * time.Second, // Check for timeouts every 10 seconds
		stopTimeoutMonitor:   make(chan struct{}),
		logger:               logger.WithName("timer-dispatcher"),
	}

	td.started.Store(true)

	return td
}

// SetMetricsDispatcher sets the metrics task dispatcher
func (td *TimerDispatcher) SetMetricsDispatcher(dispatcher MetricsTaskDispatcher) {
	td.metricsDispatcher = dispatcher
}

// SetCollectDataDispatcher sets the collect data dispatcher
func (td *TimerDispatcher) SetCollectDataDispatcher(dispatcher worker.CollectDataDispatcher) {
	td.collectDispatcher = dispatcher
}

// SetCollectService sets the collect service for metrics collection
func (td *TimerDispatcher) SetCollectService(service worker.CollectService) {
	td.collectService = service
}

// Start starts the timer dispatcher
func (td *TimerDispatcher) Start() error {
	td.logger.Info("starting timer dispatcher")

	// Start worker pool first
	if err := td.workerPool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start timer wheel
	if err := td.wheelTimer.Start(); err != nil {
		td.workerPool.Stop() // cleanup on failure
		return fmt.Errorf("failed to start timer wheel: %w", err)
	}

	// Start timeout monitoring
	td.startTimeoutMonitoring()

	td.logger.Info("timer dispatcher started successfully")
	return nil
}

// Stop stops the timer dispatcher
func (td *TimerDispatcher) Stop() error {
	td.logger.Info("stopping timer dispatcher")

	// Mark as offline
	td.GoOffline()

	// Stop timeout monitoring
	td.stopTimeoutMonitoring()

	// Stop the timer wheel first
	if err := td.wheelTimer.Stop(); err != nil {
		td.logger.Info("failed to stop timer wheel", "error", err)
	}

	// Stop the worker pool
	if err := td.workerPool.Stop(); err != nil {
		td.logger.Info("failed to stop worker pool", "error", err)
	}

	td.logger.Info("timer dispatcher stopped successfully")
	return nil
}

// AddJob adds a job to the scheduler
func (td *TimerDispatcher) AddJob(job *job.Job, eventListener CollectResponseEventListener) error {
	if !td.started.Load() {
		td.logger.Info("collector is offline, cannot dispatch collect jobs")
		return fmt.Errorf("collector is offline")
	}

	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	// Create wheel timer task
	timerTask := NewWheelTimerTask(job, td.metricsDispatcher, td.logger)

	// Calculate initial delay
	var delay time.Duration
	if job.DefaultInterval > 0 {
		delay = time.Duration(job.DefaultInterval) * time.Second
	} else {
		delay = 30 * time.Second // default delay
	}

	// Schedule the task
	timeout := td.wheelTimer.NewTimeout(timerTask, delay)
	if timeout == nil {
		return fmt.Errorf("failed to schedule job: timer wheel may be stopped")
	}

	if job.IsCyclic {
		// Cyclic job - store for management
		td.cyclicTasks.Store(job.ID, timeout)
		td.logger.Info("scheduled cyclic job",
			"jobId", job.ID,
			"monitorId", job.MonitorID,
			"app", job.App,
			"interval", delay)
	} else {
		// One-time job - store temporarily and register event listener
		td.tempTasks.Store(job.ID, timeout)
		if eventListener != nil {
			td.eventListeners.Store(job.ID, eventListener)
		}

		// Set interval to 0 for one-time jobs
		for i := range job.Metrics {
			job.Metrics[i].Interval = 0
		}
		job.Intervals = []int64{0}

		td.logger.Info("scheduled one-time job",
			"jobId", job.ID,
			"monitorId", job.MonitorID,
			"app", job.App,
			"delay", delay)
	}

	return nil
}

// CyclicJob reschedules a cyclic job for its next execution
func (td *TimerDispatcher) CyclicJob(timerTask *WheelTimerTask, interval time.Duration) error {
	if !td.started.Load() {
		td.logger.Info("collector is offline, cannot dispatch collect jobs")
		return fmt.Errorf("collector is offline")
	}

	if timerTask == nil {
		return fmt.Errorf("timer task cannot be nil")
	}

	job := timerTask.GetJob()
	if job == nil {
		return fmt.Errorf("job in timer task cannot be nil")
	}

	jobID := job.ID

	// Check if the job is still active (not cancelled)
	if _, exists := td.cyclicTasks.Load(jobID); !exists {
		td.logger.Info("cyclic job was cancelled, not rescheduling", "jobId", jobID)
		return nil
	}

	// Schedule next execution
	timeout := td.wheelTimer.NewTimeout(timerTask, interval)
	if timeout == nil {
		return fmt.Errorf("failed to reschedule cyclic job: timer wheel may be stopped")
	}

	// Update the stored timeout
	td.cyclicTasks.Store(jobID, timeout)

	td.logger.Info("rescheduled cyclic job",
		"jobId", jobID,
		"interval", interval,
		"nextExecution", timeout.Deadline())

	return nil
}

// DeleteJob cancels and removes a job from the scheduler
func (td *TimerDispatcher) DeleteJob(jobID int64, isCyclic bool) bool {
	td.logger.Info("deleting job", "jobId", jobID, "isCyclic", isCyclic)

	var removed bool

	if isCyclic {
		if timeoutInterface, exists := td.cyclicTasks.LoadAndDelete(jobID); exists {
			if timeout, ok := timeoutInterface.(*job.Timeout); ok {
				timeout.Cancel()
				removed = true
			}
		}
	} else {
		if timeoutInterface, exists := td.tempTasks.LoadAndDelete(jobID); exists {
			if timeout, ok := timeoutInterface.(*job.Timeout); ok {
				timeout.Cancel()
				removed = true
			}
		}
		// Also remove event listener
		td.eventListeners.Delete(jobID)
	}

	if removed {
		td.logger.Info("successfully deleted job", "jobId", jobID, "isCyclic", isCyclic)
	} else {
		td.logger.Info("job not found for deletion", "jobId", jobID, "isCyclic", isCyclic)
	}

	return removed
}

// GoOnline brings the dispatcher online and clears all existing tasks
func (td *TimerDispatcher) GoOnline() {
	td.logger.Info("bringing timer dispatcher online")

	// Cancel all existing tasks
	td.cyclicTasks.Range(func(key, value interface{}) bool {
		if timeout, ok := value.(*job.Timeout); ok {
			timeout.Cancel()
		}
		td.cyclicTasks.Delete(key)
		return true
	})

	td.tempTasks.Range(func(key, value interface{}) bool {
		if timeout, ok := value.(*job.Timeout); ok {
			timeout.Cancel()
		}
		td.tempTasks.Delete(key)
		return true
	})

	td.eventListeners.Range(func(key, value interface{}) bool {
		td.eventListeners.Delete(key)
		return true
	})

	td.started.Store(true)
	td.logger.Info("timer dispatcher is now online")
}

// GoOffline brings the dispatcher offline
func (td *TimerDispatcher) GoOffline() {
	td.logger.Info("bringing timer dispatcher offline")

	td.started.Store(false)

	// Cancel all existing tasks
	td.cyclicTasks.Range(func(key, value interface{}) bool {
		if timeout, ok := value.(*job.Timeout); ok {
			timeout.Cancel()
		}
		td.cyclicTasks.Delete(key)
		return true
	})

	td.tempTasks.Range(func(key, value interface{}) bool {
		if timeout, ok := value.(*job.Timeout); ok {
			timeout.Cancel()
		}
		td.tempTasks.Delete(key)
		return true
	})

	td.eventListeners.Range(func(key, value interface{}) bool {
		td.eventListeners.Delete(key)
		return true
	})

	td.logger.Info("timer dispatcher is now offline")
}

// ResponseSyncJobData handles response from sync job execution
func (td *TimerDispatcher) ResponseSyncJobData(jobID int64, metricsData []interface{}) {
	td.tempTasks.Delete(jobID)

	if listenerInterface, exists := td.eventListeners.LoadAndDelete(jobID); exists {
		if listener, ok := listenerInterface.(CollectResponseEventListener); ok {
			listener.Response(metricsData)
		}
	}

	td.logger.Info("handled sync job response", "jobId", jobID)
}

// DispatchMetricsTask implements MetricsTaskDispatcher interface
func (td *TimerDispatcher) DispatchMetricsTask(timeout *job.Timeout) error {
	if task, ok := timeout.Task().(*WheelTimerTask); ok {
		job := task.GetJob()
		td.logger.Info("dispatching metrics task",
			"jobId", job.ID,
			"monitorId", job.MonitorID,
			"app", job.App)

		// Create metrics collection tasks for each metric in the job
		for _, metric := range job.Metrics {
			metricsCollect := worker.NewMetricsCollect(
				&metric,
				timeout,
				td.collectDispatcher,
				"collector-go",    // collector identity
				td.collectService, // Use the configured collect service
				td.logger,
			)

			if metricsCollect != nil {
				// Submit to worker pool for execution
				if err := td.workerPool.Submit(metricsCollect); err != nil {
					td.logger.Info("failed to submit metrics collection task",
						"error", err,
						"jobId", job.ID,
						"metric", metric.Name)
				} else {
					td.logger.Info("submitted metrics collection task",
						"jobId", job.ID,
						"metric", metric.Name,
						"protocol", metric.Protocol)
				}
			}
		}

		// If it's a cyclic job, reschedule it
		if job.IsCyclic {
			interval := time.Duration(job.DefaultInterval) * time.Second
			if interval <= 0 {
				interval = 30 * time.Second
			}
			return td.CyclicJob(task, interval)
		}
	}

	return nil
}

// GetStats returns statistics about the timer dispatcher
func (td *TimerDispatcher) GetStats() TimerDispatcherStats {
	cyclicCount := 0
	tempCount := 0
	listenerCount := 0

	td.cyclicTasks.Range(func(key, value interface{}) bool {
		cyclicCount++
		return true
	})

	td.tempTasks.Range(func(key, value interface{}) bool {
		tempCount++
		return true
	})

	td.eventListeners.Range(func(key, value interface{}) bool {
		listenerCount++
		return true
	})

	// Get worker pool and task queue stats
	workerPoolStats, taskQueueStats := td.workerPool.GetStats()

	return TimerDispatcherStats{
		IsStarted:       td.started.Load(),
		CyclicTaskCount: cyclicCount,
		TempTaskCount:   tempCount,
		ListenerCount:   listenerCount,
		TimerWheelStats: td.wheelTimer.Stats(),
		WorkerPoolStats: workerPoolStats,
		TaskQueueStats:  taskQueueStats,
	}
}

// TimerDispatcherStats contains statistics about the timer dispatcher
type TimerDispatcherStats struct {
	IsStarted       bool                   `json:"isStarted"`
	CyclicTaskCount int                    `json:"cyclicTaskCount"`
	TempTaskCount   int                    `json:"tempTaskCount"`
	ListenerCount   int                    `json:"listenerCount"`
	TimerWheelStats TimerWheelStats        `json:"timerWheelStats"`
	WorkerPoolStats worker.WorkerPoolStats `json:"workerPoolStats"`
	TaskQueueStats  worker.TaskQueueStats  `json:"taskQueueStats"`
}

// SetMetricsTaskDispatcher sets the metrics task dispatcher (for circular dependency resolution)
func (td *TimerDispatcher) SetMetricsTaskDispatcher(dispatcher interface{}) {
	// This method is for interface compatibility with communication layer
	// The TimerDispatcher itself implements MetricsTaskDispatcher
}

// startTimeoutMonitoring starts the timeout monitoring goroutine
func (td *TimerDispatcher) startTimeoutMonitoring() {
	td.timeoutCheckTicker = time.NewTicker(td.timeoutCheckInterval)

	go func() {
		defer td.timeoutCheckTicker.Stop()

		for {
			select {
			case <-td.timeoutCheckTicker.C:
				td.checkTimeouts()
			case <-td.stopTimeoutMonitor:
				td.logger.Info("timeout monitoring stopped")
				return
			}
		}
	}()

	td.logger.Info("timeout monitoring started", "interval", td.timeoutCheckInterval)
}

// stopTimeoutMonitoring stops the timeout monitoring
func (td *TimerDispatcher) stopTimeoutMonitoring() {
	if td.timeoutCheckTicker != nil {
		close(td.stopTimeoutMonitor)
		td.timeoutCheckTicker.Stop()
	}
}

// checkTimeouts checks for timed out metrics collection tasks
func (td *TimerDispatcher) checkTimeouts() {
	now := time.Now()
	var timeoutKeys []string

	td.timeoutMonitor.Range(func(key, value interface{}) bool {
		timeoutKey := key.(string)
		timeoutInfo := value.(*MetricsTimeoutInfo)

		// Check if task has timed out
		if now.Sub(timeoutInfo.StartTime) > timeoutInfo.MaxDuration {
			timeoutKeys = append(timeoutKeys, timeoutKey)

			td.logger.Info("metrics collection task timed out",
				"key", timeoutKey,
				"metrics", timeoutInfo.Metrics.Name,
				"duration", now.Sub(timeoutInfo.StartTime),
				"maxDuration", timeoutInfo.MaxDuration,
				"retryCount", timeoutInfo.RetryCount)
		}

		return true
	})

	// Handle timed out tasks
	for _, timeoutKey := range timeoutKeys {
		td.handleTimeout(timeoutKey)
	}
}

// handleTimeout handles a timed out metrics collection task
func (td *TimerDispatcher) handleTimeout(timeoutKey string) {
	value, exists := td.timeoutMonitor.Load(timeoutKey)
	if !exists {
		return
	}

	timeoutInfo := value.(*MetricsTimeoutInfo)

	// Remove from timeout monitor
	td.timeoutMonitor.Delete(timeoutKey)

	// Check if we should retry
	if timeoutInfo.RetryCount < timeoutInfo.MaxRetries {
		td.logger.Info("retrying timed out metrics collection task",
			"key", timeoutKey,
			"metrics", timeoutInfo.Metrics.Name,
			"retryCount", timeoutInfo.RetryCount+1,
			"maxRetries", timeoutInfo.MaxRetries)

		// Retry the task
		td.retryMetricsCollection(timeoutInfo)
	} else {
		td.logger.Info("metrics collection task exceeded max retries",
			"key", timeoutKey,
			"metrics", timeoutInfo.Metrics.Name,
			"retryCount", timeoutInfo.RetryCount,
			"maxRetries", timeoutInfo.MaxRetries)

		// TODO: Notify failure to appropriate handlers
		// This could involve sending a failure response or triggering alerts
	}
}

// retryMetricsCollection retries a failed metrics collection task
func (td *TimerDispatcher) retryMetricsCollection(timeoutInfo *MetricsTimeoutInfo) {
	// Create new timeout info with incremented retry count
	newTimeoutInfo := &MetricsTimeoutInfo{
		StartTime:   time.Now(),
		Metrics:     timeoutInfo.Metrics,
		Timeout:     timeoutInfo.Timeout,
		MaxDuration: timeoutInfo.MaxDuration,
		RetryCount:  timeoutInfo.RetryCount + 1,
		MaxRetries:  timeoutInfo.MaxRetries,
	}

	// Generate retry key
	retryKey := fmt.Sprintf("%s-retry-%d",
		td.generateTimeoutKey(timeoutInfo.Metrics, timeoutInfo.Timeout),
		newTimeoutInfo.RetryCount)

	// Add to timeout monitor
	td.timeoutMonitor.Store(retryKey, newTimeoutInfo)

	// Dispatch the retry task with exponential backoff
	retryDelay := time.Duration(timeoutInfo.RetryCount) * 5 * time.Second
	time.AfterFunc(retryDelay, func() {
		if td.metricsDispatcher != nil {
			if err := td.metricsDispatcher.DispatchMetricsTask(timeoutInfo.Timeout); err != nil {
				td.logger.Error(err, "failed to retry metrics collection task",
					"key", retryKey,
					"metrics", timeoutInfo.Metrics.Name)
				// Remove from monitor if retry dispatch fails
				td.timeoutMonitor.Delete(retryKey)
			}
		}
	})
}

// AddMetricsTimeout adds a metrics collection task to timeout monitoring
func (td *TimerDispatcher) AddMetricsTimeout(metrics *job.Metrics, timeout *job.Timeout) {
	// Calculate max duration based on metrics timeout or default
	maxDuration := 120 * time.Second // default 2 minutes
	if metrics.Timeout != "" {
		if duration, err := time.ParseDuration(metrics.Timeout); err == nil {
			maxDuration = duration
		}
	}

	timeoutInfo := &MetricsTimeoutInfo{
		StartTime:   time.Now(),
		Metrics:     metrics,
		Timeout:     timeout,
		MaxDuration: maxDuration,
		RetryCount:  0,
		MaxRetries:  3, // default max retries
	}

	key := td.generateTimeoutKey(metrics, timeout)
	td.timeoutMonitor.Store(key, timeoutInfo)

	td.logger.Info("added metrics timeout monitoring",
		"key", key,
		"metrics", metrics.Name,
		"maxDuration", maxDuration,
		"maxRetries", timeoutInfo.MaxRetries)
}

// RemoveMetricsTimeout removes a metrics collection task from timeout monitoring
func (td *TimerDispatcher) RemoveMetricsTimeout(metrics *job.Metrics, timeout *job.Timeout) {
	key := td.generateTimeoutKey(metrics, timeout)
	td.timeoutMonitor.Delete(key)

	td.logger.Info("removed metrics timeout monitoring",
		"key", key,
		"metrics", metrics.Name)
}

// generateTimeoutKey generates a unique key for timeout monitoring
func (td *TimerDispatcher) generateTimeoutKey(metrics *job.Metrics, timeout *job.Timeout) string {
	// Use a combination of metrics name and timestamp to ensure uniqueness
	return fmt.Sprintf("%s-%d-%s", metrics.Name, time.Now().UnixNano(), metrics.Protocol)
}
