package collector

import (
	"fmt"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/basic"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/timer"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// CollectService manages all collectors and provides a unified interface
type CollectService struct {
	registry        *basic.CollectorRegistry
	logger          logger.Logger
	timerDispatcher *timer.TimerDispatcher // Optional: for advanced task management
	defaultPriority int                    // Default task priority
	timeoutRetries  int                    // Default retry count for timeouts
}

// NewCollectService creates a new collect service
func NewCollectService(logger logger.Logger) *CollectService {
	service := &CollectService{
		registry:        basic.NewCollectorRegistry(logger),
		logger:          logger.WithName("collect-service"),
		defaultPriority: 5, // Medium priority (1-10 scale)
		timeoutRetries:  3, // Default retry count
	}

	// Note: Collectors should be registered externally to avoid circular dependencies
	// Use RegisterCollector() method to register specific collectors

	return service
}

// SetTimerDispatcher sets the timer dispatcher for advanced task management
func (cs *CollectService) SetTimerDispatcher(dispatcher *timer.TimerDispatcher) {
	cs.timerDispatcher = dispatcher
	cs.logger.Info("timer dispatcher configured for collect service")
}

// SetDefaultPriority sets the default priority for collection tasks
func (cs *CollectService) SetDefaultPriority(priority int) {
	if priority < 1 || priority > 10 {
		cs.logger.Info("invalid priority, using default", "provided", priority, "default", cs.defaultPriority)
		return
	}
	cs.defaultPriority = priority
	cs.logger.Info("default priority updated", "priority", priority)
}

// SetTimeoutRetries sets the default retry count for timeout handling
func (cs *CollectService) SetTimeoutRetries(retries int) {
	if retries < 0 || retries > 10 {
		cs.logger.Info("invalid retry count, using default", "provided", retries, "default", cs.timeoutRetries)
		return
	}
	cs.timeoutRetries = retries
	cs.logger.Info("timeout retries updated", "retries", retries)
}

// RegisterCollector registers a collector with the service
func (cs *CollectService) RegisterCollector(protocol string, collector basic.AbstractCollector) {
	cs.registry.Register(protocol, collector)
	cs.logger.Info("registered collector", "protocol", protocol)
}

// Collect performs metrics collection using the appropriate collector
func (cs *CollectService) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	if metrics == nil {
		cs.logger.Error(fmt.Errorf("metrics is nil"), "failed to collect metrics")
		return &jobtypes.CollectRepMetricsData{
			Code: 500,
			Msg:  "metrics configuration is nil",
		}
	}

	protocol := metrics.Protocol
	if protocol == "" {
		cs.logger.Error(fmt.Errorf("protocol not specified"), "failed to collect metrics")
		return &jobtypes.CollectRepMetricsData{
			Code: 500,
			Msg:  "protocol not specified",
		}
	}

	collector, exists := cs.registry.GetCollector(protocol)
	if !exists {
		cs.logger.Error(fmt.Errorf("no collector found for protocol: %s", protocol), "failed to collect metrics")
		return &jobtypes.CollectRepMetricsData{
			Code: 500,
			Msg:  fmt.Sprintf("unsupported protocol: %s", protocol),
		}
	}

	cs.logger.Info("starting metrics collection",
		"protocol", protocol,
		"name", metrics.Name,
		"host", metrics.Host,
		"port", metrics.Port)

	// Pre-check the metrics configuration
	if err := collector.PreCheck(metrics); err != nil {
		cs.logger.Error(err, "metrics pre-check failed", "protocol", protocol)
		return &jobtypes.CollectRepMetricsData{
			Code: 400,
			Msg:  fmt.Sprintf("pre-check failed: %v", err),
		}
	}

	// Perform the actual collection
	result := collector.Collect(metrics)

	cs.logger.Info("metrics collection completed",
		"protocol", protocol,
		"name", metrics.Name,
		"code", result.Code,
		"msg", result.Msg)

	return result
}

// GetSupportedProtocols returns all supported protocols
func (cs *CollectService) GetSupportedProtocols() []string {
	return cs.registry.GetSupportedProtocols()
}

// GetCollector gets a collector for the specified protocol (for testing)
func (cs *CollectService) GetCollector(protocol string) (basic.AbstractCollector, bool) {
	return cs.registry.GetCollector(protocol)
}

// DispatchMetricsTask implements MetricsTaskDispatcher interface
func (cs *CollectService) DispatchMetricsTask(timeout *jobtypes.Timeout) error {
	if timeout == nil {
		cs.logger.Error(fmt.Errorf("timeout is nil"), "failed to dispatch metrics task")
		return fmt.Errorf("timeout cannot be nil")
	}

	cs.logger.Info("dispatching metrics task")

	// Extract job information from timeout task
	var job *jobtypes.Job
	if wheelTask, ok := timeout.Task().(*timer.WheelTimerTask); ok {
		job = wheelTask.GetJob()
	} else {
		cs.logger.Error(fmt.Errorf("timeout task is not a WheelTimerTask"), "failed to dispatch metrics task")
		return fmt.Errorf("timeout task must be a WheelTimerTask")
	}

	if job == nil {
		cs.logger.Error(fmt.Errorf("job is nil"), "failed to dispatch metrics task")
		return fmt.Errorf("job cannot be nil")
	}

	// Get metrics to collect for this job
	metricsSet := job.GetNextCollectMetrics()
	if len(metricsSet) == 0 {
		cs.logger.Info("no metrics to collect for job", "job_id", job.ID, "app", job.App)
		return nil
	}

	cs.logger.Info("creating metrics collection tasks",
		"job_id", job.ID,
		"app", job.App,
		"metrics_count", len(metricsSet))

	// Create and submit collection tasks for each metrics
	var lastError error
	successCount := 0

	for _, metrics := range metricsSet {
		if err := cs.createAndSubmitTask(metrics, timeout, job); err != nil {
			cs.logger.Error(err, "failed to create metrics collection task",
				"metrics", metrics.Name,
				"protocol", metrics.Protocol)
			lastError = err
		} else {
			successCount++
		}
	}

	cs.logger.Info("metrics tasks dispatched",
		"total", len(metricsSet),
		"success", successCount,
		"failed", len(metricsSet)-successCount)

	// Return error only if all tasks failed
	if successCount == 0 && lastError != nil {
		return fmt.Errorf("all metrics tasks failed: %w", lastError)
	}

	return nil
}

// createAndSubmitTask creates a metrics collection task and submits it to the worker pool
func (cs *CollectService) createAndSubmitTask(metrics *jobtypes.Metrics, timeout *jobtypes.Timeout, job *jobtypes.Job) error {
	// Validate metrics
	if metrics == nil {
		return fmt.Errorf("metrics is nil")
	}

	// Check if we have a collector for this protocol
	if _, exists := cs.registry.GetCollector(metrics.Protocol); !exists {
		return fmt.Errorf("no collector found for protocol: %s", metrics.Protocol)
	}

	cs.logger.Info("creating metrics collection task",
		"metrics", metrics.Name,
		"protocol", metrics.Protocol,
		"host", metrics.Host,
		"port", metrics.Port)

	// Calculate task priority based on metrics configuration
	taskPriority := cs.calculateTaskPriority(metrics, job)

	// Add timeout monitoring if timer dispatcher is available
	if cs.timerDispatcher != nil {
		cs.timerDispatcher.AddMetricsTimeout(metrics, timeout)
		defer cs.timerDispatcher.RemoveMetricsTimeout(metrics, timeout)
	}

	// Perform collection with priority and timeout handling
	start := time.Now()
	result := cs.collectWithPriority(metrics, timeout, taskPriority)
	duration := time.Since(start)

	if result == nil {
		return fmt.Errorf("collection returned nil result")
	}

	cs.logger.Info("metrics collection completed",
		"metrics", metrics.Name,
		"protocol", metrics.Protocol,
		"priority", taskPriority,
		"duration", duration,
		"code", result.Code,
		"message", result.Msg)

	// TODO: Dispatch collected data to appropriate handlers
	// This would typically involve calling collectDataDispatch.dispatchCollectData()

	return nil
}

// calculateTaskPriority calculates the priority for a metrics collection task
func (cs *CollectService) calculateTaskPriority(metrics *jobtypes.Metrics, job *jobtypes.Job) int {
	priority := cs.defaultPriority

	// Adjust priority based on metrics configuration
	if metrics.Priority > 0 {
		priority = metrics.Priority
	}

	// Adjust priority based on protocol criticality
	switch metrics.Protocol {
	case "icmp", "ping":
		priority += 2 // High priority for availability checks
	case "http", "https":
		priority += 1 // Medium-high priority for web services
	case "snmp":
		priority -= 1 // Lower priority for SNMP (usually less time-sensitive)
	}

	// Ensure priority stays within valid range
	if priority < 1 {
		priority = 1
	} else if priority > 10 {
		priority = 10
	}

	return priority
}

// collectWithPriority performs metrics collection with priority and timeout handling
func (cs *CollectService) collectWithPriority(metrics *jobtypes.Metrics, timeout *jobtypes.Timeout, priority int) *jobtypes.CollectRepMetricsData {
	// Create channel for result
	resultChan := make(chan *jobtypes.CollectRepMetricsData, 1)

	// Calculate timeout duration based on priority (higher priority gets more time)
	baseTimeout := 60 * time.Second
	if metrics.Timeout != "" {
		if duration, err := time.ParseDuration(metrics.Timeout); err == nil {
			baseTimeout = duration
		}
	}

	// Adjust timeout based on priority
	timeoutDuration := baseTimeout
	if priority >= 8 {
		timeoutDuration = baseTimeout + (baseTimeout / 2) // 50% more time for high priority
	} else if priority <= 3 {
		timeoutDuration = baseTimeout - (baseTimeout / 4) // 25% less time for low priority
	}

	cs.logger.Info("starting prioritized collection",
		"metrics", metrics.Name,
		"protocol", metrics.Protocol,
		"priority", priority,
		"timeout", timeoutDuration)

	// Perform collection in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				cs.logger.Error(fmt.Errorf("panic in collection: %v", r), "metrics collection panicked",
					"metrics", metrics.Name, "protocol", metrics.Protocol, "priority", priority)
				resultChan <- &jobtypes.CollectRepMetricsData{
					Code: 500,
					Msg:  fmt.Sprintf("collection panicked: %v", r),
				}
			}
		}()

		// Add priority-based delay for lower priority tasks (simple congestion control)
		if priority <= 3 {
			time.Sleep(time.Duration(5-priority) * 100 * time.Millisecond)
		}

		result := cs.Collect(metrics)
		resultChan <- result
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result
	case <-time.After(timeoutDuration):
		cs.logger.Info("metrics collection timed out",
			"metrics", metrics.Name,
			"protocol", metrics.Protocol,
			"priority", priority,
			"timeout", timeoutDuration)

		return &jobtypes.CollectRepMetricsData{
			Code: 504, // Gateway Timeout
			Msg:  fmt.Sprintf("collection timed out after %v (priority: %d)", timeoutDuration, priority),
		}
	}
}
