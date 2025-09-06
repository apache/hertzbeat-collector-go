/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"io"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	logger2 "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	HertzbeatCollectorGoName = "HertzbeatCollectorGoImpl"
)

type Server struct {
	Name   string
	Logger logger.Logger
	Config *config.CollectorConfig
}

func New(cfg *config.CollectorConfig, logOut io.Writer) *Server {

	return &Server{
		Config: cfg,
		Name:   HertzbeatCollectorGoName,
		Logger: logger.DefaultLogger(logOut, logger2.LogLevelInfo),
	}
}

func (s *Server) Validate() error {

	// something validate

	return nil
}
