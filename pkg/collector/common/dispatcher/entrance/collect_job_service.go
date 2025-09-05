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
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/api"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/timer"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/worker"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// CollectJobService manages collection jobs and provides API interface.
type CollectJobService struct {
	timerDispatcher   *timer.TimerDispatcher
	workerPool        *worker.WorkerPool
	collectorIdentity string
	collectorMode     string
	networkClient     *NetworkClient
	logger            logger.Logger

	// For synchronous job data collection
	responseListeners map[int64]chan []jobtypes.CollectRepMetricsData
	listenersMutex    sync.RWMutex
}

// NewCollectJobService creates a new CollectJobService instance.
func NewCollectJobService(
	timerDispatcher *timer.TimerDispatcher,
	workerPool *worker.WorkerPool,
	collectorConfig *CollectorConfig,
	logger logger.Logger,
) *CollectJobService {
	identity := collectorConfig.Identity
	if identity == "" {
		hostname, _ := os.Hostname()
		identity = hostname + "-collector"
	}

	return &CollectJobService{
		timerDispatcher:   timerDispatcher,
		workerPool:        workerPool,
		collectorIdentity: identity,
		collectorMode:     collectorConfig.Mode,
		logger:            logger.WithName("collect-job-service"),
		responseListeners: make(map[int64]chan []jobtypes.CollectRepMetricsData),
	}
}

// SetNetworkClient sets the network client for sending responses.
func (cjs *CollectJobService) SetNetworkClient(client *NetworkClient) {
	cjs.networkClient = client
}

// CollectSyncJobData executes a one-time collection task and returns the collected data.
func (cjs *CollectJobService) CollectSyncJobData(job *jobtypes.Job) ([]jobtypes.CollectRepMetricsData, error) {
	responseChan := make(chan []jobtypes.CollectRepMetricsData, 1)

	// Register response listener
	cjs.listenersMutex.Lock()
	cjs.responseListeners[job.MonitorID] = responseChan
	cjs.listenersMutex.Unlock()

	// Clean up listener after completion
	defer func() {
		cjs.listenersMutex.Lock()
		delete(cjs.responseListeners, job.MonitorID)
		cjs.listenersMutex.Unlock()
	}()

	// Add job to timer dispatcher
	if err := cjs.timerDispatcher.AddJob(job, nil); err != nil {
		return nil, fmt.Errorf("failed to add job to dispatcher: %w", err)
	}

	// Wait for response with timeout
	select {
	case metricsData := <-responseChan:
		return metricsData, nil
	case <-time.After(120 * time.Second):
		cjs.logger.Info("sync task runs for 120 seconds with no response, returning empty result",
			"jobId", job.MonitorID)
		return []jobtypes.CollectRepMetricsData{}, nil
	}
}

// CollectSyncOneTimeJobData executes a one-time collection task and sends the response.
func (cjs *CollectJobService) CollectSyncOneTimeJobData(job *jobtypes.Job) error {
	// Execute the job in a goroutine to avoid blocking
	return cjs.workerPool.Submit(&syncJobTask{
		job:     job,
		service: cjs,
		logger:  cjs.logger.WithName("sync-job-task"),
	})
}

// AddAsyncCollectJob adds a periodic asynchronous collection task.
func (cjs *CollectJobService) AddAsyncCollectJob(job *jobtypes.Job) error {
	// Clone the job and mark as cyclic
	jobCopy := *job
	jobCopy.IsCyclic = true

	return cjs.timerDispatcher.AddJob(&jobCopy, nil)
}

// CancelAsyncCollectJob cancels a periodic asynchronous collection task.
func (cjs *CollectJobService) CancelAsyncCollectJob(jobID int64) error {
	success := cjs.timerDispatcher.DeleteJob(jobID, true) // true for cyclic jobs
	if !success {
		return fmt.Errorf("failed to delete job %d", jobID)
	}
	return nil
}

// DispatchCollectData implements CollectDataDispatcher interface.
func (cjs *CollectJobService) DispatchCollectData(timeout *jobtypes.Timeout, metrics *jobtypes.Metrics, metricsData []*worker.MetricsData) error {
	// Convert worker.MetricsData to jobtypes.CollectRepMetricsData
	convertedData := make([]jobtypes.CollectRepMetricsData, 0, len(metricsData))
	for _, data := range metricsData {
		convertedData = append(convertedData, jobtypes.CollectRepMetricsData{
			ID:        data.ID,
			MonitorID: data.ID, // Use ID as MonitorID
			TenantID:  data.TenantID,
			App:       data.App,
			Metrics:   data.Metrics,
			Priority:  data.Priority,
			Time:      data.Time,
			Code:      data.Code,
			Msg:       data.Msg,
			Fields:    data.Fields,
			Values:    convertStringArraysToValueRows(data.Values),
			Labels:    data.Labels,
			Metadata:  data.Metadata,
		})
	}

	// For cyclic tasks, send the data asynchronously
	if len(convertedData) > 0 {
		for _, data := range convertedData {
			if err := cjs.SendAsyncCollectData(data); err != nil {
				cjs.logger.Error(err, "failed to send async collect data", "jobId", data.MonitorID)
				return err
			}
		}
	}

	// For sync tasks, notify the waiting listener
	if timeout != nil {
		if wheelTask, ok := timeout.Task().(*timer.WheelTimerTask); ok {
			job := wheelTask.GetJob()
			if job != nil && !job.IsCyclic {
				cjs.ResponseSyncJobData(job.MonitorID, convertedData)
			}
		}
	}

	return nil
}

// SendAsyncCollectData sends asynchronous collect response data to the manager.
func (cjs *CollectJobService) SendAsyncCollectData(metricsData jobtypes.CollectRepMetricsData) error {
	if cjs.networkClient == nil {
		return fmt.Errorf("network client is not set")
	}

	// Serialize metrics data
	data, err := cjs.serializeMetricsData([]jobtypes.CollectRepMetricsData{metricsData})
	if err != nil {
		return fmt.Errorf("failed to serialize metrics data: %w", err)
	}

	message := &cluster_msg.Message{
		Identity:  cjs.collectorIdentity,
		Direction: cluster_msg.Direction_REQUEST,
		Type:      cluster_msg.MessageType_RESPONSE_CYCLIC_TASK_DATA,
		Msg:       data,
	}

	return cjs.networkClient.SendMessage(message)
}

// SendAsyncServiceDiscoveryData sends asynchronous service discovery data to the manager.
func (cjs *CollectJobService) SendAsyncServiceDiscoveryData(metricsData jobtypes.CollectRepMetricsData) error {
	if cjs.networkClient == nil {
		return fmt.Errorf("network client is not set")
	}

	// Serialize metrics data
	data, err := cjs.serializeMetricsData([]jobtypes.CollectRepMetricsData{metricsData})
	if err != nil {
		return fmt.Errorf("failed to serialize metrics data: %w", err)
	}

	message := &cluster_msg.Message{
		Identity:  cjs.collectorIdentity,
		Direction: cluster_msg.Direction_REQUEST,
		Type:      cluster_msg.MessageType_RESPONSE_CYCLIC_TASK_SD_DATA,
		Msg:       data,
	}

	return cjs.networkClient.SendMessage(message)
}

// ResponseSyncJobData handles responses from synchronous jobs.
func (cjs *CollectJobService) ResponseSyncJobData(jobID int64, metricsData []jobtypes.CollectRepMetricsData) {
	cjs.listenersMutex.RLock()
	responseChan, exists := cjs.responseListeners[jobID]
	cjs.listenersMutex.RUnlock()

	if exists {
		select {
		case responseChan <- metricsData:
			// Response sent successfully
		default:
			// Channel is full or closed, ignore
		}
	}
}

// GetCollectorIdentity returns the collector identity.
func (cjs *CollectJobService) GetCollectorIdentity() string {
	return cjs.collectorIdentity
}

// GetCollectorMode returns the collector mode.
func (cjs *CollectJobService) GetCollectorMode() string {
	return cjs.collectorMode
}

// serializeMetricsData serializes metrics data for transmission.
func (cjs *CollectJobService) serializeMetricsData(metricsData []jobtypes.CollectRepMetricsData) ([]byte, error) {
	// TODO: Implement proper serialization using Apache Arrow format
	// For now, use JSON serialization as a placeholder
	return json.Marshal(metricsData)
}

// syncJobTask represents a synchronous job execution task.
type syncJobTask struct {
	job     *jobtypes.Job
	service *CollectJobService
	logger  logger.Logger
}

// Execute executes the synchronous job task.
func (sjt *syncJobTask) Execute() error {
	sjt.logger.Info("executing sync job task", "jobId", sjt.job.MonitorID)

	// Collect data synchronously
	metricsDataList, err := sjt.service.CollectSyncJobData(sjt.job)
	if err != nil {
		sjt.logger.Error(err, "failed to collect sync job data", "jobId", sjt.job.MonitorID)
		return err
	}

	// Serialize and send response
	if sjt.service.networkClient != nil {
		data, err := sjt.service.serializeMetricsData(metricsDataList)
		if err != nil {
			sjt.logger.Error(err, "failed to serialize response data", "jobId", sjt.job.MonitorID)
			return err
		}

		message := &cluster_msg.Message{
			Identity:  sjt.service.collectorIdentity,
			Direction: cluster_msg.Direction_REQUEST,
			Type:      cluster_msg.MessageType_RESPONSE_ONE_TIME_TASK_DATA,
			Msg:       data,
		}

		if err := sjt.service.networkClient.SendMessage(message); err != nil {
			sjt.logger.Error(err, "failed to send sync job response", "jobId", sjt.job.MonitorID)
			return err
		}
	}
	return nil
}

// Priority returns the priority of the sync job task.
func (sjt *syncJobTask) Priority() int {
	return 1 // High priority for one-time tasks
}

// Timeout returns the timeout for the sync job task.
func (sjt *syncJobTask) Timeout() time.Duration {
	return 120 * time.Second // 2 minutes timeout for sync jobs
}

// convertStringArraysToValueRows converts [][]string to []jobtypes.ValueRow
func convertStringArraysToValueRows(values [][]string) []jobtypes.ValueRow {
	valueRows := make([]jobtypes.ValueRow, len(values))
	for i, row := range values {
		valueRows[i] = jobtypes.ValueRow{Columns: row}
	}
	return valueRows
}
