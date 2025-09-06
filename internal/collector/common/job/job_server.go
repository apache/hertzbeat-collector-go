package transport

import (
	"context"

	clrServer "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/collector"
)

type Config struct {
	clrServer.Server
}

type Runner struct {
	Config
}

func New(srv *Config) *Runner {

	return &Runner{
		Config: *srv,
	}
}

func (r *Runner) Start(ctx context.Context) error {

	r.Logger = r.Logger.WithName(r.Info().Name).WithValues("runner", r.Info().Name)

	r.Logger.Info("Starting job server")

	select {
	case <-ctx.Done():
		return nil
	}
}

func (r *Runner) Info() collector.Info {

	return collector.Info{
		Name: "job",
	}
}

func (r *Runner) Close() error {

	r.Logger.Info("job close...")
	return nil
}
