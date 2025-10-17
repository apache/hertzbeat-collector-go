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

package job

import (
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// hertzbeat Collect Job related types

// Job represents a complete monitoring job
type Job struct {
	ID                  int64             `json:"id"`
	TenantID            int64             `json:"tenantId"`
	MonitorID           int64             `json:"monitorId"`
	Metadata            map[string]string `json:"metadata"`
	Labels              map[string]string `json:"labels"`
	Annotations         map[string]string `json:"annotations"`
	Hide                bool              `json:"hide"`
	Category            string            `json:"category"`
	App                 string            `json:"app"`
	Name                interface{}       `json:"name"`     // Can be string or map[string]string for i18n
	Help                interface{}       `json:"help"`     // Can be string or map[string]string for i18n
	HelpLink            interface{}       `json:"helpLink"` // Can be string or map[string]string for i18n
	Timestamp           int64             `json:"timestamp"`
	DefaultInterval     int64             `json:"defaultInterval"`
	Intervals           []int64           `json:"intervals"`
	IsCyclic            bool              `json:"isCyclic"`
	Params              []ParamDefine     `json:"params"`
	Metrics             []Metrics         `json:"metrics"`
	Configmap           []Configmap       `json:"configmap"`
	IsSd                bool              `json:"isSd"`
	Sd                  bool              `json:"sd"`
	PrometheusProxyMode bool              `json:"prometheusProxyMode"`
	Cyclic              bool              `json:"cyclic"`
	Interval            int64             `json:"interval"`

	// Internal fields
	EnvConfigmaps    map[string]Configmap `json:"-"`
	DispatchTime     int64                `json:"-"`
	PriorMetrics     [][]*Metrics         `json:"-"` // LinkedList of Set<Metrics>
	ResponseDataTemp []MetricsData        `json:"-"`

	mu sync.Mutex // Mutex for thread-safe operations
}

// NextCollectMetrics returns the metrics that should be collected next
// This is a simplified version - in the full implementation this would handle
// metric priorities, dependencies, and collection levels
func (j *Job) NextCollectMetrics() []*Metrics {
	result := make([]*Metrics, 0, len(j.Metrics))
	for i := range j.Metrics {
		result = append(result, &j.Metrics[i])
	}
	return result
}

// ConstructPriorMetrics prepares prior metrics for collection dependencies
// This method filters metrics that need to be collected based on time,
// sets default values, groups by priority, and constructs an ordered collection queue
func (j *Job) ConstructPriorMetrics() {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now().UnixMilli()

	// Filter metrics that need to be collected and set default values
	currentCollectMetrics := make(map[int][]*Metrics)

	for i := range j.Metrics {
		metric := &j.Metrics[i]

		// Check if it's time to collect this metric
		if now >= metric.CollectTime+metric.Interval*1000 {
			// Update collect time
			metric.CollectTime = now

			// Set default AliasFields if not configured
			if len(metric.AliasFields) == 0 && len(metric.Fields) > 0 {
				aliasFields := make([]string, len(metric.Fields))
				for idx, field := range metric.Fields {
					aliasFields[idx] = field.Field
				}
				metric.AliasFields = aliasFields
			}

			// Group by priority
			priority := metric.Priority
			currentCollectMetrics[priority] = append(currentCollectMetrics[priority], metric)
		}
	}

	// If no metrics need to be collected, add the default availability metric (priority 0)
	if len(currentCollectMetrics) == 0 {
		for i := range j.Metrics {
			metric := &j.Metrics[i]
			if metric.Priority == 0 {
				metric.CollectTime = now
				currentCollectMetrics[0] = []*Metrics{metric}
				break
			}
		}

		// If still empty, log error
		if len(currentCollectMetrics) == 0 {
			log.Println("ERROR: metrics must has one priority 0 metrics at least.")
		}
	}

	// Construct a linked list (slice) of task execution order of the metrics
	// Each element is a set (slice) of metrics with the same priority
	j.PriorMetrics = make([][]*Metrics, 0, len(currentCollectMetrics))

	for _, metricsSet := range currentCollectMetrics {
		// Make a copy to ensure thread safety
		metricsCopy := make([]*Metrics, len(metricsSet))
		copy(metricsCopy, metricsSet)
		j.PriorMetrics = append(j.PriorMetrics, metricsCopy)
	}

	// Sort by priority (ascending order)
	sort.Slice(j.PriorMetrics, func(i, k int) bool {
		// Get priority from the first metric in each set
		var iPriority, kPriority int = math.MaxInt, math.MaxInt

		if len(j.PriorMetrics[i]) > 0 && j.PriorMetrics[i][0] != nil {
			iPriority = j.PriorMetrics[i][0].Priority
		}
		if len(j.PriorMetrics[k]) > 0 && j.PriorMetrics[k][0] != nil {
			kPriority = j.PriorMetrics[k][0].Priority
		}

		return iPriority < kPriority
	})

	// Initialize EnvConfigmaps
	if j.EnvConfigmaps == nil {
		j.EnvConfigmaps = make(map[string]Configmap, 8)
	}
}
