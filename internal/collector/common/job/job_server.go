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

package job

import (
	"context"
	"fmt"
	"sync"

	clrServer "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/collector"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
)

// TimeDispatcher interface defines time-based job scheduling
type TimeDispatcher interface {
	AddJob(job *jobtypes.Job) error
	RemoveJob(jobID int64) error
	Start(ctx context.Context) error
	Stop() error
}

// Config represents job service configuration
type Config struct {
	clrServer.Server
}

// Runner implements the service runner interface
// It directly manages job scheduling through timeDispatch
type Runner struct {
	Config
	timeDispatch TimeDispatcher
	mu           sync.RWMutex
	runningJobs  map[int64]*jobtypes.Job
	ctx          context.Context
	cancel       context.CancelFunc
}

// AddAsyncCollectJob adds a job to async collection scheduling
// This is the main entry point that corresponds to Java's collectJobService.addAsyncCollectJob
func (r *Runner) AddAsyncCollectJob(job *jobtypes.Job) error {
	if job == nil {
		r.Logger.Error(nil, "job cannot be nil")
		return fmt.Errorf("job cannot be nil")
	}

	r.Logger.Info("adding async collect job",
		"jobID", job.ID,
		"monitorID", job.MonitorID,
		"app", job.App,
		"isCyclic", job.IsCyclic)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Store job in running jobs map
	r.runningJobs[job.ID] = job

	// Add job to time dispatcher for scheduling
	if err := r.timeDispatch.AddJob(job); err != nil {
		delete(r.runningJobs, job.ID)
		r.Logger.Error(err, "failed to add job to time dispatcher", "jobID", job.ID)
		return fmt.Errorf("failed to add job to time dispatcher: %w", err)
	}

	r.Logger.Info("successfully added job to scheduler", "jobID", job.ID)
	return nil
}

// RemoveAsyncCollectJob removes a job from scheduling
func (r *Runner) RemoveAsyncCollectJob(jobID int64) error {
	r.Logger.Info("removing async collect job", "jobID", jobID)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from running jobs
	delete(r.runningJobs, jobID)

	// Remove from time dispatcher
	if err := r.timeDispatch.RemoveJob(jobID); err != nil {
		r.Logger.Error(err, "failed to remove job from time dispatcher", "jobID", jobID)
		return fmt.Errorf("failed to remove job from time dispatcher: %w", err)
	}

	r.Logger.Info("successfully removed job from scheduler", "jobID", jobID)
	return nil
}

// RunningJobs returns a copy of currently running jobs
func (r *Runner) RunningJobs() map[int64]*jobtypes.Job {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[int64]*jobtypes.Job)
	for id, job := range r.runningJobs {
		result[id] = job
	}
	return result
}

// New creates a new job service runner
func New(srv *Config) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		Config:      *srv,
		runningJobs: make(map[int64]*jobtypes.Job),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetTimeDispatcher sets the time dispatcher for this runner
// This should be called before starting the runner
func (r *Runner) SetTimeDispatcher(td TimeDispatcher) {
	r.timeDispatch = td
}

// Start starts the job service runner
func (r *Runner) Start(ctx context.Context) error {
	r.Logger = r.Logger.WithName(r.Info().Name).WithValues("runner", r.Info().Name)
	r.Logger.Info("Starting job service runner")

	// Start the time dispatcher if it's set
	if r.timeDispatch != nil {
		if err := r.timeDispatch.Start(ctx); err != nil {
			r.Logger.Error(err, "failed to start time dispatcher")
			return fmt.Errorf("failed to start time dispatcher: %w", err)
		}
	}

	r.Logger.Info("job service runner started successfully")

	select {
	case <-ctx.Done():
		r.Logger.Info("job service runner stopped by context")
		if r.timeDispatch != nil {
			if err := r.timeDispatch.Stop(); err != nil {
				r.Logger.Error(err, "error stopping time dispatcher")
			}
		}
		return nil
	}
}

// Info returns runner information
func (r *Runner) Info() collector.Info {
	return collector.Info{
		Name: "job-service",
	}
}

// Close closes the job service runner
func (r *Runner) Close() error {
	r.Logger.Info("closing job service runner")

	r.cancel()

	if r.timeDispatch != nil {
		if err := r.timeDispatch.Stop(); err != nil {
			r.Logger.Error(err, "error stopping time dispatcher")
			return err
		}
	}

	// Clear running jobs
	r.mu.Lock()
	r.runningJobs = make(map[int64]*jobtypes.Job)
	r.mu.Unlock()

	r.Logger.Info("job service runner closed")
	return nil
}
