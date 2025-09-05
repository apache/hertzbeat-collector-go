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

package entrance

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/api"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/timer"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/worker"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// TestMySQLJobScheduling tests the complete MySQL job scheduling flow
// from CollectJobService to JDBC collection without requiring Manager connection
func TestMySQLJobScheduling(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, types.LogLevelInfo)

	t.Log("=== MySQL Job Scheduling Test (Mock Manager) ===")

	// Create components directly without CollectServer
	timerDispatcher := timer.NewTimerDispatcher(log)
	workerConfig := worker.DefaultWorkerPoolConfig()
	workerConfig.CoreSize = 2
	workerConfig.MaxSize = 4
	workerPool := worker.NewWorkerPool(workerConfig, log)

	// Create collect service with JDBC collector
	collectService := createCollectServiceWithJDBC(log)

	// Create collect job service
	collectorConfig := DefaultCollectorConfig()
	collectJobService := NewCollectJobService(timerDispatcher, workerPool, collectorConfig, log)

	// Wire components - TimerDispatcher itself implements MetricsTaskDispatcher
	timerDispatcher.SetMetricsDispatcher(timerDispatcher)
	timerDispatcher.SetCollectDataDispatcher(collectJobService)
	timerDispatcher.SetCollectService(collectService) // Set the collect service!

	// Start components
	err := workerPool.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	err = timerDispatcher.Start()
	if err != nil {
		t.Fatalf("Failed to start timer dispatcher: %v", err)
	}
	defer timerDispatcher.Stop()

	// Test 1: Mock Manager sending cyclic job
	t.Run("MockCyclicJobFromManager", func(t *testing.T) {
		testMockCyclicJob(t, collectJobService, log)
	})

	// Test 2: Mock Manager sending one-time job
	t.Run("MockOneTimeJobFromManager", func(t *testing.T) {
		testMockOneTimeJob(t, collectJobService, log)
	})

	// Test 3: Mock Manager canceling job
	t.Run("MockJobCancellation", func(t *testing.T) {
		testMockJobCancellation(t, collectJobService, log)
	})

	t.Log("=== All MySQL Job Scheduling Tests Completed ===")
}

// testMockCyclicJob simulates Manager sending a cyclic MySQL job
func testMockCyclicJob(t *testing.T, service *CollectJobService, log logger.Logger) {
	t.Log("--- Testing Mock Cyclic MySQL Job ---")

	// Create a cyclic MySQL job (simulate what Manager would send)
	job := createMySQLCyclicJob()

	// Mock Manager sending ISSUE_CYCLIC_TASK message
	message := &cluster_msg.Message{
		Identity:  "test-manager",
		Direction: cluster_msg.Direction_REQUEST,
		Type:      cluster_msg.MessageType_ISSUE_CYCLIC_TASK,
		Msg:       serializeJob(t, job),
	}

	t.Logf("Mock Manager sending cyclic job: %s (ID: %d, Interval: %ds)",
		job.Name["default"], job.ID, job.DefaultInterval)

	// Process the message (simulate receiving from Manager)
	processor := NewCollectCyclicDataProcessor(log, service)
	err := processor.Process(nil, message)
	if err != nil {
		t.Errorf("Failed to process cyclic job message: %v", err)
		return
	}

	t.Log("Cyclic job added successfully")

	// Wait for job execution cycles
	time.Sleep(8 * time.Second)

	// Mock Manager canceling the job
	cancelMessage := &cluster_msg.Message{
		Identity:  "test-manager",
		Direction: cluster_msg.Direction_REQUEST,
		Type:      cluster_msg.MessageType_DELETE_CYCLIC_TASK,
		Msg:       serializeJobCancel(t, job.ID),
	}

	cancelProcessor := NewCollectCyclicDataProcessor(log, service)
	err = cancelProcessor.Process(nil, cancelMessage)
	if err != nil {
		t.Errorf("Failed to cancel cyclic job: %v", err)
	} else {
		t.Log("Cyclic job cancelled successfully")
	}
}

// testMockOneTimeJob simulates Manager sending a one-time MySQL job
func testMockOneTimeJob(t *testing.T, service *CollectJobService, log logger.Logger) {
	t.Log("--- Testing Mock One-Time MySQL Job ---")

	// Create a one-time MySQL job
	job := createMySQLOneTimeJob()

	// Mock Manager sending ISSUE_ONE_TIME_TASK message
	message := &cluster_msg.Message{
		Identity:  "test-manager",
		Direction: cluster_msg.Direction_REQUEST,
		Type:      cluster_msg.MessageType_ISSUE_ONE_TIME_TASK,
		Msg:       serializeJob(t, job),
	}

	t.Logf("Mock Manager sending one-time job: %s (ID: %d)",
		job.Name["default"], job.ID)

	// Process the message
	processor := NewCollectOneTimeDataProcessor(log, service)
	err := processor.Process(nil, message)
	if err != nil {
		t.Errorf("Failed to process one-time job message: %v", err)
		return
	}

	t.Log("One-time job executed successfully")

	// Wait for job completion
	time.Sleep(3 * time.Second)
}

// testMockJobCancellation tests job cancellation scenario
func testMockJobCancellation(t *testing.T, service *CollectJobService, log logger.Logger) {
	t.Log("--- Testing Mock Job Cancellation ---")

	// Create a long-running cyclic job
	job := createMySQLLongRunningJob()

	// Add the job
	err := service.AddAsyncCollectJob(job)
	if err != nil {
		t.Errorf("Failed to add job for cancellation test: %v", err)
		return
	}

	t.Logf("Added job for cancellation test: %s (ID: %d)", job.Name["default"], job.ID)

	// Wait a bit to let it start
	time.Sleep(2 * time.Second)

	// Cancel the job
	err = service.CancelAsyncCollectJob(job.ID)
	if err != nil {
		t.Errorf("Failed to cancel job: %v", err)
	} else {
		t.Log("Job cancelled successfully")
	}
}

// Helper functions for creating test jobs

// createMySQLCyclicJob creates a cyclic MySQL status monitoring job
func createMySQLCyclicJob() *jobtypes.Job {
	return &jobtypes.Job{
		ID:        time.Now().UnixNano(),
		TenantID:  1,
		MonitorID: 2001,
		Name: map[string]string{
			"default": "mysql-cyclic-status",
		},
		Category: "mysql",
		App:      "mysql",
		Metadata: map[string]string{
			"description": "Cyclic MySQL status monitoring",
		},
		Metrics: []jobtypes.Metrics{
			{
				Name:     "mysql-status",
				Protocol: "jdbc",
				Host:     "localhost",
				Port:     "53306",
				JDBC: &jobtypes.JDBCProtocol{
					Host:      "localhost",
					Port:      "53306",
					Platform:  "mysql",
					Username:  "root",
					Password:  "password",
					Database:  "mysql",
					QueryType: "oneRow",
					SQL:       "SHOW STATUS LIKE 'Uptime'",
					Timeout:   "10",
				},
				Aliasfields: []string{"Variable_name", "Value"},
			},
		},
		IsCyclic:        true,
		DefaultInterval: 3, // 3 seconds for testing
	}
}

// createMySQLOneTimeJob creates a one-time MySQL variables check job
func createMySQLOneTimeJob() *jobtypes.Job {
	return &jobtypes.Job{
		ID:        time.Now().UnixNano() + 1,
		TenantID:  1,
		MonitorID: 2002,
		Name: map[string]string{
			"default": "mysql-onetime-variables",
		},
		Category: "mysql",
		App:      "mysql",
		Metadata: map[string]string{
			"description": "One-time MySQL variables check",
		},
		Metrics: []jobtypes.Metrics{
			{
				Name:     "mysql-variables",
				Protocol: "jdbc",
				Host:     "localhost",
				Port:     "53306",
				JDBC: &jobtypes.JDBCProtocol{
					Host:      "localhost",
					Port:      "53306",
					Platform:  "mysql",
					Username:  "hertzbeat",
					Password:  "hertzbeat",
					Database:  "mysql",
					QueryType: "columns",
					SQL:       "SELECT @@max_connections, @@innodb_buffer_pool_size",
					Timeout:   "10",
				},
				Aliasfields: []string{"max_connections", "innodb_buffer_pool_size"},
			},
		},
		IsCyclic:        false,
		DefaultInterval: 0,
	}
}

// createMySQLLongRunningJob creates a job for cancellation testing
func createMySQLLongRunningJob() *jobtypes.Job {
	return &jobtypes.Job{
		ID:        time.Now().UnixNano() + 2,
		TenantID:  1,
		MonitorID: 2003,
		Name: map[string]string{
			"default": "mysql-long-running",
		},
		Category: "mysql",
		App:      "mysql",
		Metadata: map[string]string{
			"description": "Long running MySQL monitoring for cancellation test",
		},
		Metrics: []jobtypes.Metrics{
			{
				Name:     "mysql-processes",
				Protocol: "jdbc",
				Host:     "localhost",
				Port:     "53306",
				JDBC: &jobtypes.JDBCProtocol{
					Host:      "localhost",
					Port:      "53306",
					Platform:  "mysql",
					Username:  "hertzbeat",
					Password:  "hertzbeat",
					Database:  "information_schema",
					QueryType: "multiRow",
					SQL:       "SELECT ID, USER, HOST, DB, COMMAND FROM PROCESSLIST LIMIT 10",
					Timeout:   "15",
				},
				Aliasfields: []string{"ID", "USER", "HOST", "DB", "COMMAND"},
			},
		},
		IsCyclic:        true,
		DefaultInterval: 5, // 5 seconds
	}
}

// Helper functions for message serialization

// serializeJob serializes a job to JSON for message transmission
func serializeJob(t *testing.T, job *jobtypes.Job) []byte {
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to serialize job: %v", err)
	}
	return data
}

// serializeJobCancel serializes job cancellation data
func serializeJobCancel(t *testing.T, jobID int64) []byte {
	cancelData := map[string]interface{}{
		"id": jobID,
	}
	data, err := json.Marshal(cancelData)
	if err != nil {
		t.Fatalf("Failed to serialize job cancel data: %v", err)
	}
	return data
}

// MockResultCollector captures test results for verification
type MockResultCollector struct {
	results []interface{}
	mu      sync.RWMutex
}

// NewMockResultCollector creates a new mock result collector
func NewMockResultCollector() *MockResultCollector {
	return &MockResultCollector{
		results: make([]interface{}, 0),
	}
}

// AddResult adds a result to the collector
func (mrc *MockResultCollector) AddResult(result interface{}) {
	mrc.mu.Lock()
	defer mrc.mu.Unlock()
	mrc.results = append(mrc.results, result)
}

// GetResults returns all collected results
func (mrc *MockResultCollector) GetResults() []interface{} {
	mrc.mu.RLock()
	defer mrc.mu.RUnlock()
	return append([]interface{}{}, mrc.results...)
}

// GetCount returns the number of collected results
func (mrc *MockResultCollector) GetCount() int {
	mrc.mu.RLock()
	defer mrc.mu.RUnlock()
	return len(mrc.results)
}

// createCollectServiceWithJDBC creates a CollectService with JDBC collector registered
func createCollectServiceWithJDBC(log logger.Logger) worker.CollectService {
	return collector.NewCollectServiceWithBuiltins(log)
}
