package protocol

import (
  "hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// MilvusProtocol represents Milvus protocol configuration
type MilvusProtocol struct {
  Host     string `json:"host"`
  Port     string `json:"port"`
  Username string `json:"username"`
  Password string `json:"password"`
  Timeout  string `json:"timeout"`

  logger logger.Logger
}

type MilvusProtocolOption func(protocol *MilvusProtocol)

func NewMilvusProtocol(host, port string, logger logger.Logger, opts ...MilvusProtocolOption) *MilvusProtocol {

  p := &MilvusProtocol{
    Host:   host,
    Port:   port,
    logger: logger,
  }
  for _, opt := range opts {
    opt(p)
  }
  return p
}

func (p *MilvusProtocol) IsInvalid() error {
  if p.Host == "" {
    p.logger.Error(ErrorInvalidHost, "milvus protocol host is empty")
    return ErrorInvalidHost
  }
  if p.Port == "" {
    p.logger.Error(ErrorInvalidPort, "milvus protocol port is empty")
    return ErrorInvalidPort
  }
  return nil
}
