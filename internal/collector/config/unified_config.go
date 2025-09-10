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

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	loggerTypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
)

// UnifiedConfigLoader provides a unified configuration loading interface
// that loads from file first, then overrides with environment variables
type UnifiedConfigLoader struct {
	fileLoader *Loader
	envLoader  *EnvConfigLoader
	logger     logger.Logger
}

// NewUnifiedConfigLoader creates a new unified configuration loader
func NewUnifiedConfigLoader(cfgPath string) *UnifiedConfigLoader {
	return &UnifiedConfigLoader{
		fileLoader: New(cfgPath),
		envLoader:  NewEnvConfigLoader(),
		logger:     logger.DefaultLogger(os.Stdout, loggerTypes.LogLevelInfo).WithName("unified-config-loader"),
	}
}

// Load loads configuration from file and environment variables
// Environment variables take precedence over file configuration
func (l *UnifiedConfigLoader) Load() (*config.CollectorConfig, error) {
	// Start with environment variables as base
	cfg := l.envLoader.LoadFromEnv()
	
	// Try to load from file and merge
	if l.fileLoader.cfgPath != "" {
		if fileCfg, err := l.fileLoader.LoadConfig(); err == nil {
			// Merge file config with env config (env takes precedence)
			l.mergeConfig(cfg, fileCfg)
			l.logger.Info("Loaded configuration from file and environment variables", "file", l.fileLoader.cfgPath)
		} else {
			l.logger.Error(err, "Failed to load configuration from file, using environment variables only")
		}
	} else {
		l.logger.Info("No configuration file specified, using environment variables only")
	}
	
	// Validate the final configuration
	if err := l.fileLoader.ValidateConfig(cfg); err != nil {
		l.logger.Error(err, "Configuration validation failed")
		return nil, err
	}
	
	return cfg, nil
}

// mergeConfig merges file configuration into environment configuration
// Environment variables take precedence over file values
func (l *UnifiedConfigLoader) mergeConfig(envCfg, fileCfg *config.CollectorConfig) {
	if fileCfg.Collector.Identity != "" && envCfg.Collector.Identity == "hertzbeat-collector-go" {
		envCfg.Collector.Identity = fileCfg.Collector.Identity
	}
	
	if fileCfg.Collector.Mode != "" && envCfg.Collector.Mode == "public" {
		envCfg.Collector.Mode = fileCfg.Collector.Mode
	}
	
	if fileCfg.Collector.Manager.Host != "" && envCfg.Collector.Manager.Host == "127.0.0.1" {
		envCfg.Collector.Manager.Host = fileCfg.Collector.Manager.Host
	}
	
	if fileCfg.Collector.Manager.Port != "" && envCfg.Collector.Manager.Port == "1158" {
		envCfg.Collector.Manager.Port = fileCfg.Collector.Manager.Port
	}
	
	if fileCfg.Collector.Manager.Protocol != "" && envCfg.Collector.Manager.Protocol == "netty" {
		envCfg.Collector.Manager.Protocol = fileCfg.Collector.Manager.Protocol
	}
	
	if fileCfg.Collector.Info.Name != "" && envCfg.Collector.Info.Name == "hertzbeat-collector" {
		envCfg.Collector.Info.Name = fileCfg.Collector.Info.Name
	}
	
	if fileCfg.Collector.Info.IP != "" && envCfg.Collector.Info.IP == "0.0.0.0" {
		envCfg.Collector.Info.IP = fileCfg.Collector.Info.IP
	}
	
	if fileCfg.Collector.Info.Port != "" && envCfg.Collector.Info.Port == "8080" {
		envCfg.Collector.Info.Port = fileCfg.Collector.Info.Port
	}
}