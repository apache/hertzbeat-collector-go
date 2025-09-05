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
	"errors"
	"os"

	"gopkg.in/yaml.v3"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	collectortypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/err"
	loggertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	DefaultHertzBeatCollectorName = "hertzbeat-collector"
)

type Loader struct {
	cfgPath string
	logger  logger.Logger
}

func New(cfgPath string) *Loader {

	return &Loader{
		cfgPath: cfgPath,
	}
}

func (l *Loader) LoadConfig() (*config.CollectorConfig, error) {

	l.logger = logger.DefaultLogger(os.Stdout, loggertypes.LogLevelInfo).WithName("config-loader")

	if l.cfgPath == "" {
		l.logger.Info("collector-config-loader: path is empty")
		return nil, errors.New("collector-config-loader: path is empty")
	}

	if _, err := os.Stat(l.cfgPath); os.IsNotExist(err) {
		l.logger.Error(err, "collector-config-loader: file not exist", "path", l.cfgPath)
		return nil, err
	}

	file, err := os.Open(l.cfgPath)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			l.logger.Error(err, "close config file failed")
		}
	}(file)

	var cfg config.CollectorConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		l.logger.Error(err, "decode config file failed")
		return nil, err
	}

	return &cfg, nil
}

func (l *Loader) ValidateConfig(cfg *config.CollectorConfig) error {

	if cfg == nil {
		l.logger.Error(collectortypes.CollectorConfigIsNil, "collector-config-loader is nil")
		return errors.New("collector-config-loader is nil")
	}

	// other check
	if cfg.Collector.Info.IP == "" {
		l.logger.Error(collectortypes.CollectorIPIsNil, "collector ip is empty")
		return errors.New("collector-config-loader ip is empty")
	}

	if cfg.Collector.Info.Port == "" {
		l.logger.Error(collectortypes.CollectorPortIsNil, "collector-config-loader: port is empty")
		return errors.New("collector-config-loader port is empty")
	}

	if cfg.Collector.Info.Name == "" {
		l.logger.Sugar().Debug("collector-config-loader: name is empty")
		cfg.Collector.Info.Name = DefaultHertzBeatCollectorName
	}

	return nil
}
