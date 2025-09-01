package job

import (
	"context"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

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
