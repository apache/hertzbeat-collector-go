package protocol

import (
	"errors"
)

var (
	ErrorInvalidHost = errors.New("invalid host")
	ErrorInvalidPort = errors.New("invalid port")
	ErrorInvalidURL  = errors.New("invalid URL")
)

type CommonRequestProtocol interface {
	SetHost(host string) error
	SetPort(port int) error
}
