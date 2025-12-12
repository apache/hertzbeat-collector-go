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

package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Job struct {
	App       string
	MonitorId int64
	Metadata  map[string]string
}

type HertzBeatMetricsCollector struct {
	collectTotal    *prometheus.CounterVec
	collectDuration *prometheus.HistogramVec
	once            sync.Once
}

func NewHertzBeatMetricsCollector() *HertzBeatMetricsCollector {
	collector := &HertzBeatMetricsCollector{}
	collector.once.Do(func() {
		collector.collectTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hertzbeat_collect_total",
				Help: "The total number of collection tasks executed",
			},
			[]string{"status", "monitor_type", "monitor_id", "monitor_name", "monitor_target"},
		)
		collector.collectDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hertzbeat_collect_duration",
				Help:    "The duration of collection task executions",
				Buckets: prometheus.ExponentialBuckets(10, 2, 10), // Example buckets
			},
			[]string{"status", "monitor_type", "monitor_id", "monitor_name", "monitor_target"},
		)
		prometheus.MustRegister(collector.collectTotal, collector.collectDuration)
	})
	return collector
}

func (c *HertzBeatMetricsCollector) RecordCollectMetrics(job *Job, durationMillis int64, status string) {
	if job == nil {
		return
	}
	monitorName := "unknown"
	monitorTarget := "unknown"
	if job.Metadata != nil {
		if v, ok := job.Metadata["instancename"]; ok {
			monitorName = v
		}
		if v, ok := job.Metadata["instancehost"]; ok {
			monitorTarget = v
		}
	}
	labels := prometheus.Labels{
		"status":         status,
		"monitor_type":   job.App,
		"monitor_id":     fmt.Sprintf("%d", job.MonitorId),
		"monitor_name":   monitorName,
		"monitor_target": monitorTarget,
	}
	c.collectTotal.With(labels).Inc()
	c.collectDuration.With(labels).Observe(float64(durationMillis))
}
