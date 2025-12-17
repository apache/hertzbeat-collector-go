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

package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	clrserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/types/collector"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	Namespace = "hertzbeat"
	Subsystem = "collector"
)

var (
	// JobExecutionTotal counts the total number of job executions
	JobExecutionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: Subsystem,
			Name:      "job_execution_total",
			Help:      "Total number of job executions",
		},
		[]string{"status", "type"},
	)

	// JobExecutionDuration tracks the duration of job executions
	JobExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: Subsystem,
			Name:      "job_execution_duration_seconds",
			Help:      "Duration of job executions in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"type"},
	)

	// CollectorUp indicates if the collector is up
	CollectorUp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: Subsystem,
			Name:      "up",
			Help:      "1 if the collector is up, 0 otherwise",
		},
	)
)

func init() {
	// Register metrics with the global prometheus registry
	prometheus.MustRegister(JobExecutionTotal)
	prometheus.MustRegister(JobExecutionDuration)
	prometheus.MustRegister(CollectorUp)
	CollectorUp.Set(1)
}

// Runner implements the metrics server runner
type Runner struct {
	cfg    *clrserver.Server
	server *http.Server
}

// New creates a new metrics runner
func New(cfg *clrserver.Server) *Runner {

	return &Runner{cfg: cfg}
}

// Start starts the metrics server
func (r *Runner) Start(ctx context.Context) error {

	// init logger
	mlog := r.initLogs()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%d", r.cfg.Config.Collector.MetricsConfig.Port)
	r.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       15 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	mlog.Info("Starting metrics server", "addr", addr)

	go func() {
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			mlog.Error(err, "Metrics server failed")
		}
	}()

	<-ctx.Done()
	return r.Close()
}

// Info returns the runner info
func (r *Runner) Info() collector.Info {

	return collector.Info{
		Name: "metrics-server",
	}
}

// Close closes the metrics server
func (r *Runner) Close() error {

	if r.server != nil {
		r.initLogs().Info("Shutting down metrics server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.server.Shutdown(ctx)
	}
	return nil
}

func (r *Runner) initLogs() logger.Logger {

	return r.cfg.Logger.WithName("metrics")
}
