package transport

import (
	"context"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

// RemotingService interface: copy java netty
// todo ?
type RemotingService interface {
	Start() error
	Shutdown() error
	isStart() error
}

type RemotingClient interface {
	RemotingService

	// todo add
}

type Server struct {
	logger logger.Logger
}

func NewServer(logger logger.Logger) *Server {

	return &Server{
		logger: logger,
	}
}

func (s *Server) Start(ctx context.Context) error {

	return nil
}

func (s *Server) Close() error {

	return nil
}
