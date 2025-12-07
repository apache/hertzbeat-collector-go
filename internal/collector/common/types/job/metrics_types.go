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

package job

import (
	"fmt"
	"time"
)

// Metrics represents a metric configuration
type Metrics struct {
	Name        string            `json:"name"`
	I18n        interface{}       `json:"i18n,omitempty"` // Internationalization info
	Priority    int               `json:"priority"`
	CollectTime int64             `json:"collectTime"`
	Interval    int64             `json:"interval"`
	Visible     *bool             `json:"visible"`
	Fields      []Field           `json:"fields"`
	AliasFields []string          `json:"aliasFields"`
	Calculates  interface{}       `json:"calculates"` // Can be []Calculate or []string
	Filters     interface{}       `json:"filters"`
	Units       interface{}       `json:"units"` // Can be []Unit or []string
	Protocol    string            `json:"protocol"`
	Host        string            `json:"host"`
	Port        string            `json:"port"`
	Timeout     string            `json:"timeout"`
	Range       string            `json:"range"`
	ConfigMap   map[string]string `json:"configMap"`
	HasSubTask  bool              `json:"hasSubTask"`

	// Protocol specific fields
	HTTP    *HTTPProtocol    `json:"http,omitempty"`
	SSH     interface{}      `json:"ssh,omitempty"`  // Can be SSHProtocol
	JDBC    interface{}      `json:"jdbc,omitempty"` // Can be JDBCProtocol or map[string]interface{}
	SNMP    *SNMPProtocol    `json:"snmp,omitempty"`
	JMX     *JMXProtocol     `json:"jmx,omitempty"`
	Redis   *RedisProtocol   `json:"redis,omitempty"`
	MongoDB *MongoDBProtocol `json:"mongodb,omitempty"`
	Milvus  *MilvusProtocol  `json:"milvus,omitempty"`
}

// Field represents a metric field
type Field struct {
	Field    string      `json:"field"`
	Type     int         `json:"type"`
	Label    bool        `json:"label"`
	Unit     string      `json:"unit"`
	Instance bool        `json:"instance"`
	Value    interface{} `json:"value"`
	I18n     interface{} `json:"i18n,omitempty"` // Internationalization info
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
	ID           interface{} `json:"id"`
	App          interface{} `json:"app"`
	Field        string      `json:"field"`
	Name         interface{} `json:"name"` // Can be string or map[string]string for i18n
	Type         string      `json:"type"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"defaultValue"`
	Placeholder  string      `json:"placeholder"`
	Range        string      `json:"range"`
	Limit        int         `json:"limit"`
	Options      []Option    `json:"options"`
	KeyAlias     interface{} `json:"keyAlias"`
	ValueAlias   interface{} `json:"valueAlias"`
	Hide         bool        `json:"hide"`
	Creator      interface{} `json:"creator"`
	Modifier     interface{} `json:"modifier"`
	GmtCreate    interface{} `json:"gmtCreate"`
	GmtUpdate    interface{} `json:"gmtUpdate"`
	Depend       *Depend     `json:"depend"`
}

// GetName returns the name as string, handling both string and i18n map cases
func (p *ParamDefine) GetName() string {
	switch v := p.Name.(type) {
	case string:
		return v
	case map[string]interface{}:
		// Try to get English name first, then any available language
		if name, ok := v["en"]; ok {
			if s, ok := name.(string); ok {
				return s
			}
		}
		// Fall back to first available name
		for _, val := range v {
			if s, ok := val.(string); ok {
				return s
			}
		}
		return "unknown"
	default:
		return fmt.Sprintf("%v", v)
	}
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

// SSHProtocol represents SSH protocol configuration
type SSHProtocol struct {
	Host                 string `json:"host"`
	Port                 string `json:"port"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	PrivateKey           string `json:"privateKey"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
	Script               string `json:"script"`
	ParseType            string `json:"parseType"`
	ParseScript          string `json:"parseScript"`
	Timeout              string `json:"timeout"`
	ReuseConnection      string `json:"reuseConnection"`
	UseProxy             string `json:"useProxy"`
	ProxyHost            string `json:"proxyHost"`
	ProxyPort            string `json:"proxyPort"`
	ProxyUsername        string `json:"proxyUsername"`
	ProxyPassword        string `json:"proxyPassword"`
	ProxyPrivateKey      string `json:"proxyPrivateKey"`
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
	Host      string     `json:"host"`
	Port      string     `json:"port"`
	Username  string     `json:"username"`
	Password  string     `json:"password"`
	Pattern   string     `json:"pattern"`
	Timeout   string     `json:"timeout"`
	SSHTunnel *SSHTunnel `json:"sshTunnel,omitempty"`
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

// MilvusProtocol represents Milvus protocol configuration
type MilvusProtocol struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Timeout  string `json:"timeout"`
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
		for i, metric := range j.Metrics {
			clone.Metrics[i] = metric

			// Deep copy ConfigMap for each metric to avoid concurrent access
			if metric.ConfigMap != nil {
				clone.Metrics[i].ConfigMap = make(map[string]string, len(metric.ConfigMap))
				for k, v := range metric.ConfigMap {
					clone.Metrics[i].ConfigMap[k] = v
				}
			}

			// Deep copy Fields slice
			if metric.Fields != nil {
				clone.Metrics[i].Fields = make([]Field, len(metric.Fields))
				copy(clone.Metrics[i].Fields, metric.Fields)
			}

			// Deep copy AliasFields slice
			if metric.AliasFields != nil {
				clone.Metrics[i].AliasFields = make([]string, len(metric.AliasFields))
				copy(clone.Metrics[i].AliasFields, metric.AliasFields)
			}

			// Deep copy HTTP Protocol if exists
			if metric.HTTP != nil {
				cloneHTTP := *metric.HTTP
				if metric.HTTP.Headers != nil {
					cloneHTTP.Headers = make(map[string]string, len(metric.HTTP.Headers))
					for k, v := range metric.HTTP.Headers {
						cloneHTTP.Headers[k] = v
					}
				}
				if metric.HTTP.Params != nil {
					cloneHTTP.Params = make(map[string]string, len(metric.HTTP.Params))
					for k, v := range metric.HTTP.Params {
						cloneHTTP.Params[k] = v
					}
				}
				if metric.HTTP.Authorization != nil {
					cloneAuth := *metric.HTTP.Authorization
					cloneHTTP.Authorization = &cloneAuth
				}
				clone.Metrics[i].HTTP = &cloneHTTP
			}
		}
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
