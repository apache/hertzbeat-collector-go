package server

import (
	"io"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	logger2 "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	HertzbeatCollectorGoName = "HertzbeatCollectorGoImpl"
)

type Server struct {
	Name   string
	Logger logger.Logger
	Config *config.CollectorConfig
}

func New(cfg *config.CollectorConfig, logOut io.Writer) *Server {

	return &Server{
		Config: cfg,
		Name:   HertzbeatCollectorGoName,
		Logger: logger.DefaultLogger(logOut, logger2.LogLevelInfo),
	}
}

func (s *Server) Validate() error {

	// something validate

	return nil
}
