package entrance

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"
	"hertzbeat.apache.org/hertzbeat-collector-go/api"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

// MessageProcessor defines the interface for processing received messages.
type MessageProcessor interface {
	Process(ctx context.Context, message *api.Message) error
	GetMessageType() api.MessageType
}

// NetworkEventListener defines the interface for network events.
type NetworkEventListener interface {
	OnConnected()
	OnDisconnected()
	OnError(err error)
}

// NetworkClient manages the TCP connection to the HertzBeat Manager.
type NetworkClient struct {
	config          *ClientConfig
	collectorConfig *CollectorConfig
	logger          logger.Logger
	eventListener   NetworkEventListener

	conn        net.Conn
	connMutex   sync.RWMutex
	isConnected atomic.Bool
	isShutdown  atomic.Bool

	processors      map[api.MessageType]MessageProcessor
	processorsMutex sync.RWMutex

	heartbeatTicker *time.Ticker
	reconnectTicker *time.Ticker
	stopChan        chan struct{}
	wg              sync.WaitGroup

	// For sending messages
	sendChan   chan *api.Message
	sendBuffer chan *api.Message
}

// NewNetworkClient creates a new NetworkClient instance.
func NewNetworkClient(
	config *ClientConfig,
	collectorConfig *CollectorConfig,
	logger logger.Logger,
	eventListener NetworkEventListener,
) *NetworkClient {
	if config == nil {
		config = DefaultClientConfig()
	}
	if collectorConfig == nil {
		collectorConfig = DefaultCollectorConfig()
	}

	return &NetworkClient{
		config:          config,
		collectorConfig: collectorConfig,
		logger:          logger.WithName("network-client"),
		eventListener:   eventListener,
		processors:      make(map[api.MessageType]MessageProcessor),
		stopChan:        make(chan struct{}),
		sendChan:        make(chan *api.Message, 100),  // Buffered channel for outgoing messages
		sendBuffer:      make(chan *api.Message, 1000), // Larger buffer for high throughput
	}
}

// RegisterProcessor registers a message processor for a specific message type.
func (nc *NetworkClient) RegisterProcessor(msgType api.MessageType, processor MessageProcessor) {
	nc.processorsMutex.Lock()
	defer nc.processorsMutex.Unlock()
	nc.processors[msgType] = processor
	nc.logger.Info("registered message processor", "messageType", msgType)
}

// Start starts the network client and attempts to connect to the manager.
func (nc *NetworkClient) Start() error {
	if nc.isShutdown.Load() {
		return fmt.Errorf("client is already shutdown")
	}

	nc.logger.Info("starting network client",
		"managerHost", nc.config.ManagerHost,
		"managerPort", nc.config.ManagerPort)

	// Start background routines
	nc.wg.Add(4)
	go nc.connectionLoop()
	go nc.heartbeatLoop()
	go nc.sendLoop()
	go nc.receiveLoop()

	return nil
}

// Stop stops the network client and closes connections.
func (nc *NetworkClient) Stop() error {
	if !nc.isShutdown.CompareAndSwap(false, true) {
		return fmt.Errorf("client is already stopped")
	}

	nc.logger.Info("stopping network client")

	// Signal all goroutines to stop
	close(nc.stopChan)

	// Close connection
	nc.closeConnection()

	// Wait for all goroutines to finish
	nc.wg.Wait()

	nc.logger.Info("network client stopped successfully")
	return nil
}

// SendMessage sends a message to the manager.
func (nc *NetworkClient) SendMessage(message *api.Message) error {
	if nc.isShutdown.Load() {
		return fmt.Errorf("client is shutdown")
	}

	select {
	case nc.sendChan <- message:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout: message queue is full")
	}
}

// IsConnected returns true if the client is connected to the manager.
func (nc *NetworkClient) IsConnected() bool {
	return nc.isConnected.Load()
}

// connectionLoop manages the connection to the manager with automatic reconnection.
func (nc *NetworkClient) connectionLoop() {
	defer nc.wg.Done()

	attempt := 0
	for {
		if nc.isShutdown.Load() {
			return
		}

		if !nc.isConnected.Load() {
			if err := nc.connect(); err != nil {
				attempt++
				nc.logger.Error(err, "failed to connect to manager",
					"attempt", attempt,
					"retryAfter", nc.config.ReconnectInterval)

				if nc.config.MaxReconnectAttempts > 0 && attempt >= nc.config.MaxReconnectAttempts {
					nc.logger.Error(fmt.Errorf("max reconnect attempts reached"),
						"stopping connection attempts")
					return
				}

				// Wait before retry
				select {
				case <-time.After(nc.config.ReconnectInterval):
					continue
				case <-nc.stopChan:
					return
				}
			} else {
				attempt = 0 // Reset attempt counter on successful connection
				nc.logger.Info("successfully connected to manager")
			}
		}

		// Wait for disconnection or shutdown
		select {
		case <-nc.stopChan:
			return
		case <-time.After(1 * time.Second):
			// Check connection status periodically
		}
	}
}

// connect establishes a TCP connection to the manager.
func (nc *NetworkClient) connect() error {
	nc.closeConnection()

	address := fmt.Sprintf("%s:%d", nc.config.ManagerHost, nc.config.ManagerPort)
	conn, err := net.DialTimeout("tcp", address, nc.config.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Set connection options
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetNoDelay(true)
	}

	nc.connMutex.Lock()
	nc.conn = conn
	nc.connMutex.Unlock()

	nc.isConnected.Store(true)

	if nc.eventListener != nil {
		nc.eventListener.OnConnected()
	}

	// Send GO_ONLINE message
	onlineMsg := &api.Message{
		Identity:  nc.collectorConfig.Identity,
		Direction: api.Direction_REQUEST,
		Type:      api.MessageType_GO_ONLINE,
		Msg: []byte(fmt.Sprintf(`{"identity":"%s","mode":"%s","version":"%s","ip":"%s"}`,
			nc.collectorConfig.Identity, nc.collectorConfig.Mode, nc.collectorConfig.Version, nc.collectorConfig.IP)),
	}
	return nc.writeMessage(onlineMsg)
}

// closeConnection closes the current connection.
func (nc *NetworkClient) closeConnection() {
	nc.connMutex.Lock()
	defer nc.connMutex.Unlock()

	if nc.conn != nil {
		nc.conn.Close()
		nc.conn = nil
	}

	if nc.isConnected.CompareAndSwap(true, false) && nc.eventListener != nil {
		nc.eventListener.OnDisconnected()
	}
}

// heartbeatLoop sends periodic heartbeat messages to keep the connection alive.
func (nc *NetworkClient) heartbeatLoop() {
	defer nc.wg.Done()

	nc.heartbeatTicker = time.NewTicker(nc.config.HeartbeatInterval)
	defer nc.heartbeatTicker.Stop()

	for {
		select {
		case <-nc.heartbeatTicker.C:
			if nc.isConnected.Load() {
				heartbeatMsg := &api.Message{
					Identity:  nc.collectorConfig.Identity,
					Direction: api.Direction_REQUEST,
					Type:      api.MessageType_HEARTBEAT,
					Msg:       []byte("{}"),
				}
				if err := nc.writeMessage(heartbeatMsg); err != nil {
					nc.logger.Error(err, "failed to send heartbeat")
					nc.closeConnection()
				}
			}
		case <-nc.stopChan:
			return
		}
	}
}

// sendLoop handles outgoing messages.
func (nc *NetworkClient) sendLoop() {
	defer nc.wg.Done()

	for {
		select {
		case message := <-nc.sendChan:
			if err := nc.writeMessage(message); err != nil {
				nc.logger.Error(err, "failed to send message", "messageType", message.Type)
				if nc.eventListener != nil {
					nc.eventListener.OnError(err)
				}
			}
		case <-nc.stopChan:
			return
		}
	}
}

// receiveLoop handles incoming messages.
func (nc *NetworkClient) receiveLoop() {
	defer nc.wg.Done()

	for {
		if nc.isShutdown.Load() {
			return
		}

		if !nc.isConnected.Load() {
			select {
			case <-time.After(1 * time.Second):
				continue
			case <-nc.stopChan:
				return
			}
		}

		message, err := nc.readMessage()
		if err != nil {
			if !nc.isShutdown.Load() {
				nc.logger.Error(err, "failed to read message")
				nc.closeConnection()
				if nc.eventListener != nil {
					nc.eventListener.OnError(err)
				}
			}
			continue
		}

		// Process the message
		go nc.processMessage(message)
	}
}

// writeMessage writes a message to the connection.
func (nc *NetworkClient) writeMessage(message *api.Message) error {
	nc.connMutex.RLock()
	conn := nc.conn
	nc.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("connection is not established")
	}

	// Serialize the message
	data, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Apply compression if enabled
	if nc.config.EnableCompression {
		data, err = nc.compressData(data)
		if err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
	}

	// Write message length (4 bytes) + message data
	length := uint32(len(data))
	lengthBytes := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	// Set write timeout
	conn.SetWriteDeadline(time.Now().Add(nc.config.WriteTimeout))

	if _, err := conn.Write(lengthBytes); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// readMessage reads a message from the connection.
func (nc *NetworkClient) readMessage() (*api.Message, error) {
	nc.connMutex.RLock()
	conn := nc.conn
	nc.connMutex.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("connection is not established")
	}

	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(nc.config.ReadTimeout))

	// Read message length (4 bytes)
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := uint32(lengthBytes[0])<<24 | uint32(lengthBytes[1])<<16 |
		uint32(lengthBytes[2])<<8 | uint32(lengthBytes[3])

	if length > 1024*1024*10 { // 10MB limit
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Decompress if compression is enabled
	if nc.config.EnableCompression {
		var err error
		data, err = nc.decompressData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress data: %w", err)
		}
	}

	// Deserialize the message
	message := &api.Message{}
	if err := proto.Unmarshal(data, message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return message, nil
}

// processMessage processes a received message using registered processors.
func (nc *NetworkClient) processMessage(message *api.Message) {
	nc.processorsMutex.RLock()
	processor, exists := nc.processors[message.Type]
	nc.processorsMutex.RUnlock()

	if !exists {
		nc.logger.Info("no processor found for message type", "messageType", message.Type)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := processor.Process(ctx, message); err != nil {
		nc.logger.Error(err, "failed to process message", "messageType", message.Type)
	}
}

// compressData compresses data using GZIP.
func (nc *NetworkClient) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompressData decompresses GZIP data.
func (nc *NetworkClient) decompressData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
