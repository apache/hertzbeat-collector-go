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

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	loggerTypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// EnvConfigLoader handles environment variable configuration
// It uses the common ConfigFactory for consistency
type EnvConfigLoader struct {
	factory *ConfigFactory
	logger  logger.Logger
}

// NewEnvConfigLoader creates a new environment variable configuration loader
func NewEnvConfigLoader() *EnvConfigLoader {
	return &EnvConfigLoader{
		factory: NewConfigFactory(),
		logger:  logger.DefaultLogger(os.Stdout, loggerTypes.LogLevelInfo).WithName("env-config-loader"),
	}
}

// LoadFromEnv loads configuration from environment variables with defaults
func (l *EnvConfigLoader) LoadFromEnv() *config.CollectorConfig {
	cfg := l.factory.CreateFromEnv()
	l.logger.Info("Configuration loaded from environment variables")
	return cfg
}

// LoadWithDefaults loads configuration with environment variables overriding provided defaults
func (l *EnvConfigLoader) LoadWithDefaults(defaultCfg *config.CollectorConfig) *config.CollectorConfig {
	if defaultCfg == nil {
		return l.LoadFromEnv()
	}

	cfg := l.factory.MergeWithEnv(defaultCfg)
	l.logger.Info("Configuration loaded with environment variable overrides")
	return cfg
}

// ValidateEnvConfig validates the environment configuration
func (l *EnvConfigLoader) ValidateEnvConfig(cfg *config.CollectorConfig) error {
	return l.factory.ValidateConfig(cfg)
}

// PrintEnvConfig prints the current environment configuration
func (l *EnvConfigLoader) PrintEnvConfig(cfg *config.CollectorConfig) {
	l.factory.PrintConfig(cfg)
}

// GetManagerAddress returns the full manager address (host:port)
func (l *EnvConfigLoader) GetManagerAddress(cfg *config.CollectorConfig) string {
	return l.factory.GetManagerAddress(cfg)
}

// IsPublicMode checks if the collector is running in public mode
func (l *EnvConfigLoader) IsPublicMode(cfg *config.CollectorConfig) bool {
	return l.factory.IsPublicMode(cfg)
}

// IsPrivateMode checks if the collector is running in private mode
func (l *EnvConfigLoader) IsPrivateMode(cfg *config.CollectorConfig) bool {
	return l.factory.IsPrivateMode(cfg)
}
