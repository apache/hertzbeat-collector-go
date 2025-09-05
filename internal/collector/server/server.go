package server

import (
	"io"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/err"
	logger2 "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	HertzbeatCollectorGoName = "HertzbeatCollectorGoImpl"
)

type Server struct {
	config *config.CollectorConfig
	logger logger.Logger
	Name   string
}

func New(cfg *config.CollectorConfig, logOut io.Writer) *Server {

	return &Server{
		config: cfg,
		Name:   HertzbeatCollectorGoName,
		logger: logger.DefaultLogger(logOut, logger2.LogLevelInfo),
	}
}

func (s *Server) Validate() error {

	if s.config.Collector.Info.IP == "" {
		s.logger.Error(err.CollectorIPIsNull, "collector server start failed")
		return err.CollectorPortIsNull
	}

	if s.config.Collector.Info.Port == "" {
		s.logger.Error(err.CollectorPortIsNull, "collector server start failed")
		return err.CollectorPortIsNull
	}

	return nil
}
