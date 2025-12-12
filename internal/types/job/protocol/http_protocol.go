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

// HTTPProtocol represents HTTP protocol configuration
type HTTPProtocol struct {
	URL           string            `json:"url"`
	Method        string            `json:"method"`
	Headers       map[string]string `json:"headers"`
	Params        map[string]string `json:"params"`
	Body          string            `json:"body"`
	ParseScript   string            `json:"parseScript"`
	ParseType     string            `json:"parseType"`
	Keyword       string            `json:"keyword"`
	Timeout       string            `json:"timeout"`
	SSL           string            `json:"ssl"`
	Authorization *Authorization    `json:"authorization"`

	logger logger.Logger
}

type HTTPProtocolOption func(protocol *HTTPProtocol)

func NewHTTPProtocol(url, method string, logger logger.Logger, opts ...HTTPProtocolOption) *HTTPProtocol {
	p := &HTTPProtocol{
		URL:    url,
		Method: method,
		logger: logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *HTTPProtocol) IsInvalid() error {
	if p.URL == "" {
		p.logger.Error(ErrorInvalidURL, "http protocol url is empty")
		return ErrorInvalidURL
	}
	return nil
}

// Authorization represents HTTP authorization configuration
type Authorization struct {
	Type               string `json:"type"`
	BasicAuthUsername  string `json:"basicAuthUsername"`
	BasicAuthPassword  string `json:"basicAuthPassword"`
	DigestAuthUsername string `json:"digestAuthUsername"`
	DigestAuthPassword string `json:"digestAuthPassword"`
	BearerTokenToken   string `json:"bearerTokenToken"`
}
