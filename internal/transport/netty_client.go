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
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api/cluster_msg"
	"google.golang.org/protobuf/proto"
)

// NettyClient implements a Netty-compatible client for Java server communication
type NettyClient struct {
	addr         string
	conn         net.Conn
	started      bool
	mu           sync.RWMutex
	registry     *ProcessorRegistry
	responseTable map[string]*ResponseFuture
	eventHandler EventHandler
	cancel       context.CancelFunc
	writer       *bufio.Writer
	reader       *bufio.Reader
	identity     string
}

func NewNettyClient(addr string) *NettyClient {
	return &NettyClient{
		addr:           addr,
		registry:       NewProcessorRegistry(),
		responseTable:  make(map[string]*ResponseFuture),
		eventHandler:   defaultEventHandler,
	}
}

// SetIdentity sets the collector identity
func (c *NettyClient) SetIdentity(identity string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.identity = identity
}

// GetIdentity returns the collector identity
func (c *NettyClient) GetIdentity() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.identity
}

func (c *NettyClient) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	
	_, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	
	// Connect to server
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		c.triggerEvent(EventConnectFailed, err)
		return err
	}
	c.conn = conn
	c.writer = bufio.NewWriter(conn)
	c.reader = bufio.NewReader(conn)
	c.started = true
	
	c.triggerEvent(EventConnected, nil)
	
	// Start background tasks
	go c.heartbeatLoop()
	go c.connectionMonitor()
	go c.readLoop()
	
	return nil
}

func (c *NettyClient) Shutdown() error {
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

func (c *NettyClient) IsStarted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.started
}

func (c *NettyClient) SetEventHandler(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = handler
}

func (c *NettyClient) triggerEvent(eventType EventType, err error) {
	if c.eventHandler != nil {
		c.eventHandler(Event{
			Type:    eventType,
			Address: c.addr,
			Error:   err,
		})
	}
}

func (c *NettyClient) RegisterProcessor(msgType int32, processor ProcessorFunc) {
	c.registry.Register(msgType, processor)
}

func (c *NettyClient) SendMsg(msg interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.started || c.conn == nil {
		return errors.New("client not started")
	}
	
	pbMsg, ok := msg.(*pb.Message)
	if !ok {
		return errors.New("invalid message type")
	}
	
	return c.writeMessage(pbMsg)
}

func (c *NettyClient) SendMsgSync(msg interface{}, timeoutMillis int) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.started || c.conn == nil {
		return nil, errors.New("client not started")
	}
	
	pbMsg, ok := msg.(*pb.Message)
	if !ok {
		return nil, errors.New("invalid message type")
	}
	
	// Use the existing identity as correlation ID
	if pbMsg.Identity == "" {
		pbMsg.Identity = generateCorrelationID()
	}
	
	// Create response future for this request
	future := NewResponseFuture()
	c.responseTable[pbMsg.Identity] = future
	defer delete(c.responseTable, pbMsg.Identity)
	
	// Send message
	if err := c.writeMessage(pbMsg); err != nil {
		future.PutError(err)
		return nil, err
	}
	
	// Wait for response
	return future.Wait(time.Duration(timeoutMillis) * time.Millisecond)
}

func (c *NettyClient) writeMessage(msg *pb.Message) error {
	// Set write deadline to prevent hanging
	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	
	// Serialize protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Create the inner message: length prefix + protobuf data
	// This matches Java Netty's expectation: GZIP decompression -> ProtobufVarint32FrameDecoder -> ProtobufDecoder
	var innerMessage bytes.Buffer
	
	// Add length prefix for the protobuf data (using varint32 format)
	length := uint32(len(data))
	var buf [binary.MaxVarintLen32]byte
	n := binary.PutUvarint(buf[:], uint64(length))
	
	if _, err := innerMessage.Write(buf[:n]); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}
	
	// Add the protobuf data
	if _, err := innerMessage.Write(data); err != nil {
		return fmt.Errorf("failed to write protobuf data: %w", err)
	}
	
	innerData := innerMessage.Bytes()
	
	// Compress the entire inner message using GZIP
	// Java Netty ZlibWrapper.GZIP expects standard GZIP format
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write(innerData); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}
	
	compressedData := compressed.Bytes()
	
	// Debug: Log message details
	log.Printf("DEBUG: Writing message - Type: %d, Original size: %d, Inner size: %d, Compressed size: %d", 
		msg.Type, len(data), len(innerData), len(compressedData))
	
	// Write the compressed data directly (no additional length prefix needed)
	// The GZIP compressed data contains the length prefix + protobuf data inside
	if _, err := c.writer.Write(compressedData); err != nil {
		return fmt.Errorf("failed to write compressed message: %w", err)
	}
	
	// Flush
	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}
	
	// Clear write deadline
	if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
		log.Printf("Warning: failed to clear write deadline: %v", err)
	}
	
	log.Printf("DEBUG: Message written successfully")
	return nil
}

func (c *NettyClient) readLoop() {
	for c.IsStarted() {
		msg, err := c.readMessage()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("readLoop error: %v", err)
			}
			break
		}
		
		// Process the received message
		c.processReceivedMessage(msg)
	}
}

func (c *NettyClient) readMessage() (*pb.Message, error) {
	// Java Netty server sends GZIP compressed data that contains:
	// [length prefix + protobuf data] compressed with GZIP
	// We need to read the GZIP stream and decompress it
	
	// Create a gzip reader directly from the buffered reader
	// This will handle the GZIP stream boundaries automatically
	gzipReader, err := gzip.NewReader(c.reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()
	
	// Read the decompressed data (which contains length prefix + protobuf data)
	innerData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	
	// Parse the length prefix from the decompressed data
	innerReader := bytes.NewReader(innerData)
	length, err := binary.ReadUvarint(innerReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read length prefix from decompressed data: %w", err)
	}
	
	// Read the protobuf data
	protobufData := make([]byte, length)
	if _, err := innerReader.Read(protobufData); err != nil {
		return nil, fmt.Errorf("failed to read protobuf data: %w", err)
	}
	
	// Deserialize protobuf message
	msg := &pb.Message{}
	if err := proto.Unmarshal(protobufData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	return msg, nil
}

func (c *NettyClient) processReceivedMessage(msg *pb.Message) {
	// Check if this is a response to a sync request
	if msg.Direction == pb.Direction_RESPONSE {
		if future, ok := c.responseTable[msg.Identity]; ok {
			future.PutResponse(msg)
			return
		}
	}
	
	// If not a sync response, distribute to registered processors
	if fn, ok := c.registry.Get(int32(msg.Type)); ok {
		// For request messages that require response, process synchronously and send response back
		if msg.Direction == pb.Direction_REQUEST {
			go func() {
				response, err := fn(msg)
				if err != nil {
					log.Printf("Error processing message type %d: %v", msg.Type, err)
					return
				}
				
				if response != nil {
					// Send the response back to server
					if err := c.SendMsg(response); err != nil {
						log.Printf("Failed to send response for message type %d: %v", msg.Type, err)
					} else {
						log.Printf("Successfully sent response for message type %d", msg.Type)
					}
				}
			}()
		} else {
			// For non-request messages, process asynchronously
			go fn(msg)
		}
	}
}

func (c *NettyClient) connectionMonitor() {
	for c.IsStarted() {
		time.Sleep(5 * time.Second)
		if c.conn != nil {
			// Test connection by checking if it's still active
			_, err := c.conn.Write([]byte{})
			if err != nil {
				c.triggerEvent(EventDisconnected, nil)
				log.Println("Connection lost, attempting to reconnect...")
				_ = c.Shutdown()
				
				// Attempt to reconnect with backoff
				for i := 0; i < 5 && c.IsStarted(); i++ {
					backoff := time.Duration(i+1) * 2 * time.Second
					log.Printf("Attempting to reconnect in %v...", backoff)
					time.Sleep(backoff)
					
					if err := c.Start(); err == nil {
						log.Println("Reconnected successfully")
						break
					} else {
						log.Printf("Failed to reconnect: %v", err)
						if i == 4 { // Last attempt
							c.triggerEvent(EventConnectFailed, err)
						}
					}
				}
			}
		}
	}
}

func (c *NettyClient) heartbeatLoop() {
	for c.IsStarted() {
		// Send heartbeat message with configured identity
		identity := c.GetIdentity()
		if identity == "" {
			identity = "collector-go" // fallback identity
		}
		
		heartbeat := &pb.Message{
			Type:      pb.MessageType_HEARTBEAT,
			Direction: pb.Direction_REQUEST,
			Identity:  identity,
		}
		if err := c.SendMsg(heartbeat); err != nil {
			log.Printf("Failed to send heartbeat: %v", err)
		} else {
			log.Printf("Heartbeat sent successfully for identity: %s", identity)
		}
		time.Sleep(5 * time.Second) // Match Java version's 5-second interval
	}
}

// TransportClientFactory creates transport clients based on protocol type
type TransportClientFactory struct{}

func (f *TransportClientFactory) CreateClient(protocol, addr string) (TransportClient, error) {
	switch protocol {
	case "grpc":
		return NewGrpcClient(addr), nil
	case "netty":
		return NewNettyClient(addr), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// RegisterDefaultProcessors registers all default message processors for Netty client
func RegisterDefaultNettyProcessors(client *NettyClient) {
	client.RegisterProcessor(MessageTypeHeartbeat, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &HeartbeatProcessor{}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoOnline, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &GoOnlineProcessor{}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoOffline, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := NewGoOfflineProcessor(client)
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoClose, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &GoCloseProcessor{client: nil} // Netty client doesn't need shutdown
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &CollectCyclicDataProcessor{client: nil}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeDeleteCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &DeleteCyclicTaskProcessor{client: nil}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueOneTimeTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			processor := &CollectOneTimeDataProcessor{client: nil}
			return processor.Process(pbMsg)
		}
		return nil, fmt.Errorf("invalid message type")
	})
}
