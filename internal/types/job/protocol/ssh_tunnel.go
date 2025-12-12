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

// SSHTunnel represents SSH tunnel configuration
type SSHTunnel struct {
	Enable   string `json:"enable"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`

	logger logger.Logger
}

type SSHTunnelOption func(tunnel *SSHTunnel)

func NewSSHTunnel(host, port string, logger logger.Logger, opts ...SSHTunnelOption) *SSHTunnel {
	t := &SSHTunnel{
		Host:   host,
		Port:   port,
		logger: logger.WithName("sshTunel-config"),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *SSHTunnel) IsInvalid() error {
	if t.Host == "" {
		t.logger.Error(ErrorInvalidHost, "ssh tunnel host is empty")
		return ErrorInvalidHost
	}
	if t.Port == "" {
		t.logger.Error(ErrorInvalidPort, "ssh tunnel port is empty")
		return ErrorInvalidPort
	}
	return nil
}
