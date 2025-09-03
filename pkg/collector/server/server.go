// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package server

import (
	"context"
	"os"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/transport"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
)

const (
	DefaultHertzBeatCollectorVersion = "0.0.1-DEV"
)

type Run interface {
	Start(ctx context.Context) error
	Close() error
}

// CollectorServer HertzBeat Collector Server
type CollectorServer struct {
	Version string
	Logger  logger.Logger

	job       *job.Server
	transport *transport.Server
}

func NewCollectorServer(version string) *CollectorServer {

	if version == "" {
		version = DefaultHertzBeatCollectorVersion
	}

	return &CollectorServer{
		Version: version,
		Logger:  logger.DefaultLogger(os.Stdout, types.LogLevelDebug),
	}
}

func (s *CollectorServer) Start(ctx context.Context) error {

	s.Logger.Info("hi, starting collector server...")

	// start job server
	s.job = job.NewServer(s.Logger.WithName("job"))

	// init and start transport server
	s.transport = transport.NewServer(s.Logger.WithName("transport"))

	// Wait until done
	<-ctx.Done()

	return nil
}

func (s *CollectorServer) Validate() error {

	return nil
}

// Close Shutdown the server hook
func (s *CollectorServer) Close() error {

	s.Logger.Info("collector server shutting down... bye!")
	return nil
}
