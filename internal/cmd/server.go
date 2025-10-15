/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	bannerouter "hertzbeat.apache.org/hertzbeat-collector-go/internal/banner"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/collect"
	jobserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/job"
	clrserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/server"
	transportserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/transport"
	collectortypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/collector"
	configtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	collectorerr "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/err"
	cfgloader "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/config"
)

var (
	cfgPath string
)

type Runner[I collectortypes.Info] interface {
	Start(ctx context.Context) error
	Info() I
	Close() error
}

func ServerCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"server", "srv", "s"},
		Short:   "Server Hertzbeat Collector Go",
		RunE: func(cmd *cobra.Command, args []string) error {
			return server(cmd.Context(), cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "config file path")
	return cmd
}

func getConfigByPath() (*configtypes.CollectorConfig, error) {

	loader := cfgloader.New(cfgPath)
	cfg, err := loader.LoadConfig()
	if err != nil {
		return nil, err
	}

	if err := loader.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func serverByCfg(cfg *configtypes.CollectorConfig, logOut io.Writer) *clrserver.Server {

	return clrserver.New(cfg, logOut)
}

func server(ctx context.Context, logOut io.Writer) error {

	cfg, err := getConfigByPath()
	if err != nil {
		return err
	}

	collectorServer := serverByCfg(cfg, logOut)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	banner := bannerouter.New(&bannerouter.Config{
		Server: *collectorServer,
	})
	err = banner.PrintBanner(cfg.Collector.Info.Name, cfg.Collector.Info.IP)
	if err != nil {
		return err
	}

	return startRunners(ctx, collectorServer)
}

func startRunners(ctx context.Context, cfg *clrserver.Server) error {
	// Create transport server first
	transportRunner := transportserver.NewFromConfig(cfg.Config)

	// Create lazy message router that can get transport client when needed
	messageRouter := collect.NewLazyMessageRouter(transportRunner, cfg.Logger, cfg.Config.Collector.Identity)

	// Create job server with message router
	jobRunner := jobserver.New(&jobserver.Config{
		Server:        *cfg,
		MessageRouter: messageRouter,
	})

	// Connect transport to job scheduler
	transportRunner.SetJobScheduler(jobRunner)

	runners := []struct {
		runner Runner[collectortypes.Info]
	}{
		{jobRunner},
		{transportRunner},
		// todo; add metrics
	}

	errCh := make(chan error, len(runners))

	var wg sync.WaitGroup

	for _, r := range runners {
		wg.Add(1)
		go func(runner Runner[collectortypes.Info]) {
			defer wg.Done()
			cfg.Logger.Info("Starting runner", "runner component", runner.Info().Name)
			if err := runner.Start(ctx); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(r.runner)
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	cleanup := func() {
		signal.Stop(signalCh)
		for _, r := range runners {
			if err := r.runner.Close(); err != nil {
				cfg.Logger.Info("error closing runner %s: %v\n", r.runner.Info(), err)
			}
		}
	}

	select {
	case <-ctx.Done():
		cfg.Logger.Info("Context cancelled")
		cleanup()
		return ctx.Err()
	case sig := <-signalCh:
		cfg.Logger.Info("Received signal: %v\n", sig)
		cleanup()
		return nil
	case err := <-errCh:
		cleanup()
		cfg.Logger.Error(collectorerr.CollectorServerStop, "runner error", "error", err)
		return err
	}
}
