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
	"os"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/types/config"
	loggertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// UnifiedConfigLoader provides a unified configuration loading interface
// that loads from file first, then overrides with environment variables
// It uses the common ConfigFactory for consistency
type UnifiedConfigLoader struct {
	fileLoader *Loader
	envLoader  *EnvConfigLoader
	factory    *ConfigFactory
	logger     logger.Logger
}

// NewUnifiedConfigLoader creates a new unified configuration loader
func NewUnifiedConfigLoader(cfgPath string) *UnifiedConfigLoader {
	return &UnifiedConfigLoader{
		fileLoader: New(cfgPath),
		envLoader:  NewEnvConfigLoader(),
		factory:    NewConfigFactory(),
		logger:     logger.DefaultLogger(os.Stdout, loggertypes.LogLevelInfo).WithName("unified-config-loader"),
	}
}

// Load loads configuration from file and environment variables
// Environment variables take precedence over file configuration
func (l *UnifiedConfigLoader) Load() (*config.CollectorConfig, error) {
	var cfg *config.CollectorConfig
	var err error

	// Try to load from file first
	if l.fileLoader.cfgPath != "" {
		if cfg, err = l.fileLoader.LoadConfig(); err != nil {
			l.logger.Error(err, "Failed to load configuration from file, falling back to environment variables only")
			cfg = nil
		} else {
			l.logger.Info("Configuration loaded from file", "file", l.fileLoader.cfgPath)
		}
	}

	// If file loading failed or no file specified, start with defaults
	if cfg == nil {
		cfg = l.factory.CreateDefaultConfig()
		l.logger.Info("Using default configuration")
	}

	// Merge with environment variables (env takes precedence)
	finalCfg := l.factory.MergeWithEnv(cfg)

	// Validate the final configuration
	if err := l.factory.ValidateConfig(finalCfg); err != nil {
		l.logger.Error(err, "Configuration validation failed")
		return nil, err
	}

	l.logger.Info("Unified configuration loaded successfully")
	return finalCfg, nil
}

// LoadEnvOnly loads configuration from environment variables only
func (l *UnifiedConfigLoader) LoadEnvOnly() *config.CollectorConfig {
	cfg := l.envLoader.LoadFromEnv()
	l.logger.Info("Configuration loaded from environment variables only")
	return cfg
}

// LoadFileOnly loads configuration from file only (without env overrides)
func (l *UnifiedConfigLoader) LoadFileOnly() (*config.CollectorConfig, error) {
	if l.fileLoader.cfgPath == "" {
		cfg := l.factory.CreateDefaultConfig()
		l.logger.Info("No file specified, using default configuration")
		return cfg, nil
	}

	return l.fileLoader.LoadConfig()
}

// ValidateConfig validates the configuration using the factory
func (l *UnifiedConfigLoader) ValidateConfig(cfg *config.CollectorConfig) error {
	return l.factory.ValidateConfig(cfg)
}

// PrintConfig prints the configuration using the factory
func (l *UnifiedConfigLoader) PrintConfig(cfg *config.CollectorConfig) {
	l.factory.PrintConfig(cfg)
}

// GetManagerAddress returns the manager address using the factory
func (l *UnifiedConfigLoader) GetManagerAddress(cfg *config.CollectorConfig) string {
	return l.factory.GetManagerAddress(cfg)
}
