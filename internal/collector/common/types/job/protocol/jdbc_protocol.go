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

// JDBCProtocol represents JDBC protocol configuration
type JDBCProtocol struct {
	Host            string     `json:"host"`
	Port            string     `json:"port"`
	Platform        string     `json:"platform"`
	Username        string     `json:"username"`
	Password        string     `json:"password"`
	Database        string     `json:"database"`
	Timeout         string     `json:"timeout"`
	QueryType       string     `json:"queryType"`
	SQL             string     `json:"sql"`
	URL             string     `json:"url"`
	ReuseConnection string     `json:"reuseConnection"`
	SSHTunnel       *SSHTunnel `json:"sshTunnel,omitempty"`

	logger logger.Logger
}

type JDBCProtocolOption func(protocol *JDBCProtocol)

func NewJDBCProtocol(host, port, platform string, logger logger.Logger, opts ...JDBCProtocolOption) *JDBCProtocol {
	p := &JDBCProtocol{
		Host:     host,
		Port:     port,
		Platform: platform,
		logger:   logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *JDBCProtocol) IsInvalid() error {
	if p.Host == "" {
		p.logger.Error(ErrorInvalidHost, "jdbc protocol host is empty")
		return ErrorInvalidHost
	}
	if p.Port == "" {
		p.logger.Error(ErrorInvalidPort, "jdbc protocol port is empty")
		return ErrorInvalidPort
	}
	return nil
}
