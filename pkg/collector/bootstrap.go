package collector

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/banner"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/config"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/server"
)

func Bootstrap(confPath, version string) error {

	// Init collector server
	cs := server.NewCollectorServer(version)

	// Load HertzBeat collector config
	loader := config.New(confPath, cs, nil)
	cfg, err := loader.LoadConfig()
	if err != nil {
		cs.Logger.Error(err, "load collector config failed")
		return err
	}
	err = loader.ValidateConfig(cfg)
	if err != nil {
		cs.Logger.Error(err, "validate collector config failed")
		return err
	}

	// todo: optimize log init eg. dynamic update log level

	// render banner
	err = banner.New(cs).PrintBanner(cfg.Collector.Info.Name, cfg.Collector.Info.Port)
	if err != nil {
		cs.Logger.Error(err, "print banner failed")
		return err
	}

	// Load collector job

	// check collector server
	err = cs.Validate()
	if err != nil {
		cs.Logger.Error(err, "validate collector server failed")
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

	err = cs.Start(ctx)
	if err != nil {
		cs.Logger.Error(err, "start collector server failed")
		return err
	}

	// shutdown collector server
	_ = cs.Close()

	return nil
}
