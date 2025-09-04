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
	"context"
	"encoding/json"
	"fmt"

	"hertzbeat.apache.org/hertzbeat-collector-go/api"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

// CollectJobServiceInterface defines the interface that CollectJobService should implement.
type CollectJobServiceInterface interface {
	AddAsyncCollectJob(job *jobtypes.Job) error
	CancelAsyncCollectJob(jobID int64) error
	CollectSyncOneTimeJobData(job *jobtypes.Job) error
}

// HeartbeatProcessor handles heartbeat messages.
type HeartbeatProcessor struct {
	logger logger.Logger
}

// NewHeartbeatProcessor creates a new HeartbeatProcessor.
func NewHeartbeatProcessor(logger logger.Logger) *HeartbeatProcessor {
	return &HeartbeatProcessor{
		logger: logger.WithName("heartbeat-processor"),
	}
}

// Process processes a heartbeat message.
func (hp *HeartbeatProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	hp.logger.Info("received heartbeat message", "identity", message.Identity)
	// Heartbeat messages typically don't require any action
	return nil
}

// GetMessageType returns the message type this processor handles.
func (hp *HeartbeatProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_HEARTBEAT
}

// CollectCyclicDataProcessor handles cyclic collection task assignment.
type CollectCyclicDataProcessor struct {
	logger            logger.Logger
	collectJobService CollectJobServiceInterface
}

// NewCollectCyclicDataProcessor creates a new CollectCyclicDataProcessor.
func NewCollectCyclicDataProcessor(logger logger.Logger, collectJobService CollectJobServiceInterface) *CollectCyclicDataProcessor {
	return &CollectCyclicDataProcessor{
		logger:            logger.WithName("cyclic-data-processor"),
		collectJobService: collectJobService,
	}
}

// Process processes a cyclic collection task message.
func (cp *CollectCyclicDataProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	cp.logger.Info("received cyclic collection task", "identity", message.Identity)

	// Deserialize job from message
	job, err := cp.deserializeJob(message.Msg)
	if err != nil {
		return fmt.Errorf("failed to deserialize job: %w", err)
	}

	// Add the job to the collection service
	if err := cp.collectJobService.AddAsyncCollectJob(job); err != nil {
		return fmt.Errorf("failed to add cyclic collect job: %w", err)
	}

	cp.logger.Info("successfully added cyclic collect job", "jobId", job.ID, "app", job.App)
	return nil
}

// GetMessageType returns the message type this processor handles.
func (cp *CollectCyclicDataProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_ISSUE_CYCLIC_TASK
}

// DeleteCyclicTaskProcessor handles cyclic collection task deletion.
type DeleteCyclicTaskProcessor struct {
	logger            logger.Logger
	collectJobService CollectJobServiceInterface
}

// NewDeleteCyclicTaskProcessor creates a new DeleteCyclicTaskProcessor.
func NewDeleteCyclicTaskProcessor(logger logger.Logger, collectJobService CollectJobServiceInterface) *DeleteCyclicTaskProcessor {
	return &DeleteCyclicTaskProcessor{
		logger:            logger.WithName("delete-cyclic-processor"),
		collectJobService: collectJobService,
	}
}

// Process processes a delete cyclic collection task message.
func (dp *DeleteCyclicTaskProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	dp.logger.Info("received delete cyclic task", "identity", message.Identity)

	// Parse job ID from message
	var request struct {
		JobID int64 `json:"jobId"`
	}
	if err := json.Unmarshal(message.Msg, &request); err != nil {
		return fmt.Errorf("failed to parse delete request: %w", err)
	}

	// Delete the job
	if err := dp.collectJobService.CancelAsyncCollectJob(request.JobID); err != nil {
		return fmt.Errorf("failed to cancel collect job: %w", err)
	}

	dp.logger.Info("successfully deleted cyclic collect job", "jobId", request.JobID)
	return nil
}

// GetMessageType returns the message type this processor handles.
func (dp *DeleteCyclicTaskProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_DELETE_CYCLIC_TASK
}

// CollectOneTimeDataProcessor handles one-time collection task assignment.
type CollectOneTimeDataProcessor struct {
	logger            logger.Logger
	collectJobService CollectJobServiceInterface
}

// NewCollectOneTimeDataProcessor creates a new CollectOneTimeDataProcessor.
func NewCollectOneTimeDataProcessor(logger logger.Logger, collectJobService CollectJobServiceInterface) *CollectOneTimeDataProcessor {
	return &CollectOneTimeDataProcessor{
		logger:            logger.WithName("onetime-data-processor"),
		collectJobService: collectJobService,
	}
}

// Process processes a one-time collection task message.
func (op *CollectOneTimeDataProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	op.logger.Info("received one-time collection task", "identity", message.Identity)

	// Deserialize job from message
	job, err := op.deserializeJob(message.Msg)
	if err != nil {
		return fmt.Errorf("failed to deserialize job: %w", err)
	}

	// Execute the one-time job
	if err := op.collectJobService.CollectSyncOneTimeJobData(job); err != nil {
		return fmt.Errorf("failed to execute one-time collect job: %w", err)
	}

	op.logger.Info("successfully started one-time collect job", "jobId", job.ID, "app", job.App)
	return nil
}

// GetMessageType returns the message type this processor handles.
func (op *CollectOneTimeDataProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_ISSUE_ONE_TIME_TASK
}

// GoOfflineProcessor handles go offline requests.
type GoOfflineProcessor struct {
	logger logger.Logger
}

// NewGoOfflineProcessor creates a new GoOfflineProcessor.
func NewGoOfflineProcessor(logger logger.Logger) *GoOfflineProcessor {
	return &GoOfflineProcessor{
		logger: logger.WithName("offline-processor"),
	}
}

// Process processes a go offline message.
func (gop *GoOfflineProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	gop.logger.Info("received go offline request", "identity", message.Identity)
	// TODO: Implement offline logic (pause collection, etc.)
	return nil
}

// GetMessageType returns the message type this processor handles.
func (gop *GoOfflineProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_GO_OFFLINE
}

// GoOnlineProcessor handles go online requests.
type GoOnlineProcessor struct {
	logger logger.Logger
}

// NewGoOnlineProcessor creates a new GoOnlineProcessor.
func NewGoOnlineProcessor(logger logger.Logger) *GoOnlineProcessor {
	return &GoOnlineProcessor{
		logger: logger.WithName("online-processor"),
	}
}

// Process processes a go online message.
func (gop *GoOnlineProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	gop.logger.Info("received go online request", "identity", message.Identity)
	// TODO: Implement online logic (resume collection, etc.)
	return nil
}

// GetMessageType returns the message type this processor handles.
func (gop *GoOnlineProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_GO_ONLINE
}

// GoCloseProcessor handles go close requests.
type GoCloseProcessor struct {
	logger logger.Logger
}

// NewGoCloseProcessor creates a new GoCloseProcessor.
func NewGoCloseProcessor(logger logger.Logger) *GoCloseProcessor {
	return &GoCloseProcessor{
		logger: logger.WithName("close-processor"),
	}
}

// Process processes a go close message.
func (gcp *GoCloseProcessor) Process(ctx context.Context, message *cluster_msg.Message) error {
	gcp.logger.Info("received go close request", "identity", message.Identity)
	// TODO: Implement close logic (shutdown gracefully, etc.)
	return nil
}

// GetMessageType returns the message type this processor handles.
func (gcp *GoCloseProcessor) GetMessageType() cluster_msg.MessageType {
	return cluster_msg.MessageType_GO_CLOSE
}

// deserializeJob deserializes a job from message bytes.
func (cp *CollectCyclicDataProcessor) deserializeJob(data []byte) (*jobtypes.Job, error) {
	job := &jobtypes.Job{}
	if err := json.Unmarshal(data, job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}
	return job, nil
}

// deserializeJob deserializes a job from message bytes.
func (op *CollectOneTimeDataProcessor) deserializeJob(data []byte) (*jobtypes.Job, error) {
	job := &jobtypes.Job{}
	if err := json.Unmarshal(data, job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}
	return job, nil
}
