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
	"os"

	"gopkg.in/yaml.v3"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	DefaultHertzBeatCollectorName = "hertzbeat-collector"
)

type HookFunc func(c context.Context, server *server.CollectorServer) error

type Loader struct {
	cfgPath string
	logger  logger.Logger
	cancel  context.CancelFunc
	server  *server.CollectorServer

	hook HookFunc

	// todo file watcher
	// watcher *fsnotify.Watcher
}

func New(cfgPath string, server *server.CollectorServer, f HookFunc) *Loader {

	return &Loader{
		cfgPath: cfgPath,
		server:  server,
		logger:  server.Logger.WithName("collector-config-loader"),
		hook:    f,
	}
}

func (ld *Loader) LoadConfig() (*config.CollectorConfig, error) {

	ld.runHook()

	if ld.cfgPath == "" {
		ld.logger.Info("collector-config-loader: path is empty")
		return nil, errors.New("collector-config-loader: path is empty")
	}

	if _, err := os.Stat(ld.cfgPath); os.IsNotExist(err) {
		ld.logger.Error(err, "collector-config-loader: file not exist", "path", ld.cfgPath)
		return nil, err
	}

	file, err := os.Open(ld.cfgPath)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			ld.logger.Error(err, "close config file failed")
		}
	}(file)

	var cfg config.CollectorConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		ld.logger.Error(err, "decode config file failed")
		return nil, err
	}

	return &cfg, nil
}

func (ld *Loader) ValidateConfig(cfg *config.CollectorConfig) error {

	if cfg == nil {
		ld.logger.Sugar().Debug("collector-config-loader is nil")
		return errors.New("collector-config-loader is nil")
	}

	// other check
	if cfg.Collector.Info.IP == "" {
		ld.logger.Sugar().Debug("collector-config-loader ip is empty")
		return errors.New("collector-config-loader ip is empty")
	}

	if cfg.Collector.Info.Port == "" {
		ld.logger.Sugar().Debug("collector-config-loader port is empty")
		return errors.New("collector-config-loader port is empty")
	}

	if cfg.Collector.Info.Name == "" {
		ld.logger.Sugar().Debug("collector-config-loader: name is empty")
		cfg.Collector.Info.Name = DefaultHertzBeatCollectorName
	}

	return nil
}

func (r *Loader) runHook() {

	if r.hook == nil {
		return
	}

	r.logger.Info("running hook")
	c, cancel := context.WithCancel(context.TODO())
	r.cancel = cancel
	go func(ctx context.Context) {
		if err := r.hook(ctx, r.server); err != nil {
			r.logger.Error(err, "hook error")
		}
	}(c)
}
