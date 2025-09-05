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

//import (
//	"os"
//	"sync/atomic"
//	"testing"
//	"time"
//
//	"hertzbeat.apache.org/hertzbeat-collector-go/internal/logger"
//)
//
//// mockTask implements Task interface for testing
//type mockTask struct {
//	executed  atomic.Bool
//	execution chan struct{}
//	priority  int
//	timeout   time.Duration
//	err       error
//}
//
//func newMockTask(priority int, timeout time.Duration) *mockTask {
//	return &mockTask{
//		execution: make(chan struct{}, 1),
//		priority:  priority,
//		timeout:   timeout,
//	}
//}
//
//func (mt *mockTask) Execute() error {
//	mt.executed.Store(true)
//	select {
//	case mt.execution <- struct{}{}:
//	default:
//	}
//	return mt.err
//}
//
//func (mt *mockTask) Priority() int {
//	return mt.priority
//}
//
//func (mt *mockTask) Timeout() time.Duration {
//	return mt.timeout
//}
//
//func (mt *mockTask) waitForExecution(timeout time.Duration) bool {
//	select {
//	case <-mt.execution:
//		return true
//	case <-time.After(timeout):
//		return false
//	}
//}
//
//func TestWorkerPool_Basic(t *testing.T) {
//	config := WorkerPoolConfig{
//		CoreSize:    2,
//		MaxSize:     4,
//		IdleTimeout: 1 * time.Second,
//		QueueSize:   10,
//	}
//
//	pool := NewWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))
//
//	if err := pool.Start(); err != nil {
//		t.Fatalf("Failed to start worker pool: %v", err)
//	}
//	defer pool.Stop()
//
//	// Submit a simple task
//	task := newMockTask(1, 1*time.Second)
//	if err := pool.Submit(task); err != nil {
//		t.Fatalf("Failed to submit task: %v", err)
//	}
//
//	// Wait for task execution
//	if !task.waitForExecution(3 * time.Second) {
//		t.Error("Task was not executed within timeout")
//	}
//
//	if !task.executed.Load() {
//		t.Error("Task executed flag not set")
//	}
//}
//
//func TestWorkerPool_MultipleTasksOrdering(t *testing.T) {
//	config := WorkerPoolConfig{
//		CoreSize:    1, // Use single core to ensure ordering
//		MaxSize:     1,
//		IdleTimeout: 1 * time.Second,
//		QueueSize:   10,
//	}
//
//	pool := NewWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))
//
//	if err := pool.Start(); err != nil {
//		t.Fatalf("Failed to start worker pool: %v", err)
//	}
//	defer pool.Stop()
//
//	// Submit multiple tasks
//	task1 := newMockTask(1, 1*time.Second)
//	task2 := newMockTask(1, 1*time.Second)
//	task3 := newMockTask(1, 1*time.Second)
//
//	pool.Submit(task1)
//	pool.Submit(task2)
//	pool.Submit(task3)
//
//	// Wait for all tasks to complete
//	if !task1.waitForExecution(3*time.Second) ||
//		!task2.waitForExecution(3*time.Second) ||
//		!task3.waitForExecution(3*time.Second) {
//		t.Error("Not all tasks were executed")
//	}
//}
//
//func TestWorkerPool_QueueFull(t *testing.T) {
//	config := WorkerPoolConfig{
//		CoreSize:    1,
//		MaxSize:     1,
//		IdleTimeout: 1 * time.Second,
//		QueueSize:   1, // Very small queue
//	}
//
//	pool := NewWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))
//
//	if err := pool.Start(); err != nil {
//		t.Fatalf("Failed to start worker pool: %v", err)
//	}
//	defer pool.Stop()
//
//	// Wait for worker to be ready
//	time.Sleep(10 * time.Millisecond)
//
//	// Create many tasks to overflow the queue
//	tasks := make([]*mockTask, 10)
//	successCount := 0
//
//	for i := 0; i < 10; i++ {
//		tasks[i] = newMockTask(1, 10*time.Millisecond)
//		if err := pool.Submit(tasks[i]); err == nil {
//			successCount++
//		}
//	}
//
//	// With queue size 1 and 1 worker, we should be able to submit at most 2 tasks
//	// (1 executing + 1 queued) before getting rejections
//	if successCount > 3 {
//		t.Errorf("Expected at most 3 successful submissions, got %d", successCount)
//	}
//
//	if successCount < 1 {
//		t.Error("Should have at least 1 successful submission")
//	}
//
//	t.Logf("Successfully submitted %d out of 10 tasks", successCount)
//}
//
//func TestWorkerPool_Stats(t *testing.T) {
//	config := WorkerPoolConfig{
//		CoreSize:    2,
//		MaxSize:     4,
//		IdleTimeout: 1 * time.Second,
//		QueueSize:   10,
//	}
//
//	pool := NewWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))
//
//	if err := pool.Start(); err != nil {
//		t.Fatalf("Failed to start worker pool: %v", err)
//	}
//	defer pool.Stop()
//
//	// Wait for core workers to start
//	time.Sleep(100 * time.Millisecond)
//
//	stats := pool.GetStats()
//
//	if stats.ActiveWorkers < 2 {
//		t.Errorf("Expected at least 2 active workers, got %d", stats.ActiveWorkers)
//	}
//
//	if stats.TotalSubmitted != 0 {
//		t.Errorf("Expected 0 total submitted, got %d", stats.TotalSubmitted)
//	}
//
//	// Submit a task and check stats
//	task := newMockTask(1, 1*time.Second)
//	pool.Submit(task)
//
//	stats = pool.GetStats()
//	if stats.TotalSubmitted != 1 {
//		t.Errorf("Expected 1 total submitted, got %d", stats.TotalSubmitted)
//	}
//}
//
//func TestWorkerPool_StopAndStart(t *testing.T) {
//	config := DefaultWorkerPoolConfig()
//	pool := NewWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))
//
//	// Test start
//	if err := pool.Start(); err != nil {
//		t.Fatalf("Failed to start worker pool: %v", err)
//	}
//
//	// Submit a task
//	task := newMockTask(1, 1*time.Second)
//	if err := pool.Submit(task); err != nil {
//		t.Errorf("Failed to submit task: %v", err)
//	}
//
//	// Test stop
//	if err := pool.Stop(); err != nil {
//		t.Fatalf("Failed to stop worker pool: %v", err)
//	}
//
//	// Task should complete even after stop
//	if !task.waitForExecution(3 * time.Second) {
//		t.Error("Task should complete even after pool stop")
//	}
//}
//
//func TestWorkerPool_HighLoad(t *testing.T) {
//	if testing.Short() {
//		t.Skip("Skipping high load test in short mode")
//	}
//
//	config := WorkerPoolConfig{
//		CoreSize:    4,
//		MaxSize:     8,
//		IdleTimeout: 1 * time.Second,
//		QueueSize:   100,
//	}
//
//	pool := NewWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))
//
//	if err := pool.Start(); err != nil {
//		t.Fatalf("Failed to start worker pool: %v", err)
//	}
//	defer pool.Stop()
//
//	const numTasks = 50
//	tasks := make([]*mockTask, numTasks)
//
//	// Submit many tasks
//	for i := 0; i < numTasks; i++ {
//		tasks[i] = newMockTask(1, 100*time.Millisecond)
//		if err := pool.Submit(tasks[i]); err != nil {
//			t.Fatalf("Failed to submit task %d: %v", i, err)
//		}
//	}
//
//	// Wait for all tasks to complete
//	time.Sleep(3 * time.Second)
//
//	// Check how many tasks executed
//	executedCount := 0
//	for _, task := range tasks {
//		if task.executed.Load() {
//			executedCount++
//		}
//	}
//
//	// We expect most tasks to execute
//	expectedMin := numTasks * 90 / 100 // 90% execution rate minimum
//	if executedCount < expectedMin {
//		t.Errorf("Expected at least %d tasks to execute, got %d", expectedMin, executedCount)
//	}
//
//	t.Logf("Executed %d out of %d tasks (%.1f%%)",
//		executedCount, numTasks, float64(executedCount)*100/float64(numTasks))
//
//	// Check final stats
//	stats := pool.GetStats()
//	t.Logf("Final stats: %+v", stats)
//}
