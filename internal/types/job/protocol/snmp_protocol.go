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

// SNMPProtocol represents SNMP protocol configuration
type SNMPProtocol struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Version     string `json:"version"`
	Community   string `json:"community"`
	Username    string `json:"username"`
	AuthType    string `json:"authType"`
	AuthPasswd  string `json:"authPasswd"`
	PrivType    string `json:"privType"`
	PrivPasswd  string `json:"privPasswd"`
	ContextName string `json:"contextName"`
	Timeout     int    `json:"timeout"`
	Operation   string `json:"operation"`
	OIDs        string `json:"oids"`

	logger logger.Logger
}

type SNMPProtocolOption func(protocol *SNMPProtocol)

func NewSNMPProtocol(host string, port int, logger logger.Logger, opts ...SNMPProtocolOption) *SNMPProtocol {
	p := &SNMPProtocol{
		Host:   host,
		Port:   port,
		logger: logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// todo add more check.
func (p *SNMPProtocol) IsInvalid() error {
	if p.Host == "" {
		p.logger.Error(ErrorInvalidHost, "snmp protocol host is empty")
		return ErrorInvalidHost
	}
	if p.Port == 0 {
		p.logger.Error(ErrorInvalidPort, "snmp protocol port is empty")
		return ErrorInvalidPort
	}
	return nil
}
