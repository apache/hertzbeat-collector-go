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

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	loggerTypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
)

// EnvConfigLoader handles environment variable configuration
type EnvConfigLoader struct {
	logger logger.Logger
}

// NewEnvConfigLoader creates a new environment variable configuration loader
func NewEnvConfigLoader() *EnvConfigLoader {
	return &EnvConfigLoader{
		logger: logger.DefaultLogger(os.Stdout, loggerTypes.LogLevelInfo).WithName("env-config-loader"),
	}
}

// LoadFromEnv loads configuration from environment variables
func (l *EnvConfigLoader) LoadFromEnv() *config.CollectorConfig {
	cfg := &config.CollectorConfig{}
	
	// Load collector identity
	cfg.Collector.Identity = l.getEnvWithDefault("IDENTITY", "hertzbeat-collector-go")
	
	// Load collector mode
	cfg.Collector.Mode = l.getEnvWithDefault("MODE", "public")
	
	// Load manager configuration
	cfg.Collector.Manager.Host = l.getEnvWithDefault("MANAGER_HOST", "127.0.0.1")
	cfg.Collector.Manager.Port = l.getEnvWithDefault("MANAGER_PORT", "1158")
	cfg.Collector.Manager.Protocol = l.getEnvWithDefault("MANAGER_PROTOCOL", "netty")
	
	// Load collector info from environment or use defaults
	cfg.Collector.Info.Name = l.getEnvWithDefault("COLLECTOR_NAME", "hertzbeat-collector-go")
	cfg.Collector.Info.IP = l.getEnvWithDefault("COLLECTOR_IP", "127.0.0.1")
	cfg.Collector.Info.Port = l.getEnvWithDefault("COLLECTOR_PORT", "8080")
	
	// Load log configuration
	cfg.Collector.Log.Level = l.getEnvWithDefault("LOG_LEVEL", "info")
	
	l.logger.Info("Loaded configuration from environment variables", 
		"identity", cfg.Collector.Identity,
		"mode", cfg.Collector.Mode,
		"manager_host", cfg.Collector.Manager.Host,
		"manager_port", cfg.Collector.Manager.Port,
		"manager_protocol", cfg.Collector.Manager.Protocol)
	
	return cfg
}

// LoadWithDefaults loads configuration with environment variables overriding defaults
func (l *EnvConfigLoader) LoadWithDefaults(defaultCfg *config.CollectorConfig) *config.CollectorConfig {
	if defaultCfg == nil {
		return l.LoadFromEnv()
	}
	
	cfg := *defaultCfg
	
	// Override with environment variables
	if identity := os.Getenv("IDENTITY"); identity != "" {
		cfg.Collector.Identity = identity
	}
	
	if mode := os.Getenv("MODE"); mode != "" {
		cfg.Collector.Mode = mode
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
	
	if name := os.Getenv("COLLECTOR_NAME"); name != "" {
		cfg.Collector.Info.Name = name
	}
	
	if ip := os.Getenv("COLLECTOR_IP"); ip != "" {
		cfg.Collector.Info.IP = ip
	}
	
	if port := os.Getenv("COLLECTOR_PORT"); port != "" {
		cfg.Collector.Info.Port = port
	}
	
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Collector.Log.Level = level
	}
	
	l.logger.Info("Configuration loaded with environment variable overrides")
	
	return &cfg
}

// getEnvWithDefault gets environment variable with default value
func (l *EnvConfigLoader) getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets boolean environment variable with default value
func (l *EnvConfigLoader) getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// getEnvInt gets integer environment variable with default value
func (l *EnvConfigLoader) getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

// ValidateEnvConfig validates the environment configuration
func (l *EnvConfigLoader) ValidateEnvConfig(cfg *config.CollectorConfig) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	
	// Validate required fields
	if cfg.Collector.Identity == "" {
		return fmt.Errorf("IDENTITY is required")
	}
	
	if cfg.Collector.Mode == "" {
		return fmt.Errorf("MODE is required")
	}
	
	if cfg.Collector.Manager.Host == "" {
		return fmt.Errorf("MANAGER_HOST is required")
	}
	
	if cfg.Collector.Manager.Port == "" {
		return fmt.Errorf("MANAGER_PORT is required")
	}
	
	// Validate mode
	validModes := map[string]bool{
		"public":  true,
		"private": true,
	}
	if !validModes[cfg.Collector.Mode] {
		return fmt.Errorf("invalid MODE: %s, must be 'public' or 'private'", cfg.Collector.Mode)
	}
	
	// Validate protocol
	validProtocols := map[string]bool{
		"netty": true,
		"grpc":  true,
	}
	if cfg.Collector.Manager.Protocol != "" && !validProtocols[cfg.Collector.Manager.Protocol] {
		return fmt.Errorf("invalid MANAGER_PROTOCOL: %s, must be 'netty' or 'grpc'", cfg.Collector.Manager.Protocol)
	}
	
	l.logger.Info("Environment configuration validation passed")
	return nil
}

// PrintEnvConfig prints the current environment configuration
func (l *EnvConfigLoader) PrintEnvConfig(cfg *config.CollectorConfig) {
	if cfg == nil {
		l.logger.Error(fmt.Errorf("configuration is nil"), "Configuration is nil")
		return
	}
	
	l.logger.Info("=== Environment Configuration ===")
	l.logger.Info("Collector Identity", "identity", cfg.Collector.Identity)
	l.logger.Info("Collector Mode", "mode", cfg.Collector.Mode)
	l.logger.Info("Collector Name", "name", cfg.Collector.Info.Name)
	l.logger.Info("Collector IP", "ip", cfg.Collector.Info.IP)
	l.logger.Info("Collector Port", "port", cfg.Collector.Info.Port)
	l.logger.Info("Manager Host", "host", cfg.Collector.Manager.Host)
	l.logger.Info("Manager Port", "port", cfg.Collector.Manager.Port)
	l.logger.Info("Manager Protocol", "protocol", cfg.Collector.Manager.Protocol)
	l.logger.Info("Log Level", "level", cfg.Collector.Log.Level)
	l.logger.Info("===================================")
}

// GetManagerAddress returns the full manager address (host:port)
func (l *EnvConfigLoader) GetManagerAddress(cfg *config.CollectorConfig) string {
	if cfg == nil || cfg.Collector.Manager.Host == "" || cfg.Collector.Manager.Port == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", cfg.Collector.Manager.Host, cfg.Collector.Manager.Port)
}

// IsPublicMode checks if the collector is running in public mode
func (l *EnvConfigLoader) IsPublicMode(cfg *config.CollectorConfig) bool {
	return cfg != nil && strings.ToLower(cfg.Collector.Mode) == "public"
}

// IsPrivateMode checks if the collector is running in private mode
func (l *EnvConfigLoader) IsPrivateMode(cfg *config.CollectorConfig) bool {
	return cfg != nil && strings.ToLower(cfg.Collector.Mode) == "private"
}
