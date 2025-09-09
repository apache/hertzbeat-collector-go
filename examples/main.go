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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	config "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/config"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/transport"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	loggerTypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
)

func main() {
	// Create simple logger
	logging := &loggerTypes.HertzBeatLogging{
		Level: map[loggerTypes.HertzbeatLogComponent]loggerTypes.LogLevel{
			loggerTypes.LogComponentHertzbeatDefault: loggerTypes.LogLevelInfo,
		},
	}
	log := logger.NewLogger(os.Stdout, logging)
	
	log.Info("=== HertzBeat Collector Go ===")
	
	// Load configuration from environment variables
	envLoader := config.NewEnvConfigLoader()
	cfg := envLoader.LoadFromEnv()
	
	if cfg == nil {
		log.Error(nil, "Failed to load configuration")
		os.Exit(1)
	}
	
	// Display configuration
	log.Info("=== Configuration ===")
	log.Info("Collector Identity", map[string]interface{}{"identity": cfg.Collector.Identity})
	log.Info("Collector Mode", map[string]interface{}{"mode": cfg.Collector.Mode})
	log.Info("Manager Host", map[string]interface{}{"host": cfg.Collector.Manager.Host})
	log.Info("Manager Port", map[string]interface{}{"port": cfg.Collector.Manager.Port})
	log.Info("Manager Protocol", map[string]interface{}{"protocol": cfg.Collector.Manager.Protocol})
	log.Info("====================")
	
	// Create transport runner from configuration
	runner := transport.NewFromConfig(cfg)
	if runner == nil {
		log.Error(nil, "Failed to create transport runner")
		os.Exit(1)
	}
	
	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		log.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()
	
	// Start the transport client
	log.Info("Starting HertzBeat Collector Go...")
	
	go func() {
		if err := runner.Start(ctx); err != nil {
			log.Error(err, "Transport client error")
			cancel()
		}
	}()
	
	// Wait for context cancellation
	<-ctx.Done()
	
	// Shutdown gracefully
	log.Info("Shutting down HertzBeat Collector Go...")
	if err := runner.Close(); err != nil {
		log.Error(err, "Error during shutdown")
	}
	
	log.Info("HertzBeat Collector Go stopped gracefully")
}
