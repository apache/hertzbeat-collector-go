package cmd

import (
	"context"
	"fmt"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"collector-go/internal/collector/common/config"
	"collector-go/internal/collector/common/job"
	clrServer "collector-go/internal/collector/common/server"
	"collector-go/internal/collector/common/transport"
)

var (
	cfgPath string
)

type Runner[I types.Info] interface {
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
			return server(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "config file path")

	return cmd
}

func getConfigByPath() (*config.Config, error) {

	cfgServer, err := config.New(cfgPath)
	if err != nil {
		return nil, err
	}

	cfg, err := cfgServer.Loader()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func serverByCfg(cfg *config.Config) *clrServer.Server {

	return clrServer.New(cfg)
}

func server(ctx context.Context) error {
	cfg, err := getConfigByPath()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	collectorServer := serverByCfg(cfg)

	// 创建一个带取消的context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 启动runners并等待完成或错误
	return startRunners(ctx, collectorServer)
}

func startRunners(ctx context.Context, cfg *clrServer.Server) error {

	runners := []struct {
		runner Runner[types.Info]
	}{
		{
			job.New(&job.Config{
				Server: *cfg,
			}),
		},
		{
			transport.New(&transport.Config{
				Server: *cfg,
			}),
		},
		// metrics
	}

	errCh := make(chan error, len(runners))

	var wg sync.WaitGroup

	for _, r := range runners {
		wg.Add(1)
		go func(runner Runner[types.Info]) {
			defer wg.Done()
			fmt.Printf("Starting runner: %s\n", runner.Info().Name)

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
				fmt.Printf("error closing runner %s: %v\n", r.runner.Info(), err)
			}
		}
	}

	select {
	case <-ctx.Done():
		fmt.Println("Context cancelled")
		cleanup()
		return ctx.Err()
	case sig := <-signalCh:
		fmt.Printf("Received signal: %v\n", sig)
		cleanup()
		return nil
	case err := <-errCh:
		cleanup()
		return fmt.Errorf("runner error: %w", err)
	}
}
