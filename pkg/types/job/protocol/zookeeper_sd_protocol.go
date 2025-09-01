package protocol

import (
	"errors"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

var (
	ErrorInvalidPathPrefix = errors.New("invalid path prefix")
)

type ZookeeperSdProtocol struct {
	URL        string
	PathPrefix string

	logger logger.Logger
}

func NewZookeeperSdProtocol(url, pathPrefix string, logger logger.Logger) *ZookeeperSdProtocol {

	return &ZookeeperSdProtocol{
		URL:        url,
		PathPrefix: pathPrefix,
		logger:     logger,
	}
}

func (zp *ZookeeperSdProtocol) IsInvalid() error {

	if zp.URL == "" {
		zp.logger.Error(ErrorInvalidURL, "zk sd protocol host is empty")
		return ErrorInvalidURL
	}
	if zp.PathPrefix == "" {
		zp.logger.Error(ErrorInvalidPathPrefix, "zk sd protocol port is empty")
		return ErrorInvalidPathPrefix
	}

	return nil
}
