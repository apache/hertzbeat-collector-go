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
	"fmt"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
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
	MessageTypeResponseCyclicTaskData   int32 = 7
	MessageTypeResponseOneTimeTaskData  int32 = 8
	MessageTypeResponseCyclicTaskSdData int32 = 9
)

// MessageProcessor defines the interface for processing messages
type MessageProcessor interface {
	Process(msg *pb.Message) (*pb.Message, error)
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
	// Handle go online message
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
	// Handle go offline message - shutdown the client
	if p.client != nil {
		// Stop heartbeat and close connection
		p.client.Shutdown()
	}
	return &pb.Message{
		Type:      pb.MessageType_GO_OFFLINE,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("offline ack"),
	}, nil
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
	client *GrpcClient
}

func NewCollectCyclicDataProcessor(client *GrpcClient) *CollectCyclicDataProcessor {
	return &CollectCyclicDataProcessor{client: client}
}

func (p *CollectCyclicDataProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	// Handle cyclic task message
	// TODO: Implement actual task processing logic
	return &pb.Message{
		Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("cyclic task ack"),
	}, nil
}

// DeleteCyclicTaskProcessor handles delete cyclic task messages
type DeleteCyclicTaskProcessor struct {
	client *GrpcClient
}

func NewDeleteCyclicTaskProcessor(client *GrpcClient) *DeleteCyclicTaskProcessor {
	return &DeleteCyclicTaskProcessor{client: client}
}

func (p *DeleteCyclicTaskProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	// Handle delete cyclic task message
	return &pb.Message{
		Type:      pb.MessageType_DELETE_CYCLIC_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("delete cyclic task ack"),
	}, nil
}

// CollectOneTimeDataProcessor handles one-time task messages
type CollectOneTimeDataProcessor struct {
	client *GrpcClient
}

func NewCollectOneTimeDataProcessor(client *GrpcClient) *CollectOneTimeDataProcessor {
	return &CollectOneTimeDataProcessor{client: client}
}

func (p *CollectOneTimeDataProcessor) Process(msg *pb.Message) (*pb.Message, error) {
	// Handle one-time task message
	// TODO: Implement actual task processing logic
	return &pb.Message{
		Type:      pb.MessageType_ISSUE_ONE_TIME_TASK,
		Direction: pb.Direction_RESPONSE,
		Identity:  msg.Identity,
		Msg:       []byte("one-time task ack"),
	}, nil
}

// RegisterDefaultProcessors registers all default message processors
func RegisterDefaultProcessors(client *GrpcClient) {
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
			processor := NewCollectCyclicDataProcessor(client)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeDeleteCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewDeleteCyclicTaskProcessor(client)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueOneTimeTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewCollectOneTimeDataProcessor(client)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})
}
