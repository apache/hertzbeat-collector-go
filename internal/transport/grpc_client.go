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
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
)

// ResponseFuture represents a future response for sync calls
type ResponseFuture struct {
	response chan *pb.Message
	error    chan error
}

func NewResponseFuture() *ResponseFuture {
	return &ResponseFuture{
		response: make(chan *pb.Message, 1),
		error:    make(chan error, 1),
	}
}

func (f *ResponseFuture) Wait(timeout time.Duration) (*pb.Message, error) {
	select {
	case resp := <-f.response:
		return resp, nil
	case err := <-f.error:
		return nil, err
	case <-time.After(timeout):
		return nil, context.DeadlineExceeded
	}
}

func (f *ResponseFuture) PutResponse(resp *pb.Message) {
	f.response <- resp
}

func (f *ResponseFuture) PutError(err error) {
	f.error <- err
}

// EventType represents connection event types
type EventType int

const (
	EventConnected EventType = iota
	EventDisconnected
	EventConnectFailed
)

// Event represents a connection event
type Event struct {
	Type    EventType
	Address string
	Error   error
}

// EventHandler handles connection events
type EventHandler func(event Event)

// GrpcClient implements TransportClient using gRPC.
type GrpcClient struct {
	conn          *grpc.ClientConn
	client        pb.ClusterMsgServiceClient
	addr          string
	started       bool
	mu            sync.RWMutex
	registry      *ProcessorRegistry
	responseTable map[string]*ResponseFuture
	eventHandler  EventHandler
	cancel        context.CancelFunc
}

func NewGrpcClient(addr string) *GrpcClient {
	return &GrpcClient{
		addr:          addr,
		registry:      NewProcessorRegistry(),
		responseTable: make(map[string]*ResponseFuture),
		eventHandler:  defaultEventHandler,
	}
}

func defaultEventHandler(event Event) {
	// Default event handler
	log.Printf("Connection event: Type=%d, Address=%s, Error=%v", event.Type, event.Address, event.Error)
}

func (c *GrpcClient) SetEventHandler(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = handler
}

func (c *GrpcClient) triggerEvent(eventType EventType, err error) {
	if c.eventHandler != nil {
		c.eventHandler(Event{
			Type:    eventType,
			Address: c.addr,
			Error:   err,
		})
	}
}

func (c *GrpcClient) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}

	_, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	conn, err := grpc.Dial(c.addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		c.triggerEvent(EventConnectFailed, err)
		return err
	}
	c.conn = conn
	c.client = pb.NewClusterMsgServiceClient(conn)
	c.started = true

	c.triggerEvent(EventConnected, nil)

	go c.heartbeatLoop()
	go c.connectionMonitor()
	go c.streamMsgLoop()
	return nil
}

func (c *GrpcClient) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.started = false
	c.triggerEvent(EventDisconnected, nil)
	return nil
}

func (c *GrpcClient) IsStarted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.started
}

// processor: func(msg interface{}) (resp interface{}, err error)
func (c *GrpcClient) RegisterProcessor(msgType int32, processor ProcessorFunc) {
	c.registry.Register(msgType, processor)
}

func (c *GrpcClient) SendMsg(msg interface{}) error {
	pbMsg, ok := msg.(*pb.Message)
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := c.client.SendMsg(ctx, pbMsg)
	return err
}

func (c *GrpcClient) SendMsgSync(msg interface{}, timeoutMillis int) (interface{}, error) {
	pbMsg, ok := msg.(*pb.Message)
	if !ok {
		return nil, nil
	}

	// Use the existing identity as correlation ID
	// If empty, generate a new one
	if pbMsg.Identity == "" {
		pbMsg.Identity = generateCorrelationID()
	}

	// Create response future for this request
	future := NewResponseFuture()
	c.responseTable[pbMsg.Identity] = future
	defer delete(c.responseTable, pbMsg.Identity)

	// Send message
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMillis)*time.Millisecond)
	defer cancel()

	resp, err := c.client.SendMsg(ctx, pbMsg)
	if err != nil {
		future.PutError(err)
		return nil, err
	}

	// Check if this is a response to our request
	if resp != nil && resp.Identity == pbMsg.Identity {
		future.PutResponse(resp)
		return resp, nil
	}

	// If no immediate response, wait for async response
	return future.Wait(time.Duration(timeoutMillis) * time.Millisecond)
}

func generateCorrelationID() string {
	return time.Now().Format("20060102150405.999999999") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().Nanosecond()%len(charset)]
	}
	return string(b)
}

func (c *GrpcClient) connectionMonitor() {
	for c.IsStarted() {
		time.Sleep(5 * time.Second)
		if c.conn != nil && c.conn.GetState() != connectivity.Ready {
			c.triggerEvent(EventDisconnected, nil)
			log.Println("gRPC connection lost, attempting to reconnect...")
			_ = c.Shutdown()
			if err := c.Start(); err != nil {
				c.triggerEvent(EventConnectFailed, err)
				log.Printf("Failed to reconnect: %v", err)
			}
		}
	}
}

func (c *GrpcClient) heartbeatLoop() {
	for c.IsStarted() {
		// 发送心跳消息
		heartbeat := &pb.Message{
			Type:      pb.MessageType_HEARTBEAT,
			Direction: pb.Direction_REQUEST,
			Identity:  "collector-go", // 可根据实际配置
		}
		_, _ = c.SendMsgSync(heartbeat, 2000)
		time.Sleep(10 * time.Second)
	}
}

// StreamMsg 双向流式通信（可用于实时推送/心跳/任务下发等）
func (c *GrpcClient) streamMsgLoop() {
	ctx := context.Background()
	stream, err := c.client.StreamMsg(ctx)
	if err != nil {
		log.Printf("streamMsgLoop error: %v", err)
		return
	}

	// Start receiving messages
	go func() {
		for c.IsStarted() {
			in, err := stream.Recv()
			if err != nil {
				log.Printf("streamMsgLoop recv error: %v", err)
				return
			}

			// Process the received message
			c.processReceivedMessage(in)
		}
	}()

	// Keep the stream open
	<-ctx.Done()
}

func (c *GrpcClient) processReceivedMessage(msg *pb.Message) {
	// Check if this is a response to a sync request
	if msg.Direction == pb.Direction_RESPONSE {
		if future, ok := c.responseTable[msg.Identity]; ok {
			future.PutResponse(msg)
			return
		}
	}

	// If not a sync response, distribute to registered processors
	if fn, ok := c.registry.Get(int32(msg.Type)); ok {
		go fn(msg)
	}
}
