package config

import (
	"context"
	"errors"
	"os"

	"gopkg.in/yaml.v3"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/internel"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
)

const (
	DefaultHertzBeatCollectorName    = "hertzbeat-collector"
	DefaultHertzBeatCollectorVersion = "0.0.1-DEV"
)

type HookFunc func(c context.Context, server *internel.CollectorServer) error

type Loader struct {
	cfgPath string
	logger  logger.Logger
	cancel  context.CancelFunc
	server  *internel.CollectorServer

	hook HookFunc

	// todo file watcher
	// watcher *fsnotify.Watcher
}

func New(cfgPath string, server *internel.CollectorServer, f HookFunc) *Loader {

	return &Loader{
		cfgPath: cfgPath,
		server:  server,
		logger:  server.Logger.WithName("collector-config-loader"),
		hook:    f,
	}
}

func (ld *Loader) LoadConfig() (*types.CollectorConfig, error) {

	ld.runHook()

	if ld.cfgPath == "" {
		ld.logger.Info("collector-config-loader: path is empty")
		return nil, errors.New("collector-config-loader: path is empty")
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

	var cfg types.CollectorConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		ld.logger.Error(err, "decode config file failed")
		return nil, err
	}

	return &cfg, nil
}

func (ld *Loader) ValidateConfig(cfg *types.CollectorConfig) error {

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

	if cfg.Collector.Info.Version == "" {
		ld.logger.Sugar().Debug("collector-config-loader version is empty, use default version")
		cfg.Collector.Info.Version = DefaultHertzBeatCollectorVersion
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
