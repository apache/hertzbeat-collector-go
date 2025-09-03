package collector

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/banner"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
)

func Bootstrap(confPath, version string) error {
	// Initialize logger
	log := logger.DefaultLogger(os.Stdout, types.LogLevelInfo)
	log.Info("starting HertzBeat Collector Go", "version", version)

	// Print banner
	bannerPrinter := banner.New(&BannerAdapter{logger: log})
	err := bannerPrinter.PrintBannerWithVersion("HertzBeat-Collector-Go", "1159", version)
	if err != nil {
		log.Error(err, "failed to print banner")
		return err
	}

	// Create and initialize the collector application
	app := NewCollectorApp(confPath, version, log)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("received shutdown signal")
		cancel()
	}()

	// Start the application
	if err := app.Start(); err != nil {
		log.Error(err, "failed to start collector application")
		return err
	}

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	log.Info("shutting down collector application")
	if err := app.Stop(); err != nil {
		log.Error(err, "error during shutdown")
		return err
	}

	log.Info("HertzBeat Collector Go stopped successfully")
	return nil
}

// CollectorApp wraps the complete collector application
type CollectorApp struct {
	confPath string
	version  string
	logger   logger.Logger

	// Core components
	collectService *CollectService
}

// NewCollectorApp creates a new collector application
func NewCollectorApp(confPath, version string, logger logger.Logger) *CollectorApp {
	return &CollectorApp{
		confPath: confPath,
		version:  version,
		logger:   logger.WithName("collector-app"),
	}
}

// Start starts the collector application
func (app *CollectorApp) Start() error {
	app.logger.Info("initializing collector application", "version", app.version)

	// Initialize collect service with enhanced scheduling
	app.collectService = NewCollectService(app.logger)

	// Register built-in collectors
	RegisterBuiltinCollectors(app.collectService, app.logger)

	app.logger.Info("collector application started successfully")
	return nil
}

// Stop stops the collector application
func (app *CollectorApp) Stop() error {
	app.logger.Info("stopping collector application")

	// TODO: Implement graceful shutdown of components

	app.logger.Info("collector application stopped successfully")
	return nil
}

// BannerAdapter adapts logger interface for banner printer
type BannerAdapter struct {
	logger logger.Logger
}

func (ba *BannerAdapter) Error(err error, msg string) {
	ba.logger.Error(err, msg)
}

func (ba *BannerAdapter) Info(msg string, keysAndValues ...interface{}) {
	ba.logger.Info(msg, keysAndValues...)
}
