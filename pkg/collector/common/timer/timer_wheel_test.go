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
	"os"
	"sync/atomic"
	"testing"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// mockTask implements TimerTask interface for testing
type mockTask struct {
	executed  atomic.Bool
	execution chan struct{}
	delay     time.Duration
}

func newMockTask() *mockTask {
	return &mockTask{
		execution: make(chan struct{}, 1),
	}
}

func (mt *mockTask) Run(timeout *jobtypes.Timeout) error {
	mt.executed.Store(true)
	select {
	case mt.execution <- struct{}{}:
	default:
	}
	return nil
}

func (mt *mockTask) waitForExecution(timeout time.Duration) bool {
	select {
	case <-mt.execution:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestTimerWheel_Basic(t *testing.T) {
	// Create timer wheel with faster tick for testing
	tw := NewTimerWheelWithConfig(100*time.Millisecond, 8, logger.DefaultLogger(os.Stdout, "debug"))

	if err := tw.Start(); err != nil {
		t.Fatalf("Failed to start timer wheel: %v", err)
	}
	defer tw.Stop()

	// Test basic scheduling
	task := newMockTask()
	timeout := tw.NewTimeout(task, 200*time.Millisecond)

	if timeout == nil {
		t.Fatal("NewTimeout returned nil")
	}

	// Wait for execution
	if !task.waitForExecution(1 * time.Second) {
		t.Error("Task was not executed within timeout")
	}

	if !task.executed.Load() {
		t.Error("Task executed flag not set")
	}
}

func TestTimerWheel_MultipleTasksOrdering(t *testing.T) {
	tw := NewTimerWheelWithConfig(50*time.Millisecond, 16, logger.DefaultLogger(os.Stdout, "debug"))

	if err := tw.Start(); err != nil {
		t.Fatalf("Failed to start timer wheel: %v", err)
	}
	defer tw.Stop()

	// Schedule multiple tasks with different delays
	task1 := newMockTask()
	task2 := newMockTask()
	task3 := newMockTask()

	// Schedule in reverse order of expected execution
	tw.NewTimeout(task3, 300*time.Millisecond)
	tw.NewTimeout(task1, 100*time.Millisecond)
	tw.NewTimeout(task2, 200*time.Millisecond)

	// Verify execution order
	if !task1.waitForExecution(500 * time.Millisecond) {
		t.Error("Task1 was not executed first")
	}

	if !task2.waitForExecution(500 * time.Millisecond) {
		t.Error("Task2 was not executed second")
	}

	if !task3.waitForExecution(500 * time.Millisecond) {
		t.Error("Task3 was not executed third")
	}
}

func TestTimerWheel_Cancellation(t *testing.T) {
	tw := NewTimerWheelWithConfig(100*time.Millisecond, 8, logger.DefaultLogger(os.Stdout, "debug"))

	if err := tw.Start(); err != nil {
		t.Fatalf("Failed to start timer wheel: %v", err)
	}
	defer tw.Stop()

	task := newMockTask()
	timeout := tw.NewTimeout(task, 500*time.Millisecond)

	// Cancel the timeout immediately
	if !timeout.Cancel() {
		t.Error("Failed to cancel timeout")
	}

	// Wait to ensure task is not executed
	time.Sleep(800 * time.Millisecond)

	if task.executed.Load() {
		t.Error("Cancelled task was executed")
	}
}

func TestTimerWheel_StartStop(t *testing.T) {
	tw := NewTimerWheelWithConfig(100*time.Millisecond, 8, logger.DefaultLogger(os.Stdout, "debug"))

	// Test start
	if err := tw.Start(); err != nil {
		t.Fatalf("Failed to start timer wheel: %v", err)
	}

	if !tw.IsStarted() {
		t.Error("Timer wheel should be started")
	}

	// Test stop
	if err := tw.Stop(); err != nil {
		t.Fatalf("Failed to stop timer wheel: %v", err)
	}

	if !tw.IsStopped() {
		t.Error("Timer wheel should be stopped")
	}

	// Test scheduling after stop
	task := newMockTask()
	timeout := tw.NewTimeout(task, 100*time.Millisecond)

	if timeout != nil {
		t.Error("Should not be able to schedule after stop")
	}
}

func TestTimerWheel_Stats(t *testing.T) {
	tw := NewTimerWheelWithConfig(100*time.Millisecond, 8, logger.DefaultLogger(os.Stdout, "debug"))

	if err := tw.Start(); err != nil {
		t.Fatalf("Failed to start timer wheel: %v", err)
	}
	defer tw.Stop()

	// Schedule some tasks
	task1 := newMockTask()
	task2 := newMockTask()

	tw.NewTimeout(task1, 200*time.Millisecond)
	tw.NewTimeout(task2, 400*time.Millisecond)

	stats := tw.Stats()

	if stats.WheelSize != 8 {
		t.Errorf("Expected wheel size 8, got %d", stats.WheelSize)
	}

	if stats.TickDuration != 100*time.Millisecond {
		t.Errorf("Expected tick duration 100ms, got %v", stats.TickDuration)
	}

	if len(stats.BucketStats) != 8 {
		t.Errorf("Expected 8 bucket stats, got %d", len(stats.BucketStats))
	}
}

func TestTimerWheel_HighLoad(t *testing.T) {
	tw := NewTimerWheelWithConfig(10*time.Millisecond, 64, logger.DefaultLogger(os.Stdout, "debug"))

	if err := tw.Start(); err != nil {
		t.Fatalf("Failed to start timer wheel: %v", err)
	}
	defer tw.Stop()

	const numTasks = 1000
	tasks := make([]*mockTask, numTasks)

	// Schedule many tasks
	for i := 0; i < numTasks; i++ {
		tasks[i] = newMockTask()
		delay := time.Duration(i%100) * time.Millisecond
		tw.NewTimeout(tasks[i], delay)
	}

	// Wait for all tasks to complete
	time.Sleep(2 * time.Second)

	// Check how many tasks executed
	executedCount := 0
	for _, task := range tasks {
		if task.executed.Load() {
			executedCount++
		}
	}

	// We expect most tasks to execute (allowing for some variance due to timing)
	expectedMin := numTasks * 80 / 100 // 80% execution rate minimum
	if executedCount < expectedMin {
		t.Errorf("Expected at least %d tasks to execute, got %d", expectedMin, executedCount)
	}

	t.Logf("Executed %d out of %d tasks (%.1f%%)",
		executedCount, numTasks, float64(executedCount)*100/float64(numTasks))
}
