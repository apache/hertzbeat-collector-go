package collector

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/banner"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/config"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/internel"
)

func Bootstrap(confPath, version string) error {

	// Init collector server
	server := internel.NewCollectorServer(version)

	server.Logger.Sugar().Debug("测试日志级别")

	// Load HertzBeat collector config
	loader := config.New(confPath, server, nil)
	cfg, err := loader.LoadConfig()
	if err != nil {
		server.Logger.Error(err, "load collector config failed")
		return err
	}
	err = loader.ValidateConfig(cfg)
	if err != nil {
		server.Logger.Error(err, "validate collector config failed")
		return err
	}

	// render banner
	err = banner.New(server).PrintBanner(cfg.Collector.Info.Name, cfg.Collector.Info.Port)
	if err != nil {
		server.Logger.Error(err, "print banner failed")
		return err
	}

	// Load collector job

	// check collector server
	err = server.Validate()
	if err != nil {
		server.Logger.Error(err, "validate collector server failed")
		return err
	}

	// Start collector server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	err = server.Start(ctx)
	if err != nil {
		server.Logger.Error(err, "start collector server failed")
		return err
	}

	// shutdown collector server
	_ = server.Close()

	return nil
}
