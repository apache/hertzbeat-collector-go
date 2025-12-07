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
