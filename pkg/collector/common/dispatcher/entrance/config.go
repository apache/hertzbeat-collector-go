package entrance

import (
	"time"
)

// ClientConfig holds the configuration for the network client.
type ClientConfig struct {
	// Manager server host
	ManagerHost string `yaml:"manager_host" json:"manager_host"`
	// Manager server port
	ManagerPort int `yaml:"manager_port" json:"manager_port"`
	// Connection timeout
	ConnectTimeout time.Duration `yaml:"connect_timeout" json:"connect_timeout"`
	// Read timeout for responses
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout"`
	// Write timeout for requests
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	// Heartbeat interval
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" json:"heartbeat_interval"`
	// Reconnect interval when connection fails
	ReconnectInterval time.Duration `yaml:"reconnect_interval" json:"reconnect_interval"`
	// Maximum reconnect attempts (0 = infinite)
	MaxReconnectAttempts int `yaml:"max_reconnect_attempts" json:"max_reconnect_attempts"`
	// Enable compression (GZIP)
	EnableCompression bool `yaml:"enable_compression" json:"enable_compression"`
	// Buffer sizes
	ReadBufferSize  int `yaml:"read_buffer_size" json:"read_buffer_size"`
	WriteBufferSize int `yaml:"write_buffer_size" json:"write_buffer_size"`
}

// DefaultClientConfig returns a default configuration.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ManagerHost:          "127.0.0.1",
		ManagerPort:          8080,
		ConnectTimeout:       30 * time.Second,
		ReadTimeout:          60 * time.Second,
		WriteTimeout:         30 * time.Second,
		HeartbeatInterval:    30 * time.Second,
		ReconnectInterval:    10 * time.Second,
		MaxReconnectAttempts: 0, // Infinite retries
		EnableCompression:    true,
		ReadBufferSize:       4096,
		WriteBufferSize:      4096,
	}
}

// CollectorConfig holds the collector identity and mode configuration.
type CollectorConfig struct {
	// Collector unique identity
	Identity string `yaml:"identity" json:"identity"`
	// Collector mode: "public" for public cluster, "private" for private cloud-edge
	Mode string `yaml:"mode" json:"mode"`
	// Collector version
	Version string `yaml:"version" json:"version"`
	// Collector IP address
	IP string `yaml:"ip" json:"ip"`
}

// DefaultCollectorConfig returns a default collector configuration.
func DefaultCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		Identity: "default-collector",
		Mode:     "public",
		Version:  "1.7.3",
		IP:       "",
	}
}
