package server

import (
	"context"
	"os"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/transport"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
)

const (
	DefaultHertzBeatCollectorVersion = "0.0.1-DEV"
)

type Run interface {
	Start(ctx context.Context) error
	Close() error
}

// CollectorServer HertzBeat Collector Server
type CollectorServer struct {
	Version string
	Logger  logger.Logger

	job       *job.Server
	transport *transport.Server
}

func NewCollectorServer(version string) *CollectorServer {

	if version == "" {
		version = DefaultHertzBeatCollectorVersion
	}

	return &CollectorServer{
		Version: version,
		Logger:  logger.DefaultLogger(os.Stdout, types.LogLevelDebug),
	}
}

func (s *CollectorServer) Start(ctx context.Context) error {

	s.Logger.Info("hi, starting collector server...")

	// start job server
	s.job = job.NewServer(s.Logger.WithName("job"))

	// init and start transport server
	s.transport = transport.NewServer(s.Logger.WithName("transport"))

	// Wait until done
	<-ctx.Done()

	return nil
}

func (s *CollectorServer) Validate() error {

	return nil
}

// Close Shutdown the server hook
func (s *CollectorServer) Close() error {

	s.Logger.Info("collector server shutting down... bye!")
	return nil
}
