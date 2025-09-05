// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package entrance

import (
	"fmt"
	"sync"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/timer"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/worker"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// CollectServer manages the complete collection infrastructure including
// communication, scheduling, and job execution.
type CollectServer struct {
	logger logger.Logger
	config *ServerConfig

	// Core components
	timerDispatcher   *timer.TimerDispatcher
	workerPool        *worker.WorkerPool
	collectJobService *CollectJobService
	collectService    *collector.CollectService
	networkClient     *NetworkClient

	// Component states
	isStarted  bool
	startMutex sync.Mutex
}

// ServerConfig holds the configuration for the CollectServer.
type ServerConfig struct {
	Client     *ClientConfig    `yaml:"client" json:"client"`
	Collector  *CollectorConfig `yaml:"collector" json:"collector"`
	WorkerPool struct {
		CoreSize    int           `yaml:"core_size" json:"core_size"`
		MaxSize     int           `yaml:"max_size" json:"max_size"`
		QueueSize   int           `yaml:"queue_size" json:"queue_size"`
		IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	} `yaml:"worker_pool" json:"worker_pool"`
	TimerWheel struct {
		WheelSize int `yaml:"wheel_size" json:"wheel_size"`
		TickMs    int `yaml:"tick_ms" json:"tick_ms"`
	} `yaml:"timer_wheel" json:"timer_wheel"`
}

// DefaultServerConfig returns a default server configuration.
func DefaultServerConfig() *ServerConfig {
	config := &ServerConfig{
		Client:    DefaultClientConfig(),
		Collector: DefaultCollectorConfig(),
	}

	// Worker pool defaults
	config.WorkerPool.CoreSize = 4
	config.WorkerPool.MaxSize = 64
	config.WorkerPool.QueueSize = 1024
	config.WorkerPool.IdleTimeout = 60 * time.Second

	// Timer wheel defaults
	config.TimerWheel.WheelSize = 512
	config.TimerWheel.TickMs = 1000

	return config
}

// CollectServerEventListener implements NetworkEventListener for the server.
type CollectServerEventListener struct {
	logger logger.Logger
	server *CollectServer
}

// NewCollectServerEventListener creates a new event listener.
func NewCollectServerEventListener(logger logger.Logger, server *CollectServer) *CollectServerEventListener {
	return &CollectServerEventListener{
		logger: logger.WithName("server-event-listener"),
		server: server,
	}
}

// OnConnected is called when the network client connects to the manager.
func (csel *CollectServerEventListener) OnConnected() {
	csel.logger.Info("connected to HertzBeat Manager")

	// Bring timer dispatcher online
	if csel.server.timerDispatcher != nil {
		csel.server.timerDispatcher.GoOnline()
	}
}

// OnDisconnected is called when the network client disconnects from the manager.
func (csel *CollectServerEventListener) OnDisconnected() {
	csel.logger.Info("disconnected from HertzBeat Manager")

	// Take timer dispatcher offline
	if csel.server.timerDispatcher != nil {
		csel.server.timerDispatcher.GoOffline()
	}
}

// OnError is called when a network error occurs.
func (csel *CollectServerEventListener) OnError(err error) {
	csel.logger.Error(err, "network client error")
}

// NewCollectServer creates a new CollectServer instance.
func NewCollectServer(config *ServerConfig, logger logger.Logger) (*CollectServer, error) {
	if config == nil {
		config = DefaultServerConfig()
	}

	server := &CollectServer{
		logger: logger.WithName("collect-server"),
		config: config,
	}

	// Initialize components
	if err := server.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	// Wire components together
	server.wireComponents()

	return server, nil
}

// initializeComponents initializes all server components.
func (cs *CollectServer) initializeComponents() error {
	// Initialize worker pool
	workerConfig := worker.WorkerPoolConfig{
		CoreSize:    cs.config.WorkerPool.CoreSize,
		MaxSize:     cs.config.WorkerPool.MaxSize,
		QueueSize:   cs.config.WorkerPool.QueueSize,
		IdleTimeout: cs.config.WorkerPool.IdleTimeout,
	}
	cs.workerPool = worker.NewWorkerPool(workerConfig, cs.logger)

	// Initialize timer dispatcher
	cs.timerDispatcher = timer.NewTimerDispatcher(cs.logger)

	// Initialize collect service with enhanced scheduling capabilities
	cs.collectService = collector.NewCollectService(cs.logger)
	cs.collectService.SetTimerDispatcher(cs.timerDispatcher)
	collector.RegisterBuiltinCollectors(cs.collectService, cs.logger)

	// Initialize collect job service
	cs.collectJobService = NewCollectJobService(
		cs.timerDispatcher,
		cs.workerPool,
		cs.config.Collector,
		cs.logger,
	)

	// Initialize network client with event listener
	eventListener := NewCollectServerEventListener(cs.logger, cs)
	cs.networkClient = NewNetworkClient(
		cs.config.Client,
		cs.config.Collector,
		cs.logger,
		eventListener,
	)

	return nil
}

// wireComponents connects the components together.
func (cs *CollectServer) wireComponents() {
	// Set network client in collect job service
	cs.collectJobService.SetNetworkClient(cs.networkClient)

	// Wire collect service with timer dispatcher for enhanced scheduling
	cs.timerDispatcher.SetCollectService(cs.collectService)
	cs.timerDispatcher.SetCollectDataDispatcher(cs.collectJobService)

	// Register message processors
	cs.registerMessageProcessors()

	// TimerDispatcher implements MetricsTaskDispatcher itself, so we use it as its own dispatcher
	cs.timerDispatcher.SetMetricsDispatcher(cs.timerDispatcher)

	// TODO: Set collect service as the primary metrics task dispatcher (enhanced version)
	// cs.timerDispatcher.SetMetricsDispatcher(cs.collectService)
}

// registerMessageProcessors registers all message processors with the network client.
func (cs *CollectServer) registerMessageProcessors() {
	// Heartbeat processor
	heartbeatProcessor := NewHeartbeatProcessor(cs.logger)
	cs.networkClient.RegisterProcessor(heartbeatProcessor.GetMessageType(), heartbeatProcessor)

	// Cyclic data processor
	cyclicProcessor := NewCollectCyclicDataProcessor(cs.logger, cs.collectJobService)
	cs.networkClient.RegisterProcessor(cyclicProcessor.GetMessageType(), cyclicProcessor)

	// Delete cyclic task processor
	deleteProcessor := NewDeleteCyclicTaskProcessor(cs.logger, cs.collectJobService)
	cs.networkClient.RegisterProcessor(deleteProcessor.GetMessageType(), deleteProcessor)

	// One-time data processor
	oneTimeProcessor := NewCollectOneTimeDataProcessor(cs.logger, cs.collectJobService)
	cs.networkClient.RegisterProcessor(oneTimeProcessor.GetMessageType(), oneTimeProcessor)

	// Online/Offline processors
	onlineProcessor := NewGoOnlineProcessor(cs.logger)
	cs.networkClient.RegisterProcessor(onlineProcessor.GetMessageType(), onlineProcessor)

	offlineProcessor := NewGoOfflineProcessor(cs.logger)
	cs.networkClient.RegisterProcessor(offlineProcessor.GetMessageType(), offlineProcessor)

	// Close processor
	closeProcessor := NewGoCloseProcessor(cs.logger)
	cs.networkClient.RegisterProcessor(closeProcessor.GetMessageType(), closeProcessor)
}

// Start starts the collect server and all its components.
func (cs *CollectServer) Start() error {
	cs.startMutex.Lock()
	defer cs.startMutex.Unlock()

	if cs.isStarted {
		return fmt.Errorf("collect server is already started")
	}

	cs.logger.Info("starting collect server",
		"managerHost", cs.config.Client.ManagerHost,
		"managerPort", cs.config.Client.ManagerPort,
		"collectorIdentity", cs.config.Collector.Identity)

	// Start worker pool
	if err := cs.workerPool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start timer dispatcher
	if err := cs.timerDispatcher.Start(); err != nil {
		cs.workerPool.Stop() // Cleanup on failure
		return fmt.Errorf("failed to start timer dispatcher: %w", err)
	}

	// Start network client (this will establish connection and send GO_ONLINE)
	if err := cs.networkClient.Start(); err != nil {
		cs.timerDispatcher.Stop() // Cleanup on failure
		cs.workerPool.Stop()
		return fmt.Errorf("failed to start network client: %w", err)
	}

	cs.isStarted = true
	cs.logger.Info("collect server started successfully")
	return nil
}

// Stop stops the collect server and all its components.
func (cs *CollectServer) Stop() error {
	cs.startMutex.Lock()
	defer cs.startMutex.Unlock()

	if !cs.isStarted {
		return fmt.Errorf("collect server is not started")
	}

	cs.logger.Info("stopping collect server")

	// Stop components in reverse order
	var errors []error

	// Stop network client
	if err := cs.networkClient.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop network client: %w", err))
	}

	// Stop timer dispatcher
	if err := cs.timerDispatcher.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop timer dispatcher: %w", err))
	}

	// Stop worker pool
	if err := cs.workerPool.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop worker pool: %w", err))
	}

	cs.isStarted = false

	if len(errors) > 0 {
		cs.logger.Error(fmt.Errorf("errors during shutdown: %v", errors), "collect server stopped with errors")
		return errors[0]
	}

	cs.logger.Info("collect server stopped successfully")
	return nil
}

// IsStarted returns true if the server is started.
func (cs *CollectServer) IsStarted() bool {
	cs.startMutex.Lock()
	defer cs.startMutex.Unlock()
	return cs.isStarted
}

// ReceiveJob simulates receiving a job from the manager and adds it to the dispatcher.
// This method is used for testing and simulation purposes when network communication is not available.
func (cs *CollectServer) ReceiveJob(job *jobtypes.Job, eventListener timer.CollectResponseEventListener) error {
	if !cs.isStarted {
		return fmt.Errorf("collect server is not started")
	}

	cs.logger.Info("received job from manager (simulated)",
		"jobId", job.ID,
		"monitorId", job.MonitorID,
		"app", job.App,
		"category", job.Category,
		"isCyclic", job.IsCyclic,
		"interval", job.DefaultInterval)

	// Add the job to the timer dispatcher for scheduling
	if err := cs.timerDispatcher.AddJob(job, eventListener); err != nil {
		cs.logger.Error(err, "failed to add received job to timer dispatcher",
			"jobId", job.ID,
			"monitorId", job.MonitorID)
		return fmt.Errorf("failed to schedule received job: %w", err)
	}

	cs.logger.Info("job successfully scheduled for execution",
		"jobId", job.ID,
		"monitorId", job.MonitorID,
		"nextExecution", "immediate")

	return nil
}

// GetCollectJobService returns the collect job service.
func (cs *CollectServer) GetCollectJobService() *CollectJobService {
	return cs.collectJobService
}

// GetNetworkClient returns the network client.
func (cs *CollectServer) GetNetworkClient() *NetworkClient {
	return cs.networkClient
}

// GetStats returns statistics about the server components.
func (cs *CollectServer) GetStats() ServerStats {
	var timerStats timer.TimerDispatcherStats

	if cs.timerDispatcher != nil {
		timerStats = cs.timerDispatcher.GetStats()
	}

	return ServerStats{
		IsStarted:         cs.isStarted,
		IsConnected:       cs.networkClient != nil && cs.networkClient.IsConnected(),
		CollectorIdentity: cs.config.Collector.Identity,
		CollectorMode:     cs.config.Collector.Mode,
		TimerStats:        timerStats,
	}
}

// ServerStats contains statistics about the server.
type ServerStats struct {
	IsStarted         bool                       `json:"isStarted"`
	IsConnected       bool                       `json:"isConnected"`
	CollectorIdentity string                     `json:"collectorIdentity"`
	CollectorMode     string                     `json:"collectorMode"`
	TimerStats        timer.TimerDispatcherStats `json:"timerStats"`
}
