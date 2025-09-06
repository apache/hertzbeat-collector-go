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

package transport

import (
	"context"

	clrServer "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/collector"
)

type Config struct {
	clrServer.Server
}

type Runner struct {
	Config
}

func New(srv *Config) *Runner {

	return &Runner{
		Config: *srv,
	}
}

func (r *Runner) Start(ctx context.Context) error {

	r.Logger = r.Logger.WithName(r.Info().Name).WithValues("runner", r.Info().Name)

	r.Logger.Info("Starting transport server")

	select {
	case <-ctx.Done():
		return nil
	}
}

func (r *Runner) Info() collector.Info {

	return collector.Info{
		Name: "transport",
	}
}

func (r *Runner) Close() error {

	r.Logger.Info("transport close...")
	return nil
}
