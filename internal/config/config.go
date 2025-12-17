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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	cfgtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/config"
	collectortypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/err"
	loggertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

var (
	globalConfig atomic.Pointer[cfgtypes.CollectorConfig]
	configMu     sync.RWMutex
)

// Loader handles file-based configuration loading with hot-reload support
type Loader struct {
	cfgPath string
	logger  logger.Logger
}

// New creates a new configuration loader
func New(cfgPath string) *Loader {

	return &Loader{
		cfgPath: cfgPath,
		logger:  logger.DefaultLogger(os.Stdout, loggertypes.LogLevelInfo).WithName("config-loader"),
	}
}

// LoadConfig loads configuration from file
func (l *Loader) LoadConfig() (*cfgtypes.CollectorConfig, error) {
	if l.cfgPath == "" {
		err := errors.New("config path is required")
		l.logger.Error(err, "config path is empty")
		return nil, err
	}

	cfg, err := l.parseConfigFile(l.cfgPath)
	if err != nil {
		return nil, err
	}

	l.logger.Info("configuration loaded successfully", "path", l.cfgPath)
	return cfg, nil
}

// parseConfigFile parses the YAML config file
func (l *Loader) parseConfigFile(path string) (*cfgtypes.CollectorConfig, error) {
	// Resolve symlinks to handle Kubernetes ConfigMap mounts
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}

	if _, err := os.Stat(resolved); os.IsNotExist(err) {
		l.logger.Error(err, "config file not exist", "path", resolved)
		return nil, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		l.logger.Error(err, "failed to read config file", "path", resolved)
		return nil, err
	}

	var cfg cfgtypes.CollectorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		l.logger.Error(err, "failed to parse config file", "path", resolved)
		return nil, err
	}

	// Apply default values
	// when some fields are missing in config file
	l.applyDefaults(&cfg)

	return &cfg, nil
}

// applyDefaults fills in missing configuration with defaults
func (l *Loader) applyDefaults(cfg *cfgtypes.CollectorConfig) {
	if cfg.Collector.Info.Name == "" {
		cfg.Collector.Info.Name = constants.DefaultHertzBeatCollectorName
	}
	if cfg.Collector.Log.Level == "" {
		cfg.Collector.Log.Level = string(loggertypes.LogLevelInfo)
	}
	if cfg.Collector.Manager.Protocol == "" {
		cfg.Collector.Manager.Protocol = "http"
	}
}

// ValidateConfig validates the configuration
func (l *Loader) ValidateConfig(cfg *cfgtypes.CollectorConfig) error {
	if cfg == nil {
		err := errors.New("config is nil")
		l.logger.Error(collectortypes.CollectorConfigIsNil, "config validation failed")
		return err
	}

	if cfg.Collector.Info.IP == "" {
		err := errors.New("collector ip is empty")
		l.logger.Error(collectortypes.CollectorIPIsNil, "config validation failed")
		return err
	}

	if cfg.Collector.Info.Port == "" {
		err := errors.New("collector port is empty")
		l.logger.Error(collectortypes.CollectorPortIsNil, "config validation failed")
		return err
	}

	if cfg.Collector.Info.Name == "" {
		cfg.Collector.Info.Name = constants.DefaultHertzBeatCollectorName
		l.logger.Sugar().Debug("collector name is empty, using default")
	}

	return nil
}

// GetManagerAddress returns the full manager address
func (l *Loader) GetManagerAddress(cfg *cfgtypes.CollectorConfig) string {
	if cfg == nil || cfg.Collector.Manager.Host == "" {
		return ""
	}
	protocol := cfg.Collector.Manager.Protocol
	if protocol == "" {
		protocol = "http"
	}
	return fmt.Sprintf("%s://%s:%s", protocol, cfg.Collector.Manager.Host, cfg.Collector.Manager.Port)
}

// PrintConfig prints the configuration
func (l *Loader) PrintConfig(cfg *cfgtypes.CollectorConfig) {
	if cfg == nil {
		l.logger.Info("config is nil")
		return
	}
	l.logger.Info("current configuration",
		"name", cfg.Collector.Info.Name,
		"ip", cfg.Collector.Info.IP,
		"port", cfg.Collector.Info.Port,
		"log_level", cfg.Collector.Log.Level,
		"manager", l.GetManagerAddress(cfg),
	)
}

// GetGlobalConfig returns the current global configuration
func GetGlobalConfig() *cfgtypes.CollectorConfig {
	return globalConfig.Load()
}

// SetGlobalConfig sets the global configuration
func SetGlobalConfig(cfg *cfgtypes.CollectorConfig) {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig.Store(cfg)
}

// WatchConfigAndReload watches the config file and reloads on changes
// This function implements hot-reload for both configuration and logging
func (l *Loader) WatchConfigAndReload(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		l.logger.Error(err, "failed to create config watcher")
		return err
	}
	defer watcher.Close()

	// Watch both the file and its directory to handle symlink swaps (Kubernetes ConfigMap)
	cfgFile := l.cfgPath
	cfgDir := filepath.Dir(cfgFile)

	if err := watcher.Add(cfgDir); err != nil {
		l.logger.Error(err, "failed to watch config directory", "dir", cfgDir)
		return err
	}

	// Try to watch the file directly (best-effort)
	_ = watcher.Add(cfgFile)

	l.logger.Info("config file watcher started", "path", cfgFile)

	// Debounce events
	var (
		pending bool
		last    time.Time
	)

	reload := func() {
		l.logger.Info("config file changed, reloading...")

		// Parse new configuration
		newCfg, err := l.parseConfigFile(cfgFile)
		if err != nil {
			l.logger.Error(err, "failed to reload config")
			return
		}

		// Validate new configuration
		if err := l.ValidateConfig(newCfg); err != nil {
			l.logger.Error(err, "invalid config after reload")
			return
		}

		// Update global configuration
		SetGlobalConfig(newCfg)

		// Hot-reload logging configuration
		if err := l.reloadLogging(newCfg); err != nil {
			l.logger.Error(err, "failed to reload logging")
		}

		l.logger.Info("configuration reloaded successfully")
	}

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("config watcher stopped")
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Handle Write, Create, Remove, Rename, Chmod events
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) != 0 {
				// Check if the event pertains to the config file or directory
				if filepath.Base(event.Name) == filepath.Base(cfgFile) || filepath.Dir(event.Name) == cfgDir {
					// Debounce: if not pending or enough time has passed
					if !pending || time.Since(last) > 250*time.Millisecond {
						pending = true
						last = time.Now()
						// Slight delay to let file settle
						go func() {
							time.Sleep(300 * time.Millisecond)
							reload()
							pending = false
						}()
					}
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			l.logger.Error(err, "config watcher error")
		}
	}
}

// reloadLogging reloads the logging configuration
// when config file changes, it helps dynamically
// adjust the log level for dynamic debugging
func (l *Loader) reloadLogging(cfg *cfgtypes.CollectorConfig) error {

	if cfg == nil {
		return errors.New("config is nil")
	}

	// Parse log level from config
	level := loggertypes.LogLevel(cfg.Collector.Log.Level)
	if level == "" {
		level = loggertypes.LogLevelInfo
	}

	// Create new logger with updated level
	newLogger := logger.DefaultLogger(os.Stdout, level).WithName("config-loader")

	// Update loader's logger
	l.logger = newLogger

	l.logger.Info("logging configuration reloaded", "level", level)
	return nil
}
