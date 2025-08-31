package internel

import (
	"context"
	"os"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
)

type Run interface {
	Start(ctx context.Context) error
	Close() error
}

// CollectorServer HertzBeat Collector Server
type CollectorServer struct {
	Logger logger.Logger
}

func NewCollectorServer() *CollectorServer {

	return &CollectorServer{
		Logger: logger.DefaultLogger(os.Stdout, types.LogLevelInfo),
	}
}

func (s *CollectorServer) Start(ctx context.Context) error {

	s.Logger.Info("hi, starting collector server...")

	// Wait until done
	<-ctx.Done()

	return nil
}

func (s *CollectorServer) Validate() error {

	return nil
}

// Shutdown the server hook
func (s *CollectorServer) Close() error {

	s.Logger.Info("collector server shutting down... bye!")
	return nil
}
