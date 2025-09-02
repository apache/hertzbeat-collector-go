package collector

import (
	"fmt"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/basic"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// CollectService manages all collectors and provides a unified interface
type CollectService struct {
	registry *basic.CollectorRegistry
	logger   logger.Logger
}

// NewCollectService creates a new collect service
func NewCollectService(logger logger.Logger) *CollectService {
	service := &CollectService{
		registry: basic.NewCollectorRegistry(logger),
		logger:   logger.WithName("collect-service"),
	}

	// Note: Collectors should be registered externally to avoid circular dependencies
	// Use RegisterCollector() method to register specific collectors

	return service
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
	cs.logger.Info("dispatching metrics task", "timeout", timeout)
	// TODO: Implement actual metrics task dispatching logic
	// This would typically involve creating MetricsCollect tasks and submitting them to a worker pool
	return nil
}
