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
	"fmt"
	"time"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// WheelTimerTask represents a task that runs in the time wheel
// This corresponds to Java's WheelTimerTask
type WheelTimerTask struct {
	job              *jobtypes.Job
	commonDispatcher MetricsTaskDispatcher
	logger           logger.Logger
}

// NewWheelTimerTask creates a new wheel timer task
func NewWheelTimerTask(job *jobtypes.Job, commonDispatcher MetricsTaskDispatcher, logger logger.Logger) *WheelTimerTask {
	return &WheelTimerTask{
		job:              job,
		commonDispatcher: commonDispatcher,
		logger:           logger.WithName("wheel-timer-task"),
	}
}

// Run executes the timer task
// This corresponds to Java's WheelTimerTask.run method
func (wtt *WheelTimerTask) Run(timeout *jobtypes.Timeout) error {
	if wtt.job == nil {
		wtt.logger.Error(nil, "job is nil, cannot execute")
		return fmt.Errorf("job is nil, cannot execute")
	}

	wtt.logger.Info("executing wheel timer task",
		"jobID", wtt.job.ID,
		"monitorID", wtt.job.MonitorID,
		"app", wtt.job.App)

	startTime := time.Now()

	// Dispatch metrics task through common dispatcher
	// This is where the job gets broken down into individual metric collection tasks
	err := wtt.commonDispatcher.DispatchMetricsTask(wtt.job, timeout)

	duration := time.Since(startTime)

	if err != nil {
		wtt.logger.Error(err, "failed to dispatch metrics task",
			"jobID", wtt.job.ID,
			"duration", duration)
		return err
	}

	wtt.logger.Info("successfully dispatched metrics task",
		"jobID", wtt.job.ID,
		"duration", duration)

	// If this is a cyclic job, reschedule it for the next execution
	if wtt.job.IsCyclic && wtt.job.DefaultInterval > 0 {
		wtt.rescheduleJob(timeout)
	}

	return nil
}

// rescheduleJob reschedules a cyclic job for the next execution
func (wtt *WheelTimerTask) rescheduleJob(timeout *jobtypes.Timeout) {
	wtt.logger.Info("rescheduling cyclic job",
		"jobID", wtt.job.ID,
		"interval", wtt.job.DefaultInterval)

	// TODO: Implement rescheduling logic
	// This would involve creating a new timeout with the same job
	// and adding it back to the time wheel
	// For now, we'll log that rescheduling is needed
	wtt.logger.Info("cyclic job needs rescheduling",
		"jobID", wtt.job.ID,
		"nextExecutionIn", time.Duration(wtt.job.DefaultInterval)*time.Second)
}

// GetJob returns the associated job
func (wtt *WheelTimerTask) GetJob() *jobtypes.Job {
	return wtt.job
}
