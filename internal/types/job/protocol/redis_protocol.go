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

// RedisProtocol represents Redis protocol configuration
type RedisProtocol struct {
	Host      string     `json:"host"`
	Port      string     `json:"port"`
	Username  string     `json:"username"`
	Password  string     `json:"password"`
	Pattern   string     `json:"pattern"`
	Timeout   string     `json:"timeout"`
	SSHTunnel *SSHTunnel `json:"sshTunnel,omitempty"`

	logger logger.Logger
}

type RedisProtocolOption func(protocol *RedisProtocol)

func NewRedisProtocol(host, port string, logger logger.Logger, opts ...RedisProtocolOption) *RedisProtocol {
	p := &RedisProtocol{
		Host:   host,
		Port:   port,
		logger: logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *RedisProtocol) IsInvalid() error {
	if p.Host == "" {
		p.logger.Error(ErrorInvalidHost, "redis protocol host is empty")
		return ErrorInvalidHost
	}
	if p.Port == "" {
		p.logger.Error(ErrorInvalidPort, "redis protocol port is empty")
		return ErrorInvalidPort
	}
	return nil
}
