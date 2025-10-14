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

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// JobInfo holds information about a scheduled job
type JobInfo struct {
	Job             *jobtypes.Job
	Timeout         *jobtypes.Timeout
	CreatedAt       time.Time
	LastExecutedAt  time.Time
	NextExecutionAt time.Time
	ExecutionCount  int64
}

// TimeDispatch manages time-based job scheduling using a hashed wheel timer
type TimeDispatch struct {
	logger           logger.Logger
	wheelTimer       HashedWheelTimer
	commonDispatcher MetricsTaskDispatcher
	cyclicTasks      sync.Map // map[int64]*jobtypes.Timeout for cyclic jobs
	tempTasks        sync.Map // map[int64]*jobtypes.Timeout for one-time jobs
	jobInfos         sync.Map // map[int64]*JobInfo for detailed job information
	started          bool
	mu               sync.RWMutex

	// Statistics
	stats       *DispatcherStats
	statsLogger *time.Ticker
	statsCancel context.CancelFunc
}

// DispatcherStats holds dispatcher statistics
type DispatcherStats struct {
	StartTime          time.Time
	TotalJobsAdded     int64
	TotalJobsRemoved   int64
	TotalJobsExecuted  int64
	TotalJobsCompleted int64
	TotalJobsFailed    int64
	LastExecutionTime  time.Time
	mu                 sync.RWMutex
}

// MetricsTaskDispatcher interface for metrics task dispatching
type MetricsTaskDispatcher interface {
	DispatchMetricsTask(ctx context.Context, job *jobtypes.Job, timeout *jobtypes.Timeout) error
}

// HashedWheelTimer interface for time wheel operations
type HashedWheelTimer interface {
	NewTimeout(task jobtypes.TimerTask, delay time.Duration) *jobtypes.Timeout
	Start(ctx context.Context) error
	Stop() error
	GetStats() map[string]interface{}
}

// NewTimeDispatch creates a new time dispatcher
func NewTimeDispatch(logger logger.Logger, commonDispatcher MetricsTaskDispatcher) *TimeDispatch {
	td := &TimeDispatch{
		logger:           logger.WithName("time-dispatch"),
		commonDispatcher: commonDispatcher,
		started:          false,
		stats: &DispatcherStats{
			StartTime: time.Now(),
		},
	}

	// Create hashed wheel timer with reasonable defaults
	// 512 buckets, 100ms tick duration
	td.wheelTimer = NewHashedWheelTimer(512, 100*time.Millisecond, logger)

	return td
}

// AddJob adds a job to the time-based scheduler
// This corresponds to Java's TimerDispatcher.addJob method
func (td *TimeDispatch) AddJob(job *jobtypes.Job) error {
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}

	// Log job addition at debug level
	td.logger.V(1).Info("adding job",
		"jobID", job.ID,
		"interval", job.DefaultInterval)

	// Update statistics
	td.stats.mu.Lock()
	td.stats.TotalJobsAdded++
	td.stats.mu.Unlock()

	// Create wheel timer task
	timerTask := NewWheelTimerTask(job, td.commonDispatcher, td, td.logger)

	// Calculate delay
	var delay time.Duration
	if job.DefaultInterval > 0 {
		delay = time.Duration(job.DefaultInterval) * time.Millisecond
		td.logger.V(1).Info("calculated delay",
			"interval", job.DefaultInterval,
			"delay", delay)
	} else {
		delay = 10 * time.Second
	}

	// Create timeout
	timeout := td.wheelTimer.NewTimeout(timerTask, delay)

	// Create job info
	now := time.Now()
	jobInfo := &JobInfo{
		Job:             job,
		Timeout:         timeout,
		CreatedAt:       now,
		NextExecutionAt: now.Add(delay),
		ExecutionCount:  0,
	}

	// Store timeout and job info based on job type
	if job.IsCyclic {
		td.cyclicTasks.Store(job.ID, timeout)
		td.jobInfos.Store(job.ID, jobInfo)
		td.logger.Info("scheduled cyclic job",
			"jobID", job.ID,
			"nextExecution", jobInfo.NextExecutionAt)
	} else {
		td.tempTasks.Store(job.ID, timeout)
		td.jobInfos.Store(job.ID, jobInfo)
		td.logger.Info("scheduled one-time job",
			"jobID", job.ID,
			"nextExecution", jobInfo.NextExecutionAt)
	}

	return nil
}

// RemoveJob removes a job from the scheduler
func (td *TimeDispatch) RemoveJob(jobID int64) error {
	td.logger.V(1).Info("removing job", "jobID", jobID)

	// Update statistics
	td.stats.mu.Lock()
	td.stats.TotalJobsRemoved++
	td.stats.mu.Unlock()

	// Try to remove from cyclic tasks
	if timeoutInterface, exists := td.cyclicTasks.LoadAndDelete(jobID); exists {
		if timeout, ok := timeoutInterface.(*jobtypes.Timeout); ok {
			timeout.Cancel()
			td.jobInfos.Delete(jobID)
			td.logger.V(1).Info("removed cyclic job", "jobID", jobID)
			return nil
		}
	}

	// Try to remove from temp tasks
	if timeoutInterface, exists := td.tempTasks.LoadAndDelete(jobID); exists {
		if timeout, ok := timeoutInterface.(*jobtypes.Timeout); ok {
			timeout.Cancel()
			td.jobInfos.Delete(jobID)
			td.logger.V(1).Info("removed one-time job", "jobID", jobID)
			return nil
		}
	}

	return fmt.Errorf("job not found: %d", jobID)
}

// Start starts the time dispatcher
func (td *TimeDispatch) Start(ctx context.Context) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	if td.started {
		return fmt.Errorf("time dispatcher already started")
	}

	td.logger.Info("starting time dispatcher")

	// Start the wheel timer
	if err := td.wheelTimer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start wheel timer: %w", err)
	}

	// Start periodic statistics logging
	td.startStatsLogging(ctx)

	td.started = true
	// Started successfully
	return nil
}

// Stop stops the time dispatcher
func (td *TimeDispatch) Stop() error {
	td.mu.Lock()
	defer td.mu.Unlock()

	if !td.started {
		return nil
	}

	td.logger.Info("stopping time dispatcher")

	// Stop statistics logging
	td.stopStatsLogging()

	// Cancel all running tasks
	td.cyclicTasks.Range(func(key, value interface{}) bool {
		if timeout, ok := value.(*jobtypes.Timeout); ok {
			timeout.Cancel()
		}
		return true
	})

	td.tempTasks.Range(func(key, value interface{}) bool {
		if timeout, ok := value.(*jobtypes.Timeout); ok {
			timeout.Cancel()
		}
		return true
	})

	// Clear maps
	td.cyclicTasks = sync.Map{}
	td.tempTasks = sync.Map{}
	td.jobInfos = sync.Map{}

	// Stop wheel timer
	if err := td.wheelTimer.Stop(); err != nil {
		td.logger.Error(err, "error stopping wheel timer")
	}

	td.started = false
	td.logger.Info("time dispatcher stopped")
	return nil
}

// Stats returns dispatcher statistics
func (td *TimeDispatch) Stats() map[string]any {
	cyclicCount := 0
	tempCount := 0

	td.cyclicTasks.Range(func(key, value interface{}) bool {
		cyclicCount++
		return true
	})

	td.tempTasks.Range(func(key, value interface{}) bool {
		tempCount++
		return true
	})

	td.stats.mu.RLock()
	uptime := time.Since(td.stats.StartTime)
	totalJobsAdded := td.stats.TotalJobsAdded
	totalJobsRemoved := td.stats.TotalJobsRemoved
	totalJobsExecuted := td.stats.TotalJobsExecuted
	totalJobsCompleted := td.stats.TotalJobsCompleted
	totalJobsFailed := td.stats.TotalJobsFailed
	lastExecutionTime := td.stats.LastExecutionTime
	td.stats.mu.RUnlock()

	return map[string]any{
		"cyclicJobs":         cyclicCount,
		"tempJobs":           tempCount,
		"started":            td.started,
		"uptime":             uptime,
		"totalJobsAdded":     totalJobsAdded,
		"totalJobsRemoved":   totalJobsRemoved,
		"totalJobsExecuted":  totalJobsExecuted,
		"totalJobsCompleted": totalJobsCompleted,
		"totalJobsFailed":    totalJobsFailed,
		"lastExecutionTime":  lastExecutionTime,
	}
}

// RecordJobExecution records job execution statistics
func (td *TimeDispatch) RecordJobExecution() {
	td.stats.mu.Lock()
	td.stats.TotalJobsExecuted++
	td.stats.LastExecutionTime = time.Now()
	td.stats.mu.Unlock()
}

// RecordJobCompleted records job completion
func (td *TimeDispatch) RecordJobCompleted() {
	td.stats.mu.Lock()
	td.stats.TotalJobsCompleted++
	td.stats.mu.Unlock()
}

// RecordJobFailed records job failure
func (td *TimeDispatch) RecordJobFailed() {
	td.stats.mu.Lock()
	td.stats.TotalJobsFailed++
	td.stats.mu.Unlock()
}

// UpdateJobExecution updates job execution information
func (td *TimeDispatch) UpdateJobExecution(jobID int64) {
	if jobInfoInterface, exists := td.jobInfos.Load(jobID); exists {
		if jobInfo, ok := jobInfoInterface.(*JobInfo); ok {
			now := time.Now()
			jobInfo.LastExecutedAt = now
			jobInfo.ExecutionCount++

			// Update next execution time for cyclic jobs
			if jobInfo.Job.IsCyclic && jobInfo.Job.DefaultInterval > 0 {
				jobInfo.NextExecutionAt = now.Add(time.Duration(jobInfo.Job.DefaultInterval) * time.Second)
			}
		}
	}
}

// RescheduleJob reschedules a cyclic job for the next execution
func (td *TimeDispatch) RescheduleJob(job *jobtypes.Job) error {
	if !job.IsCyclic || job.DefaultInterval <= 0 {
		return fmt.Errorf("job is not cyclable or has invalid interval")
	}

	// Create new timer task for next execution
	nextDelay := time.Duration(job.DefaultInterval) * time.Second
	timerTask := NewWheelTimerTask(job, td.commonDispatcher, td, td.logger)

	// Schedule the next execution
	timeout := td.wheelTimer.NewTimeout(timerTask, nextDelay)

	// Update the timeout in cyclicTasks
	td.cyclicTasks.Store(job.ID, timeout)

	// Update job info next execution time
	if jobInfoInterface, exists := td.jobInfos.Load(job.ID); exists {
		if jobInfo, ok := jobInfoInterface.(*JobInfo); ok {
			jobInfo.NextExecutionAt = time.Now().Add(nextDelay)
			jobInfo.Timeout = timeout
		}
	}

	nextExecutionTime := time.Now().Add(nextDelay)
	td.logger.Info("rescheduled cyclic job",
		"jobID", job.ID,
		"interval", job.DefaultInterval,
		"nextExecution", nextExecutionTime,
		"delay", nextDelay)

	return nil
}

// startStatsLogging starts periodic statistics logging
func (td *TimeDispatch) startStatsLogging(ctx context.Context) {
	// Create a context that can be cancelled
	statsCtx, cancel := context.WithCancel(ctx)
	td.statsCancel = cancel

	// Create ticker for 1-minute intervals
	td.statsLogger = time.NewTicker(1 * time.Minute)

	go func() {
		defer td.statsLogger.Stop()

		// Log initial statistics
		td.logStatistics()

		for {
			select {
			case <-td.statsLogger.C:
				td.logStatistics()
			case <-statsCtx.Done():
				td.logger.Info("statistics logging stopped")
				return
			}
		}
	}()

	td.logger.Info("statistics logging started", "interval", "1m")
}

// stopStatsLogging stops periodic statistics logging
func (td *TimeDispatch) stopStatsLogging() {
	if td.statsCancel != nil {
		td.statsCancel()
		td.statsCancel = nil
	}
	if td.statsLogger != nil {
		td.statsLogger.Stop()
		td.statsLogger = nil
	}
}

// logStatistics logs current dispatcher statistics
func (td *TimeDispatch) logStatistics() {
	stats := td.Stats()

	// Get detailed job information
	var cyclicJobDetails []map[string]any
	var tempJobDetails []map[string]any

	td.cyclicTasks.Range(func(key, value interface{}) bool {
		if jobID, ok := key.(int64); ok {
			if jobInfoInterface, exists := td.jobInfos.Load(jobID); exists {
				if jobInfo, ok := jobInfoInterface.(*JobInfo); ok {
					now := time.Now()
					detail := map[string]any{
						"jobID":         jobID,
						"app":           jobInfo.Job.App,
						"monitorID":     jobInfo.Job.MonitorID,
						"interval":      jobInfo.Job.DefaultInterval,
						"createdAt":     jobInfo.CreatedAt,
						"lastExecuted":  jobInfo.LastExecutedAt,
						"nextExecution": jobInfo.NextExecutionAt,
						"execCount":     jobInfo.ExecutionCount,
						"timeSinceLastExec": func() string {
							if jobInfo.LastExecutedAt.IsZero() {
								return "never"
							}
							return now.Sub(jobInfo.LastExecutedAt).String()
						}(),
						"timeToNextExec": func() string {
							if jobInfo.NextExecutionAt.IsZero() {
								return "unknown"
							}
							if jobInfo.NextExecutionAt.Before(now) {
								return "overdue"
							}
							return jobInfo.NextExecutionAt.Sub(now).String()
						}(),
					}
					cyclicJobDetails = append(cyclicJobDetails, detail)
				}
			}
		}
		return true
	})

	td.tempTasks.Range(func(key, value interface{}) bool {
		if jobID, ok := key.(int64); ok {
			if jobInfoInterface, exists := td.jobInfos.Load(jobID); exists {
				if jobInfo, ok := jobInfoInterface.(*JobInfo); ok {
					detail := map[string]any{
						"jobID":         jobID,
						"app":           jobInfo.Job.App,
						"monitorID":     jobInfo.Job.MonitorID,
						"createdAt":     jobInfo.CreatedAt,
						"nextExecution": jobInfo.NextExecutionAt,
						"execCount":     jobInfo.ExecutionCount,
					}
					tempJobDetails = append(tempJobDetails, detail)
				}
			}
		}
		return true
	})

	// Get timer wheel statistics
	var timerStats map[string]interface{}
	if td.wheelTimer != nil {
		timerStats = td.wheelTimer.GetStats()
	}

	td.logger.Info("📊 Dispatcher Statistics Report",
		"uptime", stats["uptime"],
		"activeJobs", map[string]any{
			"cyclic": stats["cyclicJobs"],
			"temp":   stats["tempJobs"],
			"total":  stats["cyclicJobs"].(int) + stats["tempJobs"].(int),
		},
		"jobCounters", map[string]any{
			"added":     stats["totalJobsAdded"],
			"removed":   stats["totalJobsRemoved"],
			"executed":  stats["totalJobsExecuted"],
			"completed": stats["totalJobsCompleted"],
			"failed":    stats["totalJobsFailed"],
		},
		"timerWheelConfig", timerStats,
		"lastExecution", stats["lastExecutionTime"],
		"cyclicJobs", cyclicJobDetails,
		"tempJobs", tempJobDetails,
	)
}
