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
	Name                map[string]string `json:"name"`
	Help                map[string]string `json:"help"`
	HelpLink            map[string]string `json:"helpLink"`
	Timestamp           int64             `json:"timestamp"`
	DefaultInterval     int64             `json:"defaultInterval"`
	Intervals           []int64           `json:"intervals"`
	IsCyclic            bool              `json:"isCyclic"`
	Params              []ParamDefine     `json:"params"`
	Metrics             []Metrics         `json:"metrics"`
	Configmap           []Configmap       `json:"configmap"`
	IsSd                bool              `json:"isSd"`
	PrometheusProxyMode bool              `json:"prometheusProxyMode"`

	// Internal fields
	EnvConfigmaps    map[string]Configmap `json:"-"`
	DispatchTime     int64                `json:"-"`
	PriorMetrics     []Metrics            `json:"-"`
	ResponseDataTemp []MetricsData        `json:"-"`
}

// GetNextCollectMetrics returns the metrics that should be collected next
// This is a simplified version - in the full implementation this would handle
// metric priorities, dependencies, and collection levels
func (j *Job) GetNextCollectMetrics() []*Metrics {
	result := make([]*Metrics, 0, len(j.Metrics))
	for i := range j.Metrics {
		result = append(result, &j.Metrics[i])
	}
	return result
}

// ConstructPriorMetrics prepares prior metrics for collection dependencies
func (j *Job) ConstructPriorMetrics() {
	j.PriorMetrics = make([]Metrics, len(j.Metrics))
	copy(j.PriorMetrics, j.Metrics)
}
