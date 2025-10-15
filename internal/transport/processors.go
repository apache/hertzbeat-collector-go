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

package transport

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/crypto"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/param"
)

// Message type constants matching Java version
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

// MessageProcessor defines the interface for processing messages
type MessageProcessor interface {
	Process(msg *pb.Message) (*pb.Message, error)
}

// JobScheduler defines the interface for job scheduling operations
// This interface allows transport layer to interact with job scheduling without direct dependencies
type JobScheduler interface {
	// AddAsyncCollectJob adds a job to async collection scheduling
	AddAsyncCollectJob(job *jobtypes.Job) error

	// RemoveAsyncCollectJob removes a job from scheduling
	RemoveAsyncCollectJob(jobID int64) error
}

// preprocessJobPasswords is a helper function to decrypt passwords in job configmap
// This should be called once when receiving a job to avoid repeated decryption
func preprocessJobPasswords(job *jobtypes.Job) error {
	replacer := param.NewReplacer()
	return replacer.PreprocessJobPasswords(job)
}

// HeartbeatProcessor handles heartbeat messages
type HeartbeatProcessor struct{}

func (p *HeartbeatProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	// Java version logs receipt of heartbeat response and returns null (no response needed)
	// This matches: log.info("collector receive manager server response heartbeat, time: {}. ", System.currentTimeMillis());
	return nil, nil
}

// GoOnlineProcessor handles go online messages
type GoOnlineProcessor struct {
	client *GrpcClient
}

func NewGoOnlineProcessor(client *GrpcClient) *GoOnlineProcessor {
	return &GoOnlineProcessor{client: client}
}

func (p *GoOnlineProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelInfo).WithName("go-online-processor")

	// Handle go online message - parse ServerInfo and extract AES secret
	log.Info("received GO_ONLINE message from manager")

	if len(msg.Msg) == 0 {
		log.Info("empty message from server, please upgrade server")
	} else {
		// Parse ServerInfo from message (matches Java: JsonUtil.fromJson(message.getMsg().toStringUtf8(), ServerInfo.class))
		var serverInfo struct {
			AesSecret string `json:"aesSecret"`
		}

		if err := json.Unmarshal(msg.Msg, &serverInfo); err != nil {
			log.Error(err, "failed to parse ServerInfo from manager")
		} else if serverInfo.AesSecret == "" {
			log.Info("server response has empty secret, please check configuration")
		} else {
			// Set the AES secret key globally (matches Java: AesUtil.setDefaultSecretKey(serverInfo.getAesSecret()))
			log.Info("received AES secret from manager", "keyLength", len(serverInfo.AesSecret))
			crypto.SetDefaultSecretKey(serverInfo.AesSecret)
		}
	}

	log.Info("online message processed successfully")
	return &pb.Message{
		Type:      pb.MessageType_GO_ONLINE,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("online ack"),
	}, nil
}

// GoOfflineProcessor handles go offline messages
type GoOfflineProcessor struct {
	client *NettyClient
}

func NewGoOfflineProcessor(client *NettyClient) *GoOfflineProcessor {
	return &GoOfflineProcessor{client: client}
}

func (p *GoOfflineProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	// Handle go offline message - first return response, then shutdown
	// Create response first
	response := &pb.Message{
		Type:      pb.MessageType_GO_OFFLINE,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("offline ack"),
	}

	// Schedule shutdown after a brief delay to allow response to be sent
	if p.client != nil {
		go func() {
			time.Sleep(100 * time.Millisecond) // Brief delay to ensure response is sent
			log.Printf("Shutting down client as requested by manager")
			p.client.Shutdown()
		}()
	}

	return response, nil
}

// GoCloseProcessor handles go close messages
type GoCloseProcessor struct {
	client *GrpcClient
}

func NewGoCloseProcessor(client *GrpcClient) *GoCloseProcessor {
	return &GoCloseProcessor{client: client}
}

func (p *GoCloseProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	// Handle go close message
	// Shutdown the client after receiving close message
	if p.client != nil {
		p.client.Shutdown()
	}
	return &pb.Message{
		Type:      pb.MessageType_GO_CLOSE,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("close ack"),
	}, nil
}

// CollectCyclicDataProcessor handles cyclic task messages
type CollectCyclicDataProcessor struct {
	client    *GrpcClient
	scheduler JobScheduler
}

func NewCollectCyclicDataProcessor(client *GrpcClient, scheduler JobScheduler) *CollectCyclicDataProcessor {
	return &CollectCyclicDataProcessor{
		client:    client,
		scheduler: scheduler,
	}
}

func (p *CollectCyclicDataProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelInfo).WithName("cyclic-task-processor")
	log.Info("processing cyclic task message", "identity", msg.Identity)

	if p.scheduler == nil {
		log.Info("no job scheduler available, cannot process cyclic task")
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte("no job scheduler available"),
		}, nil
	}

	// Parse job from message
	log.Info("Received cyclic task JSON: %s", string(msg.Msg))

	var job jobtypes.Job
	if err := json.Unmarshal(msg.Msg, &job); err != nil {
		log.Error(err, "Failed to unmarshal job from cyclic task message: %v")
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte(fmt.Sprintf("failed to parse job: %v", err)),
		}, nil
	}

	// Preprocess passwords once (decrypt encrypted passwords in configmap)
	// This avoids repeated decryption during each task execution
	if err := preprocessJobPasswords(&job); err != nil {
		log.Error(err, "Failed to preprocess job passwords")
	}

	// Ensure job is marked as cyclic
	job.IsCyclic = true

	log.Info("Adding cyclic job to scheduler",
		"jobID", job.ID,
		"monitorID", job.MonitorID,
		"app", job.App)

	// Add job to scheduler
	if err := p.scheduler.AddAsyncCollectJob(&job); err != nil {
		log.Error(err, "Failed to add cyclic job to scheduler: %v")
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte(fmt.Sprintf("failed to schedule job: %v", err)),
		}, nil
	}

	return &pb.Message{
		Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("cyclic task scheduled successfully"),
	}, nil
}

// DeleteCyclicTaskProcessor handles delete cyclic task messages
type DeleteCyclicTaskProcessor struct {
	client    *GrpcClient
	scheduler JobScheduler
}

func NewDeleteCyclicTaskProcessor(client *GrpcClient, scheduler JobScheduler) *DeleteCyclicTaskProcessor {
	return &DeleteCyclicTaskProcessor{
		client:    client,
		scheduler: scheduler,
	}
}

func (p *DeleteCyclicTaskProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	log.Printf("Processing delete cyclic task message, identity: %s", msg.Identity)

	if p.scheduler == nil {
		log.Printf("No job scheduler available, cannot process delete cyclic task")
		return &pb.Message{
			Type:      pb.MessageType_DELETE_CYCLIC_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte("no job scheduler available"),
		}, nil
	}

	// Parse job IDs from message - could be a single job ID or array of IDs
	var jobIDs []int64

	// Try to parse as array first
	if err := json.Unmarshal(msg.Msg, &jobIDs); err != nil {
		// If array parsing fails, try single job ID
		var singleJobID int64
		if err2 := json.Unmarshal(msg.Msg, &singleJobID); err2 != nil {
			log.Printf("Failed to unmarshal job IDs from delete message: %v, %v", err, err2)
			return &pb.Message{
				Type:      pb.MessageType_DELETE_CYCLIC_TASK,
				Direction: pb.Direction_RESPONSE,
				Identity:  msg.Identity,
				Msg:       []byte(fmt.Sprintf("failed to parse job IDs: %v", err)),
			}, nil
		}
		jobIDs = []int64{singleJobID}
	}

	log.Printf("Removing %d cyclic jobs from scheduler: %v", len(jobIDs), jobIDs)

	// Remove jobs from scheduler
	var errors []string
	for _, jobID := range jobIDs {
		if err := p.scheduler.RemoveAsyncCollectJob(jobID); err != nil {
			log.Printf("Failed to remove job %d from scheduler: %v", jobID, err)
			errors = append(errors, fmt.Sprintf("job %d: %v", jobID, err))
		} else {
			log.Printf("Successfully removed job %d from scheduler", jobID)
		}
	}

	// Prepare response message
	var responseMsg string
	if len(errors) > 0 {
		responseMsg = fmt.Sprintf("partially completed, errors: %v", errors)
	} else {
		responseMsg = "all cyclic tasks removed successfully"
	}

	return &pb.Message{
		Type:      pb.MessageType_DELETE_CYCLIC_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte(responseMsg),
	}, nil
}

// CollectOneTimeDataProcessor handles one-time task messages
type CollectOneTimeDataProcessor struct {
	client    *GrpcClient
	scheduler JobScheduler
}

func NewCollectOneTimeDataProcessor(client *GrpcClient, scheduler JobScheduler) *CollectOneTimeDataProcessor {
	return &CollectOneTimeDataProcessor{
		client:    client,
		scheduler: scheduler,
	}
}

func (p *CollectOneTimeDataProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	log.Printf("Processing one-time task message, identity: %s", msg.Identity)

	if p.scheduler == nil {
		log.Printf("No job scheduler available, cannot process one-time task")
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte("no job scheduler available"),
		}, nil
	}

	// Parse job from message
	log.Printf("Received one-time task JSON: %s", string(msg.Msg))

	// Parse JSON to check interval values before unmarshaling
	var rawJob map[string]interface{}
	if err := json.Unmarshal(msg.Msg, &rawJob); err == nil {
		if defaultInterval, ok := rawJob["defaultInterval"]; ok {
			log.Printf("DEBUG: Raw defaultInterval from JSON: %v (type: %T)", defaultInterval, defaultInterval)
		}
		if interval, ok := rawJob["interval"]; ok {
			log.Printf("DEBUG: Raw interval from JSON: %v (type: %T)", interval, interval)
		}
	}

	var job jobtypes.Job
	if err := json.Unmarshal(msg.Msg, &job); err != nil {
		log.Printf("Failed to unmarshal job from one-time task message: %v", err)
		log.Printf("JSON content: %s", string(msg.Msg))
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte(fmt.Sprintf("failed to parse job: %v", err)),
		}, nil
	}

	// Preprocess passwords once (decrypt encrypted passwords in configmap)
	// This avoids repeated decryption during each task execution
	if err := preprocessJobPasswords(&job); err != nil {
		log.Printf("Failed to preprocess job passwords: %v", err)
	}

	// Ensure job is marked as non-cyclic
	job.IsCyclic = false

	log.Printf("Adding one-time job to scheduler: jobID=%d, monitorID=%d, app=%s",
		job.ID, job.MonitorID, job.App)

	// Add job to scheduler
	if err := p.scheduler.AddAsyncCollectJob(&job); err != nil {
		log.Printf("Failed to add one-time job to scheduler: %v", err)
		return &pb.Message{
			Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
			Direction: pb.Direction_RESPONSE,
			Identity:  msg.Identity,
			Msg:       []byte(fmt.Sprintf("failed to schedule job: %v", err)),
		}, nil
	}

	log.Printf("Successfully scheduled one-time job: jobID=%d", job.ID)
	return &pb.Message{
		Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("one-time task scheduled successfully"),
	}, nil
}

// RegisterDefaultProcessors registers all default message processors
func RegisterDefaultProcessors(client *GrpcClient, scheduler JobScheduler) {
	client.RegisterProcessor(MessageTypeHeartbeat, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &HeartbeatProcessor{}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoOnline, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewGoOnlineProcessor(client)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoOffline, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &GoOfflineProcessor{}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoClose, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewGoCloseProcessor(client)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewCollectCyclicDataProcessor(client, scheduler)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeDeleteCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewDeleteCyclicTaskProcessor(client, scheduler)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueOneTimeTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewCollectOneTimeDataProcessor(client, scheduler)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})
}
