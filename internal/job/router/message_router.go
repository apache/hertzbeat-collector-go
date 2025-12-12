/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package router

import (
	"encoding/json"
	"fmt"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/job/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/transport"
	job2 "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// MessageType constants matching Java version
const (
	MessageTypeHeartbeat                int32 = 0
	MessageTypeGoOnline                 int32 = 1
	MessageTypeGoOffline                int32 = 2
	MessageTypeGoClose                  int32 = 3
	MessageTypeIssueCyclicTask          int32 = 4
	MessageTypeDeleteCyclicTask         int32 = 5
	MessageTypeIssueOneTimeTask         int32 = 6
	MessageTypeResponseOneTimeTaskData  int32 = 7
	MessageTypeResponseCyclicTaskData   int32 = 8
	MessageTypeResponseCyclicTaskSdData int32 = 9
)

// MessageRouter is the interface for routing messages between transport and job components
type MessageRouter interface {
	// RegisterProcessors registers all message processors
	RegisterProcessors()

	// SendResult sends collection results back to the manager
	SendResult(data *job2.CollectRepMetricsData, job *job2.Job) error

	// GetIdentity returns the collector identity
	GetIdentity() string
}

// MessageRouterImpl implements the MessageRouter interface
type MessageRouterImpl struct {
	logger    logger.Logger
	client    transport.TransportClient
	jobRunner *server.Runner
	identity  string
}

// Config represents message router configuration
type Config struct {
	Logger    logger.Logger
	Client    transport.TransportClient
	JobRunner *server.Runner
	Identity  string
}

// New creates a new message router
func New(cfg *Config) MessageRouter {
	return &MessageRouterImpl{
		logger:    cfg.Logger.WithName("message-router"),
		client:    cfg.Client,
		jobRunner: cfg.JobRunner,
		identity:  cfg.Identity,
	}
}

// RegisterProcessors registers all message processors
func (r *MessageRouterImpl) RegisterProcessors() {
	if r.client == nil {
		r.logger.Error(nil, "cannot register processors: client is nil")
		return
	}

	// Register cyclic task processor
	r.client.RegisterProcessor(MessageTypeIssueCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			return r.handleIssueCyclicTask(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	// Register delete cyclic task processor
	r.client.RegisterProcessor(MessageTypeDeleteCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			return r.handleDeleteCyclicTask(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	// Register one-time task processor
	r.client.RegisterProcessor(MessageTypeIssueOneTimeTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			return r.handleIssueOneTimeTask(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	// Other processors can be registered here
	r.logger.Info("Successfully registered all message processors")
}

// handleIssueCyclicTask handles cyclic task messages
func (r *MessageRouterImpl) handleIssueCyclicTask(msg *pb.Message) (*pb.Message, error) {
	r.logger.Info("Handling cyclic task message")

	// Parse job from message
	var job job2.Job
	if err := json.Unmarshal(msg.Msg, &job); err != nil {
		r.logger.Error(err, "failed to unmarshal job")
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  r.identity,
			Msg:       []byte("failed to unmarshal job"),
		}, nil
	}

	// Add job to job runner
	if err := r.jobRunner.AddAsyncCollectJob(&job); err != nil {
		r.logger.Error(err, "failed to add job to job runner", "jobID", job.ID)
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  r.identity,
			Msg:       []byte(fmt.Sprintf("failed to add job: %v", err)),
		}, nil
	}

	return &pb.Message{
		Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  r.identity,
		Msg:       []byte("job added successfully"),
	}, nil
}

// handleDeleteCyclicTask handles delete cyclic task messages
func (r *MessageRouterImpl) handleDeleteCyclicTask(msg *pb.Message) (*pb.Message, error) {
	r.logger.Info("Handling delete cyclic task message")

	// Parse job IDs from message
	var jobIDs []int64
	if err := json.Unmarshal(msg.Msg, &jobIDs); err != nil {
		r.logger.Error(err, "failed to unmarshal job IDs")
		return &pb.Message{
			Type:      pb.MessageType_DELETE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  r.identity,
			Msg:       []byte("failed to unmarshal job IDs"),
		}, nil
	}

	// Remove jobs from job runner
	for _, jobID := range jobIDs {
		if err := r.jobRunner.RemoveAsyncCollectJob(jobID); err != nil {
			r.logger.Error(err, "failed to remove job from job runner", "jobID", jobID)
			// Continue with other jobs even if one fails
		}
	}

	return &pb.Message{
		Type:      pb.MessageType_DELETE_CYCLIC_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  r.identity,
		Msg:       []byte("jobs removed successfully"),
	}, nil
}

// handleIssueOneTimeTask handles one-time task messages
func (r *MessageRouterImpl) handleIssueOneTimeTask(msg *pb.Message) (*pb.Message, error) {
	r.logger.Info("Handling one-time task message")

	// Parse job from message
	var job job2.Job
	if err := json.Unmarshal(msg.Msg, &job); err != nil {
		r.logger.Error(err, "failed to unmarshal job")
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  r.identity,
			Msg:       []byte("failed to unmarshal job"),
		}, nil
	}

	// Set job as non-cyclic
	job.IsCyclic = false

	// Add job to job runner
	if err := r.jobRunner.AddAsyncCollectJob(&job); err != nil {
		r.logger.Error(err, "failed to add one-time job to job runner", "jobID", job.ID)
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  r.identity,
			Msg:       []byte(fmt.Sprintf("failed to add one-time job: %v", err)),
		}, nil
	}

	return &pb.Message{
		Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  r.identity,
		Msg:       []byte("one-time job added successfully"),
	}, nil
}

// SendResult sends collection results back to the manager
func (r *MessageRouterImpl) SendResult(data *job2.CollectRepMetricsData, job *job2.Job) error {
	if r.client == nil || !r.client.IsStarted() {
		return fmt.Errorf("transport client not started")
	}

	// Determine message type based on job type
	var msgType pb.MessageType
	if job.IsCyclic {
		msgType = pb.MessageType_RESPONSE_CYCLIC_TASK_DATA
	} else {
		msgType = pb.MessageType_RESPONSE_ONE_TIME_TASK_DATA
	}

	// Serialize metrics data
	dataBytes, err := json.Marshal([]job2.CollectRepMetricsData{*data})
	if err != nil {
		return fmt.Errorf("failed to marshal metrics data: %w", err)
	}

	// Create message
	msg := &pb.Message{
		Type:      msgType,
		Direction: pb.Direction_RESPONSE,
		Identity:  r.identity,
		Msg:       dataBytes,
	}

	// Send message
	if err := r.client.SendMsg(msg); err != nil {
		return fmt.Errorf("failed to send metrics data: %w", err)
	}

	r.logger.Info("Successfully sent metrics data",
		"jobID", job.ID,
		"metricsName", data.Metrics,
		"isCyclic", job.IsCyclic)

	return nil
}

// GetIdentity returns the collector identity
func (r *MessageRouterImpl) GetIdentity() string {
	return r.identity
}
