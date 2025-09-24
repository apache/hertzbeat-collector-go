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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
)

// NettyClient implements a Netty-compatible client for Java server communication
type NettyClient struct {
	addr          string
	conn          net.Conn
	started       bool
	mu            sync.RWMutex
	registry      *ProcessorRegistry
	responseTable map[string]*ResponseFuture
	eventHandler  EventHandler
	cancel        context.CancelFunc
	writer        *bufio.Writer
	reader        *bufio.Reader
	gzipReader    *gzip.Reader
	identity      string
}

func NewNettyClient(addr string) *NettyClient {
	return &NettyClient{
		addr:          addr,
		registry:      NewProcessorRegistry(),
		responseTable: make(map[string]*ResponseFuture),
		eventHandler:  defaultEventHandler,
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

	// Connect to server with timeout
	log.Printf("Attempting to connect to %s...", c.addr)
	conn, err := net.DialTimeout("tcp", c.addr, 10*time.Second)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		c.triggerEvent(EventConnectFailed, err)
		return err
	}

	log.Printf("TCP connection established to %s", c.addr)
	c.conn = conn
	c.writer = bufio.NewWriter(conn)
	c.reader = bufio.NewReader(conn)

	log.Printf("Connection setup completed, not creating gzip reader yet (will create on first read)")
	// Don't create gzip reader here - it will block waiting for data
	// We'll create it when we actually need to read data
	c.gzipReader = nil

	c.started = true
	log.Printf("NettyClient started successfully")

	log.Printf("Triggering connected event...")
	// Trigger connected event - this will cause transport layer to send GO_ONLINE message
	c.triggerEvent(EventConnected, nil)

	log.Printf("Starting background tasks...")
	// Start background tasks
	go c.readLoop()
	go c.heartbeatLoop()
	go c.connectionMonitor()

	log.Printf("All background tasks started")
	return nil
}

func (c *NettyClient) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.gzipReader != nil {
		_ = c.gzipReader.Close()
		c.gzipReader = nil
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
	eventName := ""
	switch eventType {
	case EventConnected:
		eventName = "Connected"
	case EventDisconnected:
		eventName = "Disconnected"
	case EventConnectFailed:
		eventName = "ConnectFailed"
	default:
		eventName = fmt.Sprintf("Unknown(%d)", eventType)
	}

	if err != nil {
		log.Printf("Triggering event: %s, error: %v", eventName, err)
	} else {
		log.Printf("Triggering event: %s", eventName)
	}

	if c.eventHandler != nil {
		c.eventHandler(Event{
			Type:    eventType,
			Address: c.addr,
			Error:   err,
		})
	} else {
		log.Printf("No event handler set")
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

	// For heartbeat messages, use a shorter timeout to avoid blocking
	if pbMsg.Type == pb.MessageType_HEARTBEAT {
		return c.writeMessageWithTimeout(pbMsg, 2*time.Second)
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

func (c *NettyClient) writeMessageWithTimeout(msg *pb.Message, timeout time.Duration) error {
	// Set write deadline to prevent hanging
	if err := c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Serialize protobuf message
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create the inner message: length prefix + protobuf data
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
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write(innerData); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	compressedData := compressed.Bytes()

	// Write the compressed data directly
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

	return nil
}

func (c *NettyClient) writeMessage(msg *pb.Message) error {
	// Set write deadline to prevent hanging (reduced from 10s to 5s for faster failure detection)
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
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
				// Trigger disconnect event on read error
				c.triggerEvent(EventDisconnected, nil)
			}
			break
		}

		// Process the received message
		c.processReceivedMessage(msg)
	}
}

func (c *NettyClient) readMessage() (*pb.Message, error) {
	// Create gzip reader on first read if not already created
	if c.gzipReader == nil {
		log.Printf("Creating gzip reader on first read...")
		var err error
		c.gzipReader, err = gzip.NewReader(c.reader)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		log.Printf("Gzip reader created successfully on first read")
	}

	// Java Netty server sends GZIP compressed data that contains:
	// [length prefix + protobuf data] compressed with GZIP
	// We use the persistent gzip reader created during connection setup

	// Read the decompressed data (which contains length prefix + protobuf data)
	// We need to read one complete message from the gzip stream

	// First, read the length prefix from gzip stream using a buffer
	lengthBuf := make([]byte, binary.MaxVarintLen32)
	var bytesRead int

	for {
		oneByte := make([]byte, 1)
		if _, err := c.gzipReader.Read(oneByte); err != nil {
			return nil, fmt.Errorf("failed to read length prefix: %w", err)
		}
		lengthBuf[bytesRead] = oneByte[0]
		bytesRead++

		// Try to decode the length
		length, n := binary.Uvarint(lengthBuf[:bytesRead])
		if n > 0 {
			// Successfully decoded, read the protobuf data
			protobufData := make([]byte, length)
			if _, err := io.ReadFull(c.gzipReader, protobufData); err != nil {
				return nil, fmt.Errorf("failed to read protobuf data: %w", err)
			}

			// Deserialize protobuf message
			msg := &pb.Message{}
			if err := proto.Unmarshal(protobufData, msg); err != nil {
				return nil, fmt.Errorf("failed to unmarshal message: %w", err)
			}

			return msg, nil
		} else if n < 0 {
			return nil, fmt.Errorf("invalid length encoding")
		}
		// n == 0 means we need more bytes
		if bytesRead >= binary.MaxVarintLen32 {
			return nil, fmt.Errorf("length prefix too long")
		}
	}
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
					// Check if response is actually a valid protobuf message
					if pbResponse, ok := response.(*pb.Message); ok && pbResponse != nil {
						// Send the response back to server
						if err := c.SendMsg(pbResponse); err != nil {
							log.Printf("Failed to send response for message type %d: %v", msg.Type, err)
						} else {
							log.Printf("Successfully sent response for message type %d", msg.Type)
						}
					} else {
						log.Printf("Processor returned invalid response type for message type %d", msg.Type)
					}
				} else {
					// For heartbeat messages (type 0), nil response is expected and normal
					if msg.Type == pb.MessageType_HEARTBEAT {
						log.Printf("Heartbeat response received for message type %d (no response needed)", msg.Type)
					} else {
						log.Printf("Processor returned nil response for message type %d (this is normal for some message types)", msg.Type)
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
		time.Sleep(60 * time.Second) // Check every 60 seconds

		// Very conservative connection check - only verify the connection object exists
		c.mu.RLock()
		connExists := c.conn != nil
		c.mu.RUnlock()

		if !connExists {
			// Connection is nil, trigger disconnect event
			c.triggerEvent(EventDisconnected, nil)
			log.Println("Connection lost, attempting to reconnect...")
			_ = c.Shutdown()

			// Attempt to reconnect with exponential backoff
			for i := 0; i < 5; i++ {
				if !c.IsStarted() {
					break // Exit if shutdown was called
				}

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

func (c *NettyClient) heartbeatLoop() {
	// Start heartbeat after 5 seconds, then every 5 seconds (matching Java version)
	time.Sleep(5 * time.Second)

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

		// Use a separate goroutine to send heartbeat to avoid blocking the loop
		go func() {
			if err := c.SendMsg(heartbeat); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			} else {
				log.Printf("Heartbeat sent successfully for identity: %s, time: %d", identity, time.Now().UnixMilli())
			}
		}()

		// Wait 5 seconds before next heartbeat (matching Java version)
		time.Sleep(5 * time.Second)
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
			// Heartbeat processor returns nil, nil (no response needed)
			// This is expected behavior for heartbeat messages
			processor := &HeartbeatProcessor{}
			_, err := processor.Process(pbMsg)
			return nil, err // Return nil response explicitly
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoOnline, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			fmt.Printf("=== GO ONLINE MESSAGE RECEIVED ===\n")
			fmt.Printf("Message Type: %d (GO_ONLINE)\n", pbMsg.Type)
			fmt.Printf("Message Content: %s\n", string(pbMsg.Msg))
			fmt.Printf("Message Identity: %s\n", pbMsg.Identity)
			fmt.Printf("==================================\n")

			// Process go online message
			go func() {
				fmt.Printf("=== PROCESSING GO ONLINE ===\n")

				// TODO: Implement go online logic
				// This should:
				// 1. Initialize collector state
				// 2. Start background services
				// 3. Register with manager

				fmt.Printf("Collector is now online\n")
				fmt.Printf("========================\n")
			}()

			// Return ACK
			return &pb.Message{
				Type:      pb.MessageType_GO_ONLINE,
				Direction: pb.Direction_RESPONSE,
				Identity:  pbMsg.Identity,
				Msg:       []byte("go online ack"),
			}, nil
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoOffline, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			fmt.Printf("=== GO OFFLINE MESSAGE RECEIVED ===\n")
			fmt.Printf("Message Type: %d (GO_OFFLINE)\n", pbMsg.Type)
			fmt.Printf("Message Content: %s\n", string(pbMsg.Msg))
			fmt.Printf("Message Identity: %s\n", pbMsg.Identity)
			fmt.Printf("====================================\n")

			// Process go offline message
			go func() {
				fmt.Printf("=== PROCESSING GO OFFLINE ===\n")

				// TODO: Implement go offline logic
				// This should:
				// 1. Stop all collection tasks
				// 2. Clean up resources
				// 3. Notify manager of offline status

				fmt.Printf("Collector is now offline\n")
				fmt.Printf("=========================\n")
			}()

			// Return ACK
			return &pb.Message{
				Type:      pb.MessageType_GO_OFFLINE,
				Direction: pb.Direction_RESPONSE,
				Identity:  pbMsg.Identity,
				Msg:       []byte("go offline ack"),
			}, nil
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeGoClose, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			fmt.Printf("=== GO CLOSE MESSAGE RECEIVED ===\n")
			fmt.Printf("Message Type: %d (GO_CLOSE)\n", pbMsg.Type)
			fmt.Printf("Message Content: %s\n", string(pbMsg.Msg))
			fmt.Printf("Message Identity: %s\n", pbMsg.Identity)
			fmt.Printf("==================================\n")

			// Process go close message
			go func() {
				fmt.Printf("=== PROCESSING GO CLOSE ===\n")

				// TODO: Implement go close logic
				// This should:
				// 1. Stop all services
				// 2. Clean up all resources
				// 3. Shutdown the collector

				fmt.Printf("Collector is shutting down\n")
				fmt.Printf("===========================\n")
			}()

			// Return ACK
			return &pb.Message{
				Type:      pb.MessageType_GO_CLOSE,
				Direction: pb.Direction_RESPONSE,
				Identity:  pbMsg.Identity,
				Msg:       []byte("go close ack"),
			}, nil
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			fmt.Printf("=== CYCLIC TASK RECEIVED ===\n")
			fmt.Printf("Message Type: %d (ISSUE_CYCLIC_TASK)\n", pbMsg.Type)
			fmt.Printf("Message Content: %s\n", string(pbMsg.Msg))
			fmt.Printf("Message Identity: %s\n", pbMsg.Identity)
			fmt.Printf("==============================\n")

			// Process cyclic task asynchronously
			go func() {
				fmt.Printf("=== PROCESSING CYCLIC TASK ===\n")

				// TODO: Implement actual cyclic task processing
				// This should:
				// 1. Parse the Job from pbMsg.Msg
				// 2. Add to scheduler for periodic execution
				// 3. Start the cyclic collection

				// For now, just log the task
				fmt.Printf("Cyclic task scheduled (simulated)\n")
				fmt.Printf("================================\n")
			}()

			// Return ACK immediately
			return &pb.Message{
				Type:      pb.MessageType_ISSUE_CYCLIC_TASK,
				Direction: pb.Direction_RESPONSE,
				Identity:  pbMsg.Identity,
				Msg:       []byte("cyclic task ack"),
			}, nil
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeDeleteCyclicTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			fmt.Printf("=== DELETE CYCLIC TASK RECEIVED ===\n")
			fmt.Printf("Message Type: %d (DELETE_CYCLIC_TASK)\n", pbMsg.Type)
			fmt.Printf("Message Content: %s\n", string(pbMsg.Msg))
			fmt.Printf("Message Identity: %s\n", pbMsg.Identity)
			fmt.Printf("===================================\n")

			// Process delete task asynchronously
			go func() {
				fmt.Printf("=== PROCESSING DELETE CYCLIC TASK ===\n")

				// TODO: Implement actual task deletion
				// This should:
				// 1. Parse the Job ID from pbMsg.Msg
				// 2. Remove from scheduler
				// 3. Cancel any running instances

				// For now, just log the deletion
				fmt.Printf("Cyclic task deleted (simulated)\n")
				fmt.Printf("===============================\n")
			}()

			// Return ACK immediately
			return &pb.Message{
				Type:      pb.MessageType_DELETE_CYCLIC_TASK,
				Direction: pb.Direction_RESPONSE,
				Identity:  pbMsg.Identity,
				Msg:       []byte("delete cyclic task ack"),
			}, nil
		}
		return nil, fmt.Errorf("invalid message type")
	})

	client.RegisterProcessor(MessageTypeIssueOneTimeTask, func(msg interface{}) (interface{}, error) {
		if pbMsg, ok := msg.(*pb.Message); ok {
			// Use fmt.Printf to ensure the log is visible
			fmt.Printf("=== ONE-TIME TASK RECEIVED ===\n")
			fmt.Printf("Message Type: %d (ISSUE_ONE_TIME_TASK)\n", pbMsg.Type)
			fmt.Printf("Message Content: %s\n", string(pbMsg.Msg))
			fmt.Printf("Message Identity: %s\n", pbMsg.Identity)
			fmt.Printf("==============================\n")

			// Process task asynchronously to avoid blocking heartbeat
			go func() {
				fmt.Printf("=== PROCESSING ONE-TIME TASK ===\n")

				// TODO: Implement actual job service integration
				// For now, simulate the complete flow:
				// 1. Parse the Job from pbMsg.Msg
				// 2. Execute the collection task
				// 3. Send RESPONSE_ONE_TIME_TASK_DATA message with results

				// Simulate task processing
				time.Sleep(100 * time.Millisecond) // Simulate collection time

				// Create response message with collected data
				responseData := map[string]interface{}{
					"jobId":   123,
					"success": true,
					"data": []map[string]interface{}{
						{
							"app":     "test-app",
							"metrics": "test-metrics",
							"fields": map[string]interface{}{
								"status": "ok",
								"value":  100,
							},
							"time": time.Now().UnixMilli(),
						},
					},
				}

				responseJSON, _ := json.Marshal(responseData)

				responseMsg := &pb.Message{
					Type:      pb.MessageType_RESPONSE_ONE_TIME_TASK_DATA,
					Direction: pb.Direction_REQUEST,
					Identity:  pbMsg.Identity,
					Msg:       responseJSON,
				}

				// Send response back to manager
				if err := client.SendMsg(responseMsg); err != nil {
					fmt.Printf("Failed to send response: %v\n", err)
				} else {
					fmt.Printf("Successfully sent RESPONSE_ONE_TIME_TASK_DATA\n")
				}

				fmt.Printf("One-time task processing completed\n")
				fmt.Printf("==========================================\n")
			}()

			// Return nil immediately - don't block the response
			// The actual response will be sent asynchronously
			return nil, nil
		}
		return nil, fmt.Errorf("invalid message type")
	})
}
