/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package protocol

import (
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// SSHProtocol represents SSH protocol configuration
type SSHProtocol struct {
	Host                 string `json:"host"`
	Port                 string `json:"port"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	PrivateKey           string `json:"privateKey"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
	Script               string `json:"script"`
	ParseType            string `json:"parseType"`
	ParseScript          string `json:"parseScript"`
	Timeout              string `json:"timeout"`
	ReuseConnection      string `json:"reuseConnection"`
	UseProxy             string `json:"useProxy"`
	ProxyHost            string `json:"proxyHost"`
	ProxyPort            string `json:"proxyPort"`
	ProxyUsername        string `json:"proxyUsername"`
	ProxyPassword        string `json:"proxyPassword"`
	ProxyPrivateKey      string `json:"proxyPrivateKey"`

	logger logger.Logger
}

type SSHProtocolOption func(protocol *SSHProtocol)

func NewSSHProtocol(host, port string, logger logger.Logger, opts ...SSHProtocolOption) *SSHProtocol {
	p := &SSHProtocol{
		Host:   host,
		Port:   port,
		logger: logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *SSHProtocol) IsInvalid() error {
	if p.Host == "" {
		p.logger.Error(ErrorInvalidHost, "ssh protocol host is empty")
		return ErrorInvalidHost
	}
	if p.Port == "" {
		p.logger.Error(ErrorInvalidPort, "ssh protocol port is empty")
		return ErrorInvalidPort
	}
	return nil
}
