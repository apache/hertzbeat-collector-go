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
	"container/heap"
	"context"
	"sync"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// TaskQueue manages a priority queue of collection tasks
type TaskQueue struct {
	heap   *taskHeap
	mutex  sync.RWMutex
	notify chan struct{}

	// Control
	ctx    context.Context
	cancel context.CancelFunc

	logger logger.Logger
}

// taskHeap implements a priority queue for tasks
type taskHeap struct {
	tasks []Task
}

// TaskQueueStats contains statistics about the task queue
type TaskQueueStats struct {
	QueueSize      int `json:"queueSize"`
	HighPriority   int `json:"highPriority"`
	NormalPriority int `json:"normalPriority"`
	LowPriority    int `json:"lowPriority"`
}

// NewTaskQueue creates a new task queue
func NewTaskQueue(logger logger.Logger) *TaskQueue {
	ctx, cancel := context.WithCancel(context.Background())

	return &TaskQueue{
		heap:   &taskHeap{tasks: make([]Task, 0)},
		notify: make(chan struct{}, 1),
		ctx:    ctx,
		cancel: cancel,
		logger: logger.WithName("task-queue"),
	}
}

// Push adds a task to the queue
func (tq *TaskQueue) Push(task Task) {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	heap.Push(tq.heap, task)

	// Notify waiting consumers
	select {
	case tq.notify <- struct{}{}:
	default:
	}

	tq.logger.Info("task added to queue",
		"priority", task.Priority(),
		"queueSize", tq.heap.Len())
}

// Pop removes and returns the highest priority task
func (tq *TaskQueue) Pop() Task {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	if tq.heap.Len() == 0 {
		return nil
	}

	task := heap.Pop(tq.heap).(Task)

	tq.logger.Info("task removed from queue",
		"priority", task.Priority(),
		"queueSize", tq.heap.Len())

	return task
}

// PopWithTimeout waits for a task or timeout
func (tq *TaskQueue) PopWithTimeout(timeout time.Duration) Task {
	// Try to get a task immediately
	if task := tq.Pop(); task != nil {
		return task
	}

	// Wait for notification or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-tq.notify:
		return tq.Pop()
	case <-timer.C:
		return nil
	case <-tq.ctx.Done():
		return nil
	}
}

// PopBlocking waits for a task to become available
func (tq *TaskQueue) PopBlocking() Task {
	for {
		// Try to get a task
		if task := tq.Pop(); task != nil {
			return task
		}

		// Wait for notification
		select {
		case <-tq.notify:
			// Try again
		case <-tq.ctx.Done():
			return nil
		}
	}
}

// Size returns the current queue size
func (tq *TaskQueue) Size() int {
	tq.mutex.RLock()
	defer tq.mutex.RUnlock()

	return tq.heap.Len()
}

// IsEmpty returns true if the queue is empty
func (tq *TaskQueue) IsEmpty() bool {
	return tq.Size() == 0
}

// Clear removes all tasks from the queue
func (tq *TaskQueue) Clear() {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	tq.heap.tasks = tq.heap.tasks[:0]

	tq.logger.Info("task queue cleared")
}

// GetStats returns queue statistics
func (tq *TaskQueue) GetStats() TaskQueueStats {
	tq.mutex.RLock()
	defer tq.mutex.RUnlock()

	stats := TaskQueueStats{
		QueueSize: tq.heap.Len(),
	}

	// Count priorities
	for _, task := range tq.heap.tasks {
		priority := task.Priority()
		switch {
		case priority > 0:
			stats.HighPriority++
		case priority == 0:
			stats.NormalPriority++
		default:
			stats.LowPriority++
		}
	}

	return stats
}

// Close closes the task queue
func (tq *TaskQueue) Close() {
	tq.cancel()
	close(tq.notify)
	tq.logger.Info("task queue closed")
}

// Heap interface implementation for taskHeap

func (h taskHeap) Len() int {
	return len(h.tasks)
}

func (h taskHeap) Less(i, j int) bool {
	// Higher priority first (max heap)
	return h.tasks[i].Priority() > h.tasks[j].Priority()
}

func (h taskHeap) Swap(i, j int) {
	h.tasks[i], h.tasks[j] = h.tasks[j], h.tasks[i]
}

func (h *taskHeap) Push(x interface{}) {
	h.tasks = append(h.tasks, x.(Task))
}

func (h *taskHeap) Pop() interface{} {
	old := h.tasks
	n := len(old)
	task := old[n-1]
	h.tasks = old[0 : n-1]
	return task
}

// PriorityWorkerPool combines WorkerPool with TaskQueue for priority-based task execution
type PriorityWorkerPool struct {
	workerPool *WorkerPool
	taskQueue  *TaskQueue

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger logger.Logger
}

// NewPriorityWorkerPool creates a new priority-based worker pool
func NewPriorityWorkerPool(config WorkerPoolConfig, logger logger.Logger) *PriorityWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &PriorityWorkerPool{
		workerPool: NewWorkerPool(config, logger),
		taskQueue:  NewTaskQueue(logger),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger.WithName("priority-worker-pool"),
	}
}

// Start starts the priority worker pool
func (pwp *PriorityWorkerPool) Start() error {
	pwp.logger.Info("starting priority worker pool")

	// Start the worker pool
	if err := pwp.workerPool.Start(); err != nil {
		return err
	}

	// Start the dispatcher goroutine
	pwp.wg.Add(1)
	go pwp.dispatch()

	pwp.logger.Info("priority worker pool started successfully")
	return nil
}

// Stop stops the priority worker pool
func (pwp *PriorityWorkerPool) Stop() error {
	pwp.logger.Info("stopping priority worker pool")

	// Cancel context
	pwp.cancel()

	// Close task queue
	pwp.taskQueue.Close()

	// Wait for dispatcher to finish
	pwp.wg.Wait()

	// Stop worker pool
	if err := pwp.workerPool.Stop(); err != nil {
		return err
	}

	pwp.logger.Info("priority worker pool stopped successfully")
	return nil
}

// Submit submits a task to the priority queue
func (pwp *PriorityWorkerPool) Submit(task Task) error {
	pwp.taskQueue.Push(task)
	return nil
}

// dispatch continuously moves tasks from priority queue to worker pool
func (pwp *PriorityWorkerPool) dispatch() {
	defer pwp.wg.Done()

	pwp.logger.Info("priority dispatcher started")
	defer pwp.logger.Info("priority dispatcher stopped")

	for {
		select {
		case <-pwp.ctx.Done():
			return
		default:
			// Get highest priority task
			task := pwp.taskQueue.PopWithTimeout(1 * time.Second)
			if task == nil {
				continue
			}

			// Submit to worker pool
			if err := pwp.workerPool.Submit(task); err != nil {
				pwp.logger.Info("failed to submit task to worker pool", "error", err)
				// Put task back in queue for retry
				pwp.taskQueue.Push(task)

				// Wait a bit before retrying
				select {
				case <-time.After(100 * time.Millisecond):
				case <-pwp.ctx.Done():
					return
				}
			}
		}
	}
}

// GetStats returns combined statistics
func (pwp *PriorityWorkerPool) GetStats() (WorkerPoolStats, TaskQueueStats) {
	return pwp.workerPool.GetStats(), pwp.taskQueue.GetStats()
}
