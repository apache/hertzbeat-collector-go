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

//import (
//	"os"
//	"sync/atomic"
//	"testing"
//	"time"
//
//	"hertzbeat.apache.org/hertzbeat-collector-go/internal/logger"
//	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
//)
//
//// mockMetricsDispatcher implements MetricsTaskDispatcher for testing
//type mockMetricsDispatcher struct {
//	dispatchCount atomic.Int64
//	lastTimeout   *jobtypes.Timeout
//}
//
//func (m *mockMetricsDispatcher) DispatchMetricsTask(timeout *jobtypes.Timeout) error {
//	m.dispatchCount.Add(1)
//	m.lastTimeout = timeout
//	return nil
//}
//
//func (m *mockMetricsDispatcher) getDispatchCount() int64 {
//	return m.dispatchCount.Load()
//}
//
//// mockEventListener implements CollectResponseEventListener for testing
//type mockEventListener struct {
//	responseCount atomic.Int64
//	lastResponse  []interface{}
//}
//
//func (m *mockEventListener) Response(metricsData []interface{}) {
//	m.responseCount.Add(1)
//	m.lastResponse = metricsData
//}
//
//func (m *mockEventListener) getResponseCount() int64 {
//	return m.responseCount.Load()
//}
//
//func createTestJob(id int64, isCyclic bool, interval int64) *jobtypes.Job {
//	return &jobtypes.Job{
//		ID:              id,
//		MonitorID:       id + 1000,
//		TenantID:        1,
//		App:             "test",
//		IsCyclic:        isCyclic,
//		DefaultInterval: interval,
//		Metrics: []jobtypes.Metrics{
//			{
//				Name:     "test-metric",
//				Priority: 0,
//				Protocol: "http",
//				Interval: interval,
//			},
//		},
//		Configmap: []jobtypes.Configmap{
//			{
//				Key:   "host",
//				Value: "localhost",
//				Type:  0,
//			},
//		},
//	}
//}
//
//func TestTimerDispatcher_AddCyclicJob(t *testing.T) {
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Create a cyclic job
//	job := createTestJob(1, true, 1) // 1 second interval
//
//	// Add job
//	if err := dispatcher.AddJob(job, nil); err != nil {
//		t.Fatalf("Failed to add job: %v", err)
//	}
//
//	// Wait for at least one execution
//	time.Sleep(1500 * time.Millisecond)
//
//	stats := dispatcher.GetStats()
//	if stats.CyclicTaskCount != 1 {
//		t.Errorf("Expected 1 cyclic task, got %d", stats.CyclicTaskCount)
//	}
//
//	// Should have been dispatched at least once
//	if mockDispatcher.getDispatchCount() < 1 {
//		t.Errorf("Expected at least 1 dispatch, got %d", mockDispatcher.getDispatchCount())
//	}
//}
//
//func TestTimerDispatcher_AddOneTimeJob(t *testing.T) {
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Create event listener
//	listener := &mockEventListener{}
//
//	// Create a one-time job
//	job := createTestJob(2, false, 1) // one-time job
//
//	// Add job
//	if err := dispatcher.AddJob(job, listener); err != nil {
//		t.Fatalf("Failed to add job: %v", err)
//	}
//
//	// Wait for execution
//	time.Sleep(1500 * time.Millisecond)
//
//	// One-time job should be dispatched but removed from temp tasks after execution
//	if mockDispatcher.getDispatchCount() < 1 {
//		t.Errorf("Expected at least 1 dispatch, got %d", mockDispatcher.getDispatchCount())
//	}
//}
//
//func TestTimerDispatcher_DeleteJob(t *testing.T) {
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Add cyclic job
//	job := createTestJob(3, true, 2) // 2 second interval
//	if err := dispatcher.AddJob(job, nil); err != nil {
//		t.Fatalf("Failed to add job: %v", err)
//	}
//
//	// Verify job was added
//	stats := dispatcher.GetStats()
//	if stats.CyclicTaskCount != 1 {
//		t.Errorf("Expected 1 cyclic task, got %d", stats.CyclicTaskCount)
//	}
//
//	// Delete the job
//	if !dispatcher.DeleteJob(3, true) {
//		t.Error("DeleteJob should return true for existing job")
//	}
//
//	// Verify job was removed
//	stats = dispatcher.GetStats()
//	if stats.CyclicTaskCount != 0 {
//		t.Errorf("Expected 0 cyclic tasks after deletion, got %d", stats.CyclicTaskCount)
//	}
//
//	// Try to delete non-existent job
//	if dispatcher.DeleteJob(999, true) {
//		t.Error("DeleteJob should return false for non-existent job")
//	}
//}
//
//func TestTimerDispatcher_GoOnlineOffline(t *testing.T) {
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Add some jobs
//	job1 := createTestJob(4, true, 1)
//	job2 := createTestJob(5, false, 1)
//	listener := &mockEventListener{}
//
//	dispatcher.AddJob(job1, nil)
//	dispatcher.AddJob(job2, listener)
//
//	// Verify jobs were added
//	stats := dispatcher.GetStats()
//	if stats.CyclicTaskCount != 1 || stats.TempTaskCount != 1 {
//		t.Errorf("Expected 1 cyclic and 1 temp task, got %d cyclic and %d temp",
//			stats.CyclicTaskCount, stats.TempTaskCount)
//	}
//
//	// Go online (should clear all tasks)
//	dispatcher.GoOnline()
//
//	// Verify all tasks were cleared
//	stats = dispatcher.GetStats()
//	if stats.CyclicTaskCount != 0 || stats.TempTaskCount != 0 || stats.ListenerCount != 0 {
//		t.Errorf("Expected all tasks to be cleared, got %d cyclic, %d temp, %d listeners",
//			stats.CyclicTaskCount, stats.TempTaskCount, stats.ListenerCount)
//	}
//}
//
//func TestTimerDispatcher_ResponseSyncJobData(t *testing.T) {
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Add one-time job with listener
//	listener := &mockEventListener{}
//	job := createTestJob(6, false, 1)
//
//	dispatcher.AddJob(job, listener)
//
//	// Simulate response
//	testData := []interface{}{"metric1", "metric2"}
//	dispatcher.ResponseSyncJobData(6, testData)
//
//	// Verify listener was called
//	if listener.getResponseCount() != 1 {
//		t.Errorf("Expected 1 response call, got %d", listener.getResponseCount())
//	}
//
//	// Verify temp task was removed
//	stats := dispatcher.GetStats()
//	if stats.TempTaskCount != 0 {
//		t.Errorf("Expected temp task to be removed, got %d", stats.TempTaskCount)
//	}
//}
//
//func TestTimerDispatcher_MultipleCyclicJobs(t *testing.T) {
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Add multiple cyclic jobs with different intervals
//	const numJobs = 5
//	for i := 0; i < numJobs; i++ {
//		job := createTestJob(int64(100+i), true, 1) // 1 second interval
//		if err := dispatcher.AddJob(job, nil); err != nil {
//			t.Fatalf("Failed to add job %d: %v", i, err)
//		}
//	}
//
//	// Verify all jobs were added
//	stats := dispatcher.GetStats()
//	if stats.CyclicTaskCount != numJobs {
//		t.Errorf("Expected %d cyclic tasks, got %d", numJobs, stats.CyclicTaskCount)
//	}
//
//	// Wait for multiple executions
//	time.Sleep(2500 * time.Millisecond)
//
//	// Should have multiple dispatches (numJobs * ~2 executions)
//	expectedMin := int64(numJobs)
//	if mockDispatcher.getDispatchCount() < expectedMin {
//		t.Errorf("Expected at least %d dispatches, got %d",
//			expectedMin, mockDispatcher.getDispatchCount())
//	}
//
//	t.Logf("Total dispatches: %d for %d jobs", mockDispatcher.getDispatchCount(), numJobs)
//}
//
//func TestTimerDispatcher_StressTest(t *testing.T) {
//	if testing.Short() {
//		t.Skip("Skipping stress test in short mode")
//	}
//
//	dispatcher := NewTimerDispatcher(logger.DefaultLogger(os.Stdout, "debug"))
//	mockDispatcher := &mockMetricsDispatcher{}
//	dispatcher.SetMetricsDispatcher(mockDispatcher)
//
//	if err := dispatcher.Start(); err != nil {
//		t.Fatalf("Failed to start dispatcher: %v", err)
//	}
//	defer dispatcher.Stop()
//
//	// Add many jobs quickly
//	const numJobs = 100
//	for i := 0; i < numJobs; i++ {
//		job := createTestJob(int64(200+i), true, 1) // 1 second interval
//		if err := dispatcher.AddJob(job, nil); err != nil {
//			t.Fatalf("Failed to add job %d: %v", i, err)
//		}
//	}
//
//	// Let them run for a bit
//	time.Sleep(3 * time.Second)
//
//	// Delete half of them
//	for i := 0; i < numJobs/2; i++ {
//		dispatcher.DeleteJob(int64(200+i), true)
//	}
//
//	// Verify correct number remain
//	stats := dispatcher.GetStats()
//	expectedRemaining := numJobs - numJobs/2
//	if stats.CyclicTaskCount != expectedRemaining {
//		t.Errorf("Expected %d remaining tasks, got %d", expectedRemaining, stats.CyclicTaskCount)
//	}
//
//	t.Logf("Stress test completed: %d jobs, %d dispatches",
//		numJobs, mockDispatcher.getDispatchCount())
//}
