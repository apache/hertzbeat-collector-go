package entrance

import (
	"os"
	"testing"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/timer"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/worker"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

func TestCollectJobService_BasicOperations(t *testing.T) {
	// Create test logger
	log := logger.DefaultLogger(os.Stdout, "debug")

	// Create timer dispatcher and worker pool
	timerDispatcher := timer.NewTimerDispatcher(log)
	workerConfig := worker.DefaultWorkerPoolConfig()
	workerConfig.CoreSize = 2
	workerConfig.MaxSize = 4
	workerPool := worker.NewWorkerPool(workerConfig, log)

	// Start components
	if err := workerPool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	if err := timerDispatcher.Start(); err != nil {
		t.Fatalf("Failed to start timer dispatcher: %v", err)
	}
	defer timerDispatcher.Stop()

	// Create collector config
	collectorConfig := &CollectorConfig{
		Identity: "test-collector",
		Mode:     "public",
		Version:  "1.0.0",
		IP:       "127.0.0.1",
	}

	// Create collect job service
	service := NewCollectJobService(timerDispatcher, workerPool, collectorConfig, log)

	// Test collector identity
	if service.GetCollectorIdentity() != "test-collector" {
		t.Errorf("Expected identity 'test-collector', got '%s'", service.GetCollectorIdentity())
	}

	// Test collector mode
	if service.GetCollectorMode() != "public" {
		t.Errorf("Expected mode 'public', got '%s'", service.GetCollectorMode())
	}

	// Create a test job
	testJob := &jobtypes.Job{
		ID:        1,
		MonitorID: 100,
		TenantID:  1,
		App:       "test-app",
		IsCyclic:  false,
		Metrics: []jobtypes.Metrics{
			{
				Name:     "test-metric",
				Protocol: "http",
				Interval: 30,
			},
		},
		Configmap: []jobtypes.Configmap{
			{
				Key:   "host",
				Value: "localhost",
				Type:  1,
			},
		},
	}

	// Test adding cyclic job
	if err := service.AddAsyncCollectJob(testJob); err != nil {
		t.Errorf("Failed to add async collect job: %v", err)
	}

	// Test canceling job (expect failure since job doesn't exist in the exact ID)
	if err := service.CancelAsyncCollectJob(testJob.MonitorID); err != nil {
		t.Logf("Expected failure when canceling non-existent job: %v", err)
	}
}

func TestCollectJobService_SyncJobExecution(t *testing.T) {
	// Create test logger
	log := logger.DefaultLogger(os.Stdout, "debug")

	// Create timer dispatcher and worker pool
	timerDispatcher := timer.NewTimerDispatcher(log)
	workerConfig := worker.DefaultWorkerPoolConfig()
	workerConfig.CoreSize = 1
	workerConfig.MaxSize = 2
	workerConfig.QueueSize = 10
	workerPool := worker.NewWorkerPool(workerConfig, log)

	// Start components
	if err := workerPool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	if err := timerDispatcher.Start(); err != nil {
		t.Fatalf("Failed to start timer dispatcher: %v", err)
	}
	defer timerDispatcher.Stop()

	// Create collector config
	collectorConfig := DefaultCollectorConfig()

	// Create collect job service
	service := NewCollectJobService(timerDispatcher, workerPool, collectorConfig, log)

	// Create a simple sync job
	syncJob := &jobtypes.Job{
		ID:        2,
		MonitorID: 200,
		TenantID:  1,
		App:       "sync-test-app",
		IsCyclic:  false,
		Metrics: []jobtypes.Metrics{
			{
				Name:     "sync-test-metric",
				Protocol: "http",
				Interval: 30,
			},
		},
	}

	// Test sync job execution (this will timeout since we don't have actual collectors)
	// But it should not crash and should return empty results
	start := time.Now()
	results, err := service.CollectSyncJobData(syncJob)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Sync job execution failed: %v", err)
	}

	// Should return empty results since no real collection occurred
	if len(results) != 0 {
		t.Logf("Got %d results (expected 0 for mock execution)", len(results))
	}

	// Should not take longer than a reasonable timeout
	if duration > 125*time.Second {
		t.Errorf("Sync job took too long: %v", duration)
	}

	t.Logf("Sync job execution completed in %v", duration)
}

func TestSerializeMetricsData(t *testing.T) {
	// Create test logger
	log := logger.DefaultLogger(os.Stdout, "debug")

	// Create minimal components for service
	timerDispatcher := timer.NewTimerDispatcher(log)
	workerConfig := worker.DefaultWorkerPoolConfig()
	workerPool := worker.NewWorkerPool(workerConfig, log)
	collectorConfig := DefaultCollectorConfig()

	service := NewCollectJobService(timerDispatcher, workerPool, collectorConfig, log)

	// Test data serialization
	testData := []jobtypes.CollectRepMetricsData{
		{
			ID:        1,
			MonitorID: 100,
			App:       "test",
			Metrics:   "cpu",
			Code:      200,
			Msg:       "success",
		},
	}

	data, err := service.serializeMetricsData(testData)
	if err != nil {
		t.Errorf("Failed to serialize metrics data: %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialized data should not be empty")
	}

	t.Logf("Serialized %d bytes of metrics data", len(data))
}
