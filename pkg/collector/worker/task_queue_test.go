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
	"os"
	"testing"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

func TestTaskQueue_BasicOperations(t *testing.T) {
	queue := NewTaskQueue(logger.DefaultLogger(os.Stdout, "debug"))
	defer queue.Close()

	if !queue.IsEmpty() {
		t.Error("New queue should be empty")
	}

	if queue.Size() != 0 {
		t.Error("New queue size should be 0")
	}

	// Push a task
	task := newMockTask(1, 1*time.Second)
	queue.Push(task)

	if queue.IsEmpty() {
		t.Error("Queue should not be empty after push")
	}

	if queue.Size() != 1 {
		t.Errorf("Queue size should be 1, got %d", queue.Size())
	}

	// Pop the task
	poppedTask := queue.Pop()
	if poppedTask != task {
		t.Error("Popped task should be the same as pushed task")
	}

	if !queue.IsEmpty() {
		t.Error("Queue should be empty after pop")
	}
}

func TestTaskQueue_PriorityOrdering(t *testing.T) {
	queue := NewTaskQueue(logger.DefaultLogger(os.Stdout, "debug"))
	defer queue.Close()

	// Push tasks with different priorities
	lowPriority := newMockTask(-1, 1*time.Second)
	normalPriority := newMockTask(0, 1*time.Second)
	highPriority := newMockTask(1, 1*time.Second)

	// Push in reverse priority order
	queue.Push(lowPriority)
	queue.Push(normalPriority)
	queue.Push(highPriority)

	// Pop should return highest priority first
	task1 := queue.Pop()
	if task1.Priority() != 1 {
		t.Errorf("First popped task should have priority 1, got %d", task1.Priority())
	}

	task2 := queue.Pop()
	if task2.Priority() != 0 {
		t.Errorf("Second popped task should have priority 0, got %d", task2.Priority())
	}

	task3 := queue.Pop()
	if task3.Priority() != -1 {
		t.Errorf("Third popped task should have priority -1, got %d", task3.Priority())
	}
}

func TestTaskQueue_PopBlocking(t *testing.T) {
	queue := NewTaskQueue(logger.DefaultLogger(os.Stdout, "debug"))
	defer queue.Close()

	task := newMockTask(1, 1*time.Second)

	// Start a goroutine to pop blocking
	done := make(chan Task, 1)
	go func() {
		poppedTask := queue.PopBlocking()
		done <- poppedTask
	}()

	// Wait a bit then push a task
	time.Sleep(100 * time.Millisecond)
	queue.Push(task)

	// Should receive the task
	select {
	case poppedTask := <-done:
		if poppedTask != task {
			t.Error("Popped task should be the same as pushed task")
		}
	case <-time.After(1 * time.Second):
		t.Error("PopBlocking should have returned a task")
	}
}

func TestTaskQueue_PopWithTimeout(t *testing.T) {
	queue := NewTaskQueue(logger.DefaultLogger(os.Stdout, "debug"))
	defer queue.Close()

	// Test timeout with no tasks
	task := queue.PopWithTimeout(100 * time.Millisecond)
	if task != nil {
		t.Error("PopWithTimeout should return nil when no tasks available")
	}

	// Test with task available
	mockTask := newMockTask(1, 1*time.Second)
	queue.Push(mockTask)

	task = queue.PopWithTimeout(100 * time.Millisecond)
	if task != mockTask {
		t.Error("PopWithTimeout should return the available task")
	}
}

func TestTaskQueue_Stats(t *testing.T) {
	queue := NewTaskQueue(logger.DefaultLogger(os.Stdout, "debug"))
	defer queue.Close()

	// Initial stats
	stats := queue.GetStats()
	if stats.QueueSize != 0 {
		t.Errorf("Initial queue size should be 0, got %d", stats.QueueSize)
	}

	// Add tasks with different priorities
	queue.Push(newMockTask(1, 1*time.Second))  // high
	queue.Push(newMockTask(0, 1*time.Second))  // normal
	queue.Push(newMockTask(-1, 1*time.Second)) // low

	stats = queue.GetStats()
	if stats.QueueSize != 3 {
		t.Errorf("Queue size should be 3, got %d", stats.QueueSize)
	}

	if stats.HighPriority != 1 {
		t.Errorf("High priority count should be 1, got %d", stats.HighPriority)
	}

	if stats.NormalPriority != 1 {
		t.Errorf("Normal priority count should be 1, got %d", stats.NormalPriority)
	}

	if stats.LowPriority != 1 {
		t.Errorf("Low priority count should be 1, got %d", stats.LowPriority)
	}
}

func TestTaskQueue_Clear(t *testing.T) {
	queue := NewTaskQueue(logger.DefaultLogger(os.Stdout, "debug"))
	defer queue.Close()

	// Add some tasks
	queue.Push(newMockTask(1, 1*time.Second))
	queue.Push(newMockTask(2, 1*time.Second))
	queue.Push(newMockTask(3, 1*time.Second))

	if queue.Size() != 3 {
		t.Errorf("Queue size should be 3, got %d", queue.Size())
	}

	// Clear the queue
	queue.Clear()

	if !queue.IsEmpty() {
		t.Error("Queue should be empty after clear")
	}

	if queue.Size() != 0 {
		t.Errorf("Queue size should be 0 after clear, got %d", queue.Size())
	}
}

func TestPriorityWorkerPool_Integration(t *testing.T) {
	config := WorkerPoolConfig{
		CoreSize:    2,
		MaxSize:     4,
		IdleTimeout: 1 * time.Second,
		QueueSize:   10,
	}

	pwp := NewPriorityWorkerPool(config, logger.DefaultLogger(os.Stdout, "debug"))

	if err := pwp.Start(); err != nil {
		t.Fatalf("Failed to start priority worker pool: %v", err)
	}
	defer pwp.Stop()

	// Submit tasks with different priorities
	highTask := newMockTask(2, 100*time.Millisecond)
	normalTask := newMockTask(0, 100*time.Millisecond)
	lowTask := newMockTask(-1, 100*time.Millisecond)

	// Submit in reverse priority order
	if err := pwp.Submit(lowTask); err != nil {
		t.Fatalf("Failed to submit low priority task: %v", err)
	}

	if err := pwp.Submit(normalTask); err != nil {
		t.Fatalf("Failed to submit normal priority task: %v", err)
	}

	if err := pwp.Submit(highTask); err != nil {
		t.Fatalf("Failed to submit high priority task: %v", err)
	}

	// Wait for all tasks to complete
	time.Sleep(1 * time.Second)

	// All tasks should be executed
	if !highTask.executed.Load() {
		t.Error("High priority task should be executed")
	}

	if !normalTask.executed.Load() {
		t.Error("Normal priority task should be executed")
	}

	if !lowTask.executed.Load() {
		t.Error("Low priority task should be executed")
	}

	// Check stats
	workerStats, queueStats := pwp.GetStats()
	t.Logf("Worker stats: %+v", workerStats)
	t.Logf("Queue stats: %+v", queueStats)

	if workerStats.CompletedTasks < 3 {
		t.Errorf("Expected at least 3 completed tasks, got %d", workerStats.CompletedTasks)
	}
}
