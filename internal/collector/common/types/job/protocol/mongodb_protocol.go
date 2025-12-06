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

// MongoDBProtocol represents MongoDB protocol configuration
type MongoDBProtocol struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Database     string `json:"database"`
	AuthDatabase string `json:"authDatabase"`
	Command      string `json:"command"`
	Timeout      int    `json:"timeout"`

	logger logger.Logger
}

type MongoDBProtocolOption func(protocol *MongoDBProtocol)

func NewMongoDBProtocol(host string, port int, logger logger.Logger, opts ...MongoDBProtocolOption) *MongoDBProtocol {
	p := &MongoDBProtocol{
		Host:   host,
		Port:   port,
		logger: logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *MongoDBProtocol) IsInvalid() error {
	if p.Host == "" {
		p.logger.Error(ErrorInvalidHost, "mongodb protocol host is empty")
		return ErrorInvalidHost
	}
	if p.Port == 0 {
		p.logger.Error(ErrorInvalidPort, "mongodb protocol port is empty")
		return ErrorInvalidPort
	}
	return nil
}
