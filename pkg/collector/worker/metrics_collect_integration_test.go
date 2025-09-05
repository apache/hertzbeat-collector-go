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
//	"testing"
//
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector"
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
//	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
//)
//
//// MockCollectDataDispatcher implements CollectDataDispatcher for testing
//type MockCollectDataDispatcher struct {
//	dispatched []*MetricsData
//}
//
//func (mcd *MockCollectDataDispatcher) DispatchCollectData(timeout *jobtypes.Timeout, metrics *jobtypes.Metrics, data []*MetricsData) error {
//	mcd.dispatched = append(mcd.dispatched, data...)
//	return nil
//}
//
//func TestMetricsCollect_JDBCIntegration(t *testing.T) {
//	// Create logger
//	log := logger.DefaultLogger(os.Stdout, types.LogLevelInfo)
//
//	// Create collect service with JDBC collector
//	collectService := collector.NewCollectServiceWithBuiltins(log)
//
//	// Create mock dispatcher
//	dispatcher := &MockCollectDataDispatcher{
//		dispatched: make([]*MetricsData, 0),
//	}
//
//	// Test JDBC collection with connection error (expected since we don't have a real database)
//	metrics := &jobtypes.Metrics{
//		Name:     "mysql-test",
//		Protocol: "jdbc",
//		Host:     "localhost",
//		Port:     "3306",
//		JDBC: &jobtypes.JDBCProtocol{
//			Host:      "localhost",
//			Port:      "3306",
//			Platform:  "mysql",
//			Username:  "test",
//			Password:  "test",
//			Database:  "test",
//			QueryType: "oneRow",
//			SQL:       "SELECT 1 as test_value",
//			Timeout:   "10",
//		},
//	}
//
//	// Create MetricsCollect task
//	mc := NewMetricsCollect(
//		metrics,
//		nil, // timeout - passing nil for simplified testing
//		dispatcher,
//		"test-collector",
//		collectService,
//		log,
//	)
//
//	if mc == nil {
//		t.Fatal("failed to create MetricsCollect")
//	}
//
//	// Execute the task
//	err := mc.Execute()
//	if err != nil {
//		t.Errorf("Execute() failed: %v", err)
//	}
//
//	// Check that data was dispatched
//	if len(dispatcher.dispatched) == 0 {
//		t.Error("no data was dispatched")
//	}
//
//	// Check the dispatched data
//	data := dispatcher.dispatched[len(dispatcher.dispatched)-1]
//	if data.Metrics != "mysql-test" {
//		t.Errorf("expected metrics name 'mysql-test', got '%s'", data.Metrics)
//	}
//
//	// Should have error code due to connection failure
//	if data.Code != 502 {
//		t.Errorf("expected error code 502 (connection error), got %d", data.Code)
//	}
//
//	t.Logf("Dispatched data: Code=%d, Msg=%s", data.Code, data.Msg)
//}
//
//func TestMetricsCollect_UnsupportedProtocol(t *testing.T) {
//	// Create logger
//	log := logger.DefaultLogger(os.Stdout, types.LogLevelInfo)
//
//	// Create collect service
//	collectService := collector.NewCollectServiceWithBuiltins(log)
//
//	// Create mock dispatcher
//	dispatcher := &MockCollectDataDispatcher{
//		dispatched: make([]*MetricsData, 0),
//	}
//
//	// Test unsupported protocol
//	metrics := &jobtypes.Metrics{
//		Name:     "unknown-test",
//		Protocol: "unknown",
//		Host:     "localhost",
//		Port:     "8080",
//	}
//
//	mc := NewMetricsCollect(
//		metrics,
//		nil,
//		dispatcher,
//		"test-collector",
//		collectService,
//		log,
//	)
//
//	if mc == nil {
//		t.Fatal("failed to create MetricsCollect")
//	}
//
//	err := mc.Execute()
//	if err != nil {
//		t.Errorf("Execute() failed: %v", err)
//	}
//
//	// Check that data was dispatched
//	if len(dispatcher.dispatched) == 0 {
//		t.Error("no data was dispatched")
//	}
//
//	// Check the dispatched data
//	data := dispatcher.dispatched[len(dispatcher.dispatched)-1]
//	if data.Code == 0 {
//		t.Error("expected error code for unsupported protocol, got success")
//	}
//
//	t.Logf("Dispatched data for unsupported protocol: Code=%d, Msg=%s", data.Code, data.Msg)
//}
//
//func TestMetricsDataBuilder(t *testing.T) {
//	builder := NewMetricsDataBuilder()
//
//	// Build a complete metrics data
//	fields := []jobtypes.Field{
//		{Field: "name", Type: 1, Label: false},
//		{Field: "value", Type: 2, Label: false},
//	}
//
//	metadata := map[string]string{"source": "test"}
//	labels := map[string]string{"env": "testing"}
//
//	data := builder.
//		SetApp("test-app").
//		SetID(456).
//		SetTenantID(789).
//		SetMetrics("test-metrics").
//		SetCode(200).
//		SetMsg("success").
//		SetFields(fields).
//		AddValue([]string{"test", "123"}).
//		AddValue([]string{"test2", "456"}).
//		SetMetadata(metadata).
//		SetLabels(labels).
//		Build()
//
//	// Verify the built data
//	if data.App != "test-app" {
//		t.Errorf("expected App 'test-app', got '%s'", data.App)
//	}
//
//	if data.ID != 456 {
//		t.Errorf("expected ID 456, got %d", data.ID)
//	}
//
//	if data.TenantID != 789 {
//		t.Errorf("expected TenantID 789, got %d", data.TenantID)
//	}
//
//	if data.Metrics != "test-metrics" {
//		t.Errorf("expected Metrics 'test-metrics', got '%s'", data.Metrics)
//	}
//
//	if data.Code != 200 {
//		t.Errorf("expected Code 200, got %d", data.Code)
//	}
//
//	if data.Msg != "success" {
//		t.Errorf("expected Msg 'success', got '%s'", data.Msg)
//	}
//
//	if len(data.Fields) != 2 {
//		t.Errorf("expected 2 fields, got %d", len(data.Fields))
//	}
//
//	if len(data.Values) != 2 {
//		t.Errorf("expected 2 rows, got %d", len(data.Values))
//	}
//
//	if data.Values[0][0] != "test" {
//		t.Errorf("expected first value 'test', got '%s'", data.Values[0][0])
//	}
//
//	if data.Metadata["source"] != "test" {
//		t.Errorf("expected metadata source 'test', got '%s'", data.Metadata["source"])
//	}
//
//	if data.Labels["env"] != "testing" {
//		t.Errorf("expected label env 'testing', got '%s'", data.Labels["env"])
//	}
//
//	if data.Time <= 0 {
//		t.Error("timestamp should be positive")
//	}
//}
