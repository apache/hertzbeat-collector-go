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

// TimeDispatch manages time-based job scheduling using a hashed wheel timer
type TimeDispatch struct {
	logger           logger.Logger
	wheelTimer       HashedWheelTimer
	commonDispatcher MetricsTaskDispatcher
	cyclicTasks      sync.Map // map[int64]*jobtypes.Timeout for cyclic jobs
	tempTasks        sync.Map // map[int64]*jobtypes.Timeout for one-time jobs
	started          bool
	mu               sync.RWMutex
}

// MetricsTaskDispatcher interface for metrics task dispatching
type MetricsTaskDispatcher interface {
	DispatchMetricsTask(ctx context.Context, job *jobtypes.Job, timeout *jobtypes.Timeout) error
}

// HashedWheelTimer interface for time wheel operations
type HashedWheelTimer interface {
	NewTimeout(task jobtypes.TimerTask, delay time.Duration) *jobtypes.Timeout
	Start() error
	Stop() error
}

// NewTimeDispatch creates a new time dispatcher
func NewTimeDispatch(logger logger.Logger, commonDispatcher MetricsTaskDispatcher) *TimeDispatch {
	td := &TimeDispatch{
		logger:           logger.WithName("time-dispatch"),
		commonDispatcher: commonDispatcher,
		started:          false,
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

	td.logger.Info("adding job to time dispatcher",
		"jobID", job.ID,
		"isCyclic", job.IsCyclic,
		"interval", job.DefaultInterval)

	// Create wheel timer task
	timerTask := NewWheelTimerTask(job, td.commonDispatcher, td.logger)

	// Calculate delay
	var delay time.Duration
	if job.DefaultInterval > 0 {
		delay = time.Duration(job.DefaultInterval) * time.Second
	} else {
		delay = 30 * time.Second // default interval
	}

	// Create timeout
	timeout := td.wheelTimer.NewTimeout(timerTask, delay)

	// Store timeout based on job type
	if job.IsCyclic {
		td.cyclicTasks.Store(job.ID, timeout)
		td.logger.Info("added cyclic job to scheduler", "jobID", job.ID, "delay", delay)
	} else {
		td.tempTasks.Store(job.ID, timeout)
		td.logger.Info("added one-time job to scheduler", "jobID", job.ID, "delay", delay)
	}

	return nil
}

// RemoveJob removes a job from the scheduler
func (td *TimeDispatch) RemoveJob(jobID int64) error {
	td.logger.Info("removing job from time dispatcher", "jobID", jobID)

	// Try to remove from cyclic tasks
	if timeoutInterface, exists := td.cyclicTasks.LoadAndDelete(jobID); exists {
		if timeout, ok := timeoutInterface.(*jobtypes.Timeout); ok {
			timeout.Cancel()
			td.logger.Info("removed cyclic job", "jobID", jobID)
			return nil
		}
	}

	// Try to remove from temp tasks
	if timeoutInterface, exists := td.tempTasks.LoadAndDelete(jobID); exists {
		if timeout, ok := timeoutInterface.(*jobtypes.Timeout); ok {
			timeout.Cancel()
			td.logger.Info("removed one-time job", "jobID", jobID)
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
	if err := td.wheelTimer.Start(); err != nil {
		return fmt.Errorf("failed to start wheel timer: %w", err)
	}

	td.started = true
	td.logger.Info("time dispatcher started successfully")
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

	return map[string]any{
		"cyclicJobs": cyclicCount,
		"tempJobs":   tempCount,
		"started":    td.started,
	}
}
