package protocol

import "hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"

type ConsulSdProtocol struct {
	Host string
	Port string

	logger logger.Logger
}

func NewConsulSdProtocol(host, port string, logger logger.Logger) *ConsulSdProtocol {

	return &ConsulSdProtocol{
		Host:   host,
		Port:   port,
		logger: logger,
	}
}

func (cp *ConsulSdProtocol) IsInvalid() error {

	if cp.Host == "" {
		cp.logger.Error(ErrorInvalidHost, "consul sd protocol host is empty")
		return ErrorInvalidHost
	}
	if cp.Port == "" {
		cp.logger.Error(ErrorInvalidPort, "consul sd protocol port is empty")
		return ErrorInvalidPort
	}

	return nil
}
