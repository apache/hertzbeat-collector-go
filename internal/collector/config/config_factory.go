package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	loggertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// ConfigDefaults contains all default configuration values
type ConfigDefaults struct {
	Identity        string
	Mode            string
	CollectorName   string
	CollectorIP     string
	CollectorPort   string
	ManagerHost     string
	ManagerPort     string
	ManagerProtocol string
	LogLevel        string
}

// GetDefaultConfig returns the default configuration values
func GetDefaultConfig() *ConfigDefaults {
	return &ConfigDefaults{
		Identity:        "hertzbeat-collector-go",
		Mode:            "public",
		CollectorName:   "hertzbeat-collector-go",
		CollectorIP:     "127.0.0.1",
		CollectorPort:   "8080",
		ManagerHost:     "127.0.0.1",
		ManagerPort:     "1158",
		ManagerProtocol: "netty",
		LogLevel:        "info",
	}
}

// ConfigFactory provides factory methods for creating configurations
type ConfigFactory struct {
	defaults *ConfigDefaults
	logger   logger.Logger
}

// NewConfigFactory creates a new configuration factory
func NewConfigFactory() *ConfigFactory {
	return &ConfigFactory{
		defaults: GetDefaultConfig(),
		logger:   logger.DefaultLogger(os.Stdout, loggertypes.LogLevelInfo).WithName("config-factory"),
	}
}

// CreateDefaultConfig creates a configuration with default values
func (f *ConfigFactory) CreateDefaultConfig() *config.CollectorConfig {
	return &config.CollectorConfig{
		Collector: config.CollectorSection{
			Info: config.CollectorInfo{
				Name: f.defaults.CollectorName,
				IP:   f.defaults.CollectorIP,
				Port: f.defaults.CollectorPort,
			},
			Log: config.CollectorLogConfig{
				Level: f.defaults.LogLevel,
			},
			Manager: config.ManagerConfig{
				Host:     f.defaults.ManagerHost,
				Port:     f.defaults.ManagerPort,
				Protocol: f.defaults.ManagerProtocol,
			},
			Identity: f.defaults.Identity,
			Mode:     f.defaults.Mode,
		},
	}
}

// CreateFromEnv creates configuration from environment variables with fallback to defaults
func (f *ConfigFactory) CreateFromEnv() *config.CollectorConfig {
	cfg := f.CreateDefaultConfig()

	// Override with environment variables if they exist
	if identity := os.Getenv("IDENTITY"); identity != "" {
		cfg.Collector.Identity = identity
	}

	if mode := os.Getenv("MODE"); mode != "" {
		cfg.Collector.Mode = mode
	}

	if name := os.Getenv("COLLECTOR_NAME"); name != "" {
		cfg.Collector.Info.Name = name
	}

	if ip := os.Getenv("COLLECTOR_IP"); ip != "" {
		cfg.Collector.Info.IP = ip
	}

	if port := os.Getenv("COLLECTOR_PORT"); port != "" {
		cfg.Collector.Info.Port = port
	}

	if host := os.Getenv("MANAGER_HOST"); host != "" {
		cfg.Collector.Manager.Host = host
	}

	if port := os.Getenv("MANAGER_PORT"); port != "" {
		cfg.Collector.Manager.Port = port
	}

	if protocol := os.Getenv("MANAGER_PROTOCOL"); protocol != "" {
		cfg.Collector.Manager.Protocol = protocol
	}

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Collector.Log.Level = level
	}

	f.logger.Info("Configuration created from environment variables",
		"identity", cfg.Collector.Identity,
		"mode", cfg.Collector.Mode,
		"manager", fmt.Sprintf("%s:%s", cfg.Collector.Manager.Host, cfg.Collector.Manager.Port))

	return cfg
}

// MergeWithEnv merges file configuration with environment variable overrides
func (f *ConfigFactory) MergeWithEnv(fileCfg *config.CollectorConfig) *config.CollectorConfig {
	if fileCfg == nil {
		return f.CreateFromEnv()
	}

	// Start with file configuration
	cfg := *fileCfg

	// Override with environment variables if they exist
	if identity := os.Getenv("IDENTITY"); identity != "" {
		cfg.Collector.Identity = identity
	}

	if mode := os.Getenv("MODE"); mode != "" {
		cfg.Collector.Mode = mode
	}

	if name := os.Getenv("COLLECTOR_NAME"); name != "" {
		cfg.Collector.Info.Name = name
	}

	if ip := os.Getenv("COLLECTOR_IP"); ip != "" {
		cfg.Collector.Info.IP = ip
	}

	if port := os.Getenv("COLLECTOR_PORT"); port != "" {
		cfg.Collector.Info.Port = port
	}

	if host := os.Getenv("MANAGER_HOST"); host != "" {
		cfg.Collector.Manager.Host = host
	}

	if port := os.Getenv("MANAGER_PORT"); port != "" {
		cfg.Collector.Manager.Port = port
	}

	if protocol := os.Getenv("MANAGER_PROTOCOL"); protocol != "" {
		cfg.Collector.Manager.Protocol = protocol
	}

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Collector.Log.Level = level
	}

	// Fill in defaults for any empty fields
	f.fillDefaults(&cfg)

	f.logger.Info("Configuration merged with environment variables")
	return &cfg
}

// fillDefaults fills in default values for any empty fields
func (f *ConfigFactory) fillDefaults(cfg *config.CollectorConfig) {
	if cfg.Collector.Identity == "" {
		cfg.Collector.Identity = f.defaults.Identity
	}

	if cfg.Collector.Mode == "" {
		cfg.Collector.Mode = f.defaults.Mode
	}

	if cfg.Collector.Info.Name == "" {
		cfg.Collector.Info.Name = f.defaults.CollectorName
	}

	if cfg.Collector.Info.IP == "" {
		cfg.Collector.Info.IP = f.defaults.CollectorIP
	}

	if cfg.Collector.Info.Port == "" {
		cfg.Collector.Info.Port = f.defaults.CollectorPort
	}

	if cfg.Collector.Manager.Host == "" {
		cfg.Collector.Manager.Host = f.defaults.ManagerHost
	}

	if cfg.Collector.Manager.Port == "" {
		cfg.Collector.Manager.Port = f.defaults.ManagerPort
	}

	if cfg.Collector.Manager.Protocol == "" {
		cfg.Collector.Manager.Protocol = f.defaults.ManagerProtocol
	}

	if cfg.Collector.Log.Level == "" {
		cfg.Collector.Log.Level = f.defaults.LogLevel
	}
}

// ValidateConfig validates the configuration
func (f *ConfigFactory) ValidateConfig(cfg *config.CollectorConfig) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate required fields
	if cfg.Collector.Identity == "" {
		return fmt.Errorf("collector identity is required")
	}

	if cfg.Collector.Mode == "" {
		return fmt.Errorf("collector mode is required")
	}

	if cfg.Collector.Manager.Host == "" {
		return fmt.Errorf("manager host is required")
	}

	if cfg.Collector.Manager.Port == "" {
		return fmt.Errorf("manager port is required")
	}

	// Validate mode
	validModes := map[string]bool{
		"public":  true,
		"private": true,
	}
	if !validModes[strings.ToLower(cfg.Collector.Mode)] {
		return fmt.Errorf("invalid mode: %s, must be 'public' or 'private'", cfg.Collector.Mode)
	}

	// Validate protocol
	validProtocols := map[string]bool{
		"netty": true,
		"grpc":  true,
	}
	if cfg.Collector.Manager.Protocol != "" && !validProtocols[strings.ToLower(cfg.Collector.Manager.Protocol)] {
		return fmt.Errorf("invalid protocol: %s, must be 'netty' or 'grpc'", cfg.Collector.Manager.Protocol)
	}

	// Validate port numbers
	if _, err := strconv.Atoi(cfg.Collector.Info.Port); err != nil {
		return fmt.Errorf("invalid collector port: %s", cfg.Collector.Info.Port)
	}

	if _, err := strconv.Atoi(cfg.Collector.Manager.Port); err != nil {
		return fmt.Errorf("invalid manager port: %s", cfg.Collector.Manager.Port)
	}

	f.logger.Info("Configuration validation passed")
	return nil
}

// GetManagerAddress returns the full manager address (host:port)
func (f *ConfigFactory) GetManagerAddress(cfg *config.CollectorConfig) string {
	if cfg == nil || cfg.Collector.Manager.Host == "" || cfg.Collector.Manager.Port == "" {
		return fmt.Sprintf("%s:%s", f.defaults.ManagerHost, f.defaults.ManagerPort)
	}
	return fmt.Sprintf("%s:%s", cfg.Collector.Manager.Host, cfg.Collector.Manager.Port)
}

// IsPublicMode checks if the collector is running in public mode
func (f *ConfigFactory) IsPublicMode(cfg *config.CollectorConfig) bool {
	if cfg == nil {
		return strings.ToLower(f.defaults.Mode) == "public"
	}
	return strings.ToLower(cfg.Collector.Mode) == "public"
}

// IsPrivateMode checks if the collector is running in private mode
func (f *ConfigFactory) IsPrivateMode(cfg *config.CollectorConfig) bool {
	if cfg == nil {
		return strings.ToLower(f.defaults.Mode) == "private"
	}
	return strings.ToLower(cfg.Collector.Mode) == "private"
}

// PrintConfig prints the configuration in a readable format
func (f *ConfigFactory) PrintConfig(cfg *config.CollectorConfig) {
	if cfg == nil {
		f.logger.Error(fmt.Errorf("configuration is nil"), "Configuration is nil")
		return
	}

	f.logger.Info("=== Collector Configuration ===")
	f.logger.Info("Identity", "value", cfg.Collector.Identity)
	f.logger.Info("Mode", "value", cfg.Collector.Mode)
	f.logger.Info("Collector Name", "value", cfg.Collector.Info.Name)
	f.logger.Info("Collector Address", "value", fmt.Sprintf("%s:%s", cfg.Collector.Info.IP, cfg.Collector.Info.Port))
	f.logger.Info("Manager Address", "value", fmt.Sprintf("%s:%s", cfg.Collector.Manager.Host, cfg.Collector.Manager.Port))
	f.logger.Info("Manager Protocol", "value", cfg.Collector.Manager.Protocol)
	f.logger.Info("Log Level", "value", cfg.Collector.Log.Level)
	f.logger.Info("===============================")
}

// === Configuration Entry Points ===

// LoadFromFile loads configuration from file only
func LoadFromFile(cfgPath string) (*config.CollectorConfig, error) {
	loader := New(cfgPath)
	return loader.LoadConfig()
}

// LoadFromEnv loads configuration from environment variables only
func LoadFromEnv() *config.CollectorConfig {
	envLoader := NewEnvConfigLoader()
	return envLoader.LoadFromEnv()
}

// LoadUnified loads configuration with file + env priority (recommended)
func LoadUnified(cfgPath string) (*config.CollectorConfig, error) {
	unifiedLoader := NewUnifiedConfigLoader(cfgPath)
	return unifiedLoader.Load()
}
