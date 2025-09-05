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
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// Task represents a task that can be executed by the worker pool
type Task interface {
	Execute() error
	Priority() int // Higher number = higher priority
	Timeout() time.Duration
}

// WorkerPool manages a pool of goroutines to execute collection tasks
type WorkerPool struct {
	// Configuration
	coreSize    int
	maxSize     int
	idleTimeout time.Duration

	// Internal state
	workers     sync.Map     // map[int]*worker
	taskQueue   chan Task    // buffered channel for tasks
	workerCount atomic.Int64 // current number of workers
	workerID    atomic.Int64 // counter for assigning worker IDs

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats WorkerPoolStats

	logger logger.Logger
}

// WorkerPoolConfig contains configuration for the worker pool
type WorkerPoolConfig struct {
	CoreSize    int           // Core number of workers (always running)
	MaxSize     int           // Maximum number of workers
	IdleTimeout time.Duration // Time after which idle workers are terminated
	QueueSize   int           // Size of the task queue buffer
}

// WorkerPoolStats contains statistics about the worker pool
type WorkerPoolStats struct {
	ActiveWorkers  int64 `json:"activeWorkers"`
	QueuedTasks    int64 `json:"queuedTasks"`
	CompletedTasks int64 `json:"completedTasks"`
	RejectedTasks  int64 `json:"rejectedTasks"`
	TotalSubmitted int64 `json:"totalSubmitted"`
}

// worker represents a single worker goroutine
type worker struct {
	id       int64
	pool     *WorkerPool
	lastUsed time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

// DefaultWorkerPoolConfig returns a default configuration
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	coreSize := max(2, runtime.NumCPU())
	maxSize := runtime.NumCPU() * 16

	return WorkerPoolConfig{
		CoreSize:    coreSize,
		MaxSize:     maxSize,
		IdleTimeout: 10 * time.Second,
		QueueSize:   1000,
	}
}

// NewWorkerPool creates a new worker pool with the given configuration
func NewWorkerPool(config WorkerPoolConfig, logger logger.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		coreSize:    config.CoreSize,
		maxSize:     config.MaxSize,
		idleTimeout: config.IdleTimeout,
		taskQueue:   make(chan Task, config.QueueSize),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger.WithName("worker-pool"),
	}

	return pool
}

// Start starts the worker pool
func (wp *WorkerPool) Start() error {
	wp.logger.Info("starting worker pool",
		"coreSize", wp.coreSize,
		"maxSize", wp.maxSize,
		"idleTimeout", wp.idleTimeout)

	// Start core workers
	for i := 0; i < wp.coreSize; i++ {
		wp.startWorker(true) // core workers
	}

	// Start the manager goroutine
	wp.wg.Add(1)
	go wp.manage()

	wp.logger.Info("worker pool started successfully")
	return nil
}

// Stop stops the worker pool and waits for all workers to finish
func (wp *WorkerPool) Stop() error {
	wp.logger.Info("stopping worker pool")

	// Cancel all workers
	wp.cancel()

	// Close task queue to prevent new submissions
	close(wp.taskQueue)

	// Wait for all workers to finish
	wp.wg.Wait()

	wp.logger.Info("worker pool stopped successfully")
	return nil
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(task Task) error {
	atomic.AddInt64(&wp.stats.TotalSubmitted, 1)

	select {
	case wp.taskQueue <- task:
		atomic.AddInt64(&wp.stats.QueuedTasks, 1)

		// Try to start a new worker if queue is getting full and we haven't reached max
		if wp.shouldStartNewWorker() {
			wp.startWorker(false) // non-core worker
		}

		return nil
	case <-wp.ctx.Done():
		atomic.AddInt64(&wp.stats.RejectedTasks, 1)
		return fmt.Errorf("worker pool is shutting down")
	default:
		// Queue is full, reject the task
		atomic.AddInt64(&wp.stats.RejectedTasks, 1)
		return fmt.Errorf("task queue is full, rejecting task")
	}
}

// shouldStartNewWorker determines if a new worker should be started
func (wp *WorkerPool) shouldStartNewWorker() bool {
	currentWorkers := wp.workerCount.Load()
	queuedTasks := atomic.LoadInt64(&wp.stats.QueuedTasks)

	// Start new worker if:
	// 1. We have queued tasks
	// 2. We haven't reached max workers
	// 3. Queue is more than half full
	return queuedTasks > 0 &&
		currentWorkers < int64(wp.maxSize) &&
		queuedTasks > int64(cap(wp.taskQueue)/2)
}

// startWorker starts a new worker
func (wp *WorkerPool) startWorker(isCore bool) {
	workerID := wp.workerID.Add(1)
	ctx, cancel := context.WithCancel(wp.ctx)

	w := &worker{
		id:       workerID,
		pool:     wp,
		lastUsed: time.Now(),
		ctx:      ctx,
		cancel:   cancel,
	}

	wp.workers.Store(workerID, w)
	wp.workerCount.Add(1)
	atomic.AddInt64(&wp.stats.ActiveWorkers, 1)

	wp.wg.Add(1)
	go w.run(isCore)

	wp.logger.Info("started new worker", "workerID", workerID, "isCore", isCore)
}

// manage manages the worker pool (cleanup idle workers, etc.)
func (wp *WorkerPool) manage() {
	defer wp.wg.Done()

	ticker := time.NewTicker(wp.idleTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			wp.cleanupIdleWorkers()
		}
	}
}

// cleanupIdleWorkers removes idle non-core workers
func (wp *WorkerPool) cleanupIdleWorkers() {
	now := time.Now()
	currentWorkers := wp.workerCount.Load()

	// Only cleanup if we have more than core workers
	if currentWorkers <= int64(wp.coreSize) {
		return
	}

	toRemove := make([]int64, 0)

	wp.workers.Range(func(key, value interface{}) bool {
		workerID := key.(int64)
		w := value.(*worker)

		// Don't remove core workers (first coreSize workers)
		if workerID <= int64(wp.coreSize) {
			return true
		}

		// Check if worker is idle
		if now.Sub(w.lastUsed) > wp.idleTimeout {
			toRemove = append(toRemove, workerID)
		}

		return true
	})

	// Remove idle workers
	for _, workerID := range toRemove {
		if w, exists := wp.workers.LoadAndDelete(workerID); exists {
			worker := w.(*worker)
			worker.cancel()
			wp.logger.Info("removed idle worker", "workerID", workerID)
		}
	}
}

// GetStats returns current statistics
func (wp *WorkerPool) GetStats() WorkerPoolStats {
	stats := wp.stats
	stats.ActiveWorkers = wp.workerCount.Load()
	stats.QueuedTasks = int64(len(wp.taskQueue))
	return stats
}

// run is the main loop for a worker
func (w *worker) run(isCore bool) {
	defer func() {
		w.pool.wg.Done()
		w.pool.workerCount.Add(-1)
		atomic.AddInt64(&w.pool.stats.ActiveWorkers, -1)
		w.pool.workers.Delete(w.id)

		if r := recover(); r != nil {
			w.pool.logger.Info("worker panic recovered", "workerID", w.id, "error", r)
		}

		w.pool.logger.Info("worker stopped", "workerID", w.id, "isCore", isCore)
	}()

	w.pool.logger.Info("worker started", "workerID", w.id, "isCore", isCore)

	for {
		select {
		case <-w.ctx.Done():
			return
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				// Task queue is closed
				return
			}

			w.lastUsed = time.Now()
			atomic.AddInt64(&w.pool.stats.QueuedTasks, -1)

			// Execute task with timeout
			w.executeTask(task)

			atomic.AddInt64(&w.pool.stats.CompletedTasks, 1)
		}
	}
}

// executeTask executes a single task with timeout handling
func (w *worker) executeTask(task Task) {
	defer func() {
		if r := recover(); r != nil {
			w.pool.logger.Info("task execution panic recovered",
				"workerID", w.id,
				"error", r)
		}
	}()

	// Create timeout context for the task
	timeout := task.Timeout()
	if timeout <= 0 {
		timeout = 60 * time.Second // default timeout
	}

	ctx, cancel := context.WithTimeout(w.ctx, timeout)
	defer cancel()

	// Execute task in a goroutine to handle timeout
	done := make(chan error, 1)
	go func() {
		done <- task.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			w.pool.logger.Info("task execution failed",
				"workerID", w.id,
				"error", err)
		}
	case <-ctx.Done():
		w.pool.logger.Info("task execution timeout",
			"workerID", w.id,
			"timeout", timeout)
	}
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
