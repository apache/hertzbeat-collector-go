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
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// StatsRecorder interface for recording job execution statistics
type StatsRecorder interface {
	RecordJobExecution()
	RecordJobCompleted()
	RecordJobFailed()
	UpdateJobExecution(jobID int64)
	RescheduleJob(job *job.Job) error
}

// WheelTimerTask represents a task that runs in the time wheel
// This corresponds to Java's WheelTimerTask
type WheelTimerTask struct {
	job              *job.Job
	commonDispatcher MetricsTaskDispatcher
	statsRecorder    StatsRecorder
	logger           logger.Logger
}

// NewWheelTimerTask creates a new wheel timer task
func NewWheelTimerTask(job *job.Job, commonDispatcher MetricsTaskDispatcher, statsRecorder StatsRecorder, logger logger.Logger) *WheelTimerTask {
	return &WheelTimerTask{
		job:              job,
		commonDispatcher: commonDispatcher,
		statsRecorder:    statsRecorder,
		logger:           logger.WithName("wheel-timer-task"),
	}
}

// Run executes the timer task
// This corresponds to Java's WheelTimerTask.run method
func (wtt *WheelTimerTask) Run(timeout *job.Timeout) error {
	if wtt.job == nil {
		wtt.logger.Error(nil, "job is nil, cannot execute")
		return fmt.Errorf("job is nil, cannot execute")
	}

	wtt.logger.V(1).Info("executing task",
		"jobID", wtt.job.ID,
		"app", wtt.job.App)

	// Record job execution start
	if wtt.statsRecorder != nil {
		wtt.statsRecorder.RecordJobExecution()
		wtt.statsRecorder.UpdateJobExecution(wtt.job.ID)
	}

	startTime := time.Now()

	// Create a context for this specific task execution
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dispatch metrics task through common dispatcher
	// This is where the job gets broken down into individual metric collection tasks
	err := wtt.commonDispatcher.DispatchMetricsTask(ctx, wtt.job, timeout)

	duration := time.Since(startTime)

	if err != nil {
		wtt.logger.Error(err, "failed to dispatch metrics task",
			"jobID", wtt.job.ID,
			"duration", duration)

		// Record job failure
		if wtt.statsRecorder != nil {
			wtt.statsRecorder.RecordJobFailed()
		}
		return err
	}

	wtt.logger.V(1).Info("task completed",
		"jobID", wtt.job.ID,
		"duration", duration)

	// Record job completion
	if wtt.statsRecorder != nil {
		wtt.statsRecorder.RecordJobCompleted()
	}

	// If this is a cyclic job, reschedule it for the next execution
	if wtt.job.IsCyclic && wtt.job.DefaultInterval > 0 {
		wtt.rescheduleJob(timeout)
	}

	return nil
}

// rescheduleJob reschedules a cyclic job for the next execution
func (wtt *WheelTimerTask) rescheduleJob(timeout *job.Timeout) {
	if wtt.job.DefaultInterval <= 0 {
		wtt.logger.Info("job has no valid interval, not rescheduling",
			"jobID", wtt.job.ID)
		return
	}

	// Use the stats recorder to reschedule the job
	if wtt.statsRecorder != nil {
		if err := wtt.statsRecorder.RescheduleJob(wtt.job); err != nil {
			wtt.logger.Error(err, "failed to reschedule job", "jobID", wtt.job.ID)
		}
	} else {
		wtt.logger.Error(nil, "stats recorder is nil", "jobID", wtt.job.ID)
	}
}

// GetJob returns the associated job
func (wtt *WheelTimerTask) GetJob() *job.Job {
	return wtt.job
}
