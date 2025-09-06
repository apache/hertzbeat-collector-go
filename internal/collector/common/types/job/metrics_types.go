/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package job

import "time"

// Metrics represents a metric configuration
type Metrics struct {
	Name        string            `json:"name"`
	Priority    int               `json:"priority"`
	Fields      []Field           `json:"fields"`
	Aliasfields []string          `json:"aliasfields"`
	Calculates  []Calculate       `json:"calculates"`
	Units       []Unit            `json:"units"`
	Protocol    string            `json:"protocol"`
	Host        string            `json:"host"`
	Port        string            `json:"port"`
	Timeout     string            `json:"timeout"`
	Interval    int64             `json:"interval"`
	Range       string            `json:"range"`
	Visible     *bool             `json:"visible"`
	ConfigMap   map[string]string `json:"configMap"`

	// Protocol specific fields
	HTTP    *HTTPProtocol    `json:"http,omitempty"`
	SSH     *SSHProtocol     `json:"ssh,omitempty"`
	JDBC    *JDBCProtocol    `json:"jdbc,omitempty"`
	SNMP    *SNMPProtocol    `json:"snmp,omitempty"`
	JMX     *JMXProtocol     `json:"jmx,omitempty"`
	Redis   *RedisProtocol   `json:"redis,omitempty"`
	MongoDB *MongoDBProtocol `json:"mongodb,omitempty"`
}

// Field represents a metric field
type Field struct {
	Field    string      `json:"field"`
	Type     int         `json:"type"`
	Label    bool        `json:"label"`
	Unit     string      `json:"unit"`
	Instance bool        `json:"instance"`
	Value    interface{} `json:"value"`
}

// Calculate represents a calculation configuration
type Calculate struct {
	Field      string `json:"field"`
	Script     string `json:"script"`
	AliasField string `json:"aliasField"`
}

// Unit represents a unit conversion configuration
type Unit struct {
	Field string `json:"field"`
	Unit  string `json:"unit"`
}

// ParamDefine represents a parameter definition
type ParamDefine struct {
	Field        string      `json:"field"`
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"defaultValue"`
	Placeholder  string      `json:"placeholder"`
	Range        string      `json:"range"`
	Limit        int         `json:"limit"`
	Options      []Option    `json:"options"`
	Depend       *Depend     `json:"depend"`
	Hide         bool        `json:"hide"`
}

// Option represents a parameter option
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Depend represents a parameter dependency
type Depend struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// Configmap represents a configuration map entry
type Configmap struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Type   int         `json:"type"`
	Option []string    `json:"option"`
}

// MetricsData represents collected metrics data
type MetricsData struct {
	ID       int64             `json:"id"`
	TenantID int64             `json:"tenantId"`
	App      string            `json:"app"`
	Metrics  string            `json:"metrics"`
	Priority int               `json:"priority"`
	Time     int64             `json:"time"`
	Code     int               `json:"code"`
	Msg      string            `json:"msg"`
	Fields   []Field           `json:"fields"`
	Values   []ValueRow        `json:"values"`
	Metadata map[string]string `json:"metadata"`
	Labels   map[string]string `json:"labels"`
}

// ValueRow represents a row of metric values
type ValueRow struct {
	Columns []string `json:"columns"`
}

// Protocol specific types

// HTTPProtocol represents HTTP protocol configuration
type HTTPProtocol struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Params      map[string]string `json:"params"`
	Body        string            `json:"body"`
	ParseScript string            `json:"parseScript"`
	ParseType   string            `json:"parseType"`
	Keyword     string            `json:"keyword"`
	Username    string            `json:"username"`
	Password    string            `json:"password"`
	SSL         bool              `json:"ssl"`
}

// SSHProtocol represents SSH protocol configuration
type SSHProtocol struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	PrivateKey  string `json:"privateKey"`
	Script      string `json:"script"`
	ParseScript string `json:"parseScript"`
	Timeout     int    `json:"timeout"`
}

// JDBCProtocol represents JDBC protocol configuration
type JDBCProtocol struct {
	Host            string     `json:"host"`
	Port            string     `json:"port"`
	Platform        string     `json:"platform"`
	Username        string     `json:"username"`
	Password        string     `json:"password"`
	Database        string     `json:"database"`
	Timeout         string     `json:"timeout"`
	QueryType       string     `json:"queryType"`
	SQL             string     `json:"sql"`
	URL             string     `json:"url"`
	ReuseConnection string     `json:"reuseConnection"`
	SSHTunnel       *SSHTunnel `json:"sshTunnel,omitempty"`
}

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
}

// JMXProtocol represents JMX protocol configuration
type JMXProtocol struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
	Timeout  int    `json:"timeout"`
}

// RedisProtocol represents Redis protocol configuration
type RedisProtocol struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Pattern  string `json:"pattern"`
	Timeout  int    `json:"timeout"`
}

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
}

// GetInterval returns the interval for the metric, using default if not set
func (m *Metrics) GetInterval() time.Duration {
	if m.Interval > 0 {
		return time.Duration(m.Interval) * time.Second
	}
	return 30 * time.Second // default interval
}

// Clone creates a deep copy of the job
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}

	clone := *j

	// Deep copy maps
	if j.Metadata != nil {
		clone.Metadata = make(map[string]string, len(j.Metadata))
		for k, v := range j.Metadata {
			clone.Metadata[k] = v
		}
	}

	if j.Labels != nil {
		clone.Labels = make(map[string]string, len(j.Labels))
		for k, v := range j.Labels {
			clone.Labels[k] = v
		}
	}

	if j.Annotations != nil {
		clone.Annotations = make(map[string]string, len(j.Annotations))
		for k, v := range j.Annotations {
			clone.Annotations[k] = v
		}
	}

	// Deep copy slices
	if j.Intervals != nil {
		clone.Intervals = make([]int64, len(j.Intervals))
		copy(clone.Intervals, j.Intervals)
	}

	if j.Params != nil {
		clone.Params = make([]ParamDefine, len(j.Params))
		copy(clone.Params, j.Params)
	}

	if j.Metrics != nil {
		clone.Metrics = make([]Metrics, len(j.Metrics))
		copy(clone.Metrics, j.Metrics)
	}

	if j.Configmap != nil {
		clone.Configmap = make([]Configmap, len(j.Configmap))
		copy(clone.Configmap, j.Configmap)
	}

	return &clone
}

// CollectRepMetricsData represents the collected metrics data response.
type CollectRepMetricsData struct {
	ID        int64             `json:"id,omitempty"`
	MonitorID int64             `json:"monitorId,omitempty"`
	TenantID  int64             `json:"tenantId,omitempty"`
	App       string            `json:"app,omitempty"`
	Metrics   string            `json:"metrics,omitempty"`
	Priority  int               `json:"priority,omitempty"`
	Time      int64             `json:"time,omitempty"`
	Code      int               `json:"code,omitempty"`
	Msg       string            `json:"msg,omitempty"`
	Fields    []Field           `json:"fields,omitempty"`
	Values    []ValueRow        `json:"values,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SSHTunnel represents SSH tunnel configuration
type SSHTunnel struct {
	Enable   string `json:"enable"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CollectResponseEventListener defines the interface for handling collect response events
type CollectResponseEventListener interface {
	Response(metricsData []CollectRepMetricsData)
}
