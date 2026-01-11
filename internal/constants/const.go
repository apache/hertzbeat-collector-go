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

// Package constants defines public common constants,
// ported from org.apache.hertzbeat.common.constants.CommonConstants
package constants

const (
	DefaultHertzBeatCollectorName = "hertzbeat-collector"
)

// Collect Response status code (Go Collector specific)
const (
	CollectSuccess       = 0 // Generic success
	CollectUnavailable   = 1 // Service unavailable (e.g., target service error)
	CollectUnReachable   = 2 // Network unreachable (e.g., network timeout, DNS fail)
	CollectUnConnectable = 3 // Connection failed (e.g., port closed, SSH auth fail)
	CollectFail          = 4 // Generic failure (e.g., query error, script exec fail)
	CollectTimeout       = 5 // Collection timeout
)

// Field parameter types (Aligned with CommonConstants.java)
const (
	TypeNumber = 0 // Field parameter type: number
	TypeString = 1 // Field parameter type: String
	TypeSecret = 2 // Field parameter type: encrypted string
	TypeTime   = 3 // Field parameter type: time
)

// Monitoring status (Aligned with CommonConstants.java)
const (
	MonitorPausedCode = 0 // 0: Paused
	MonitorUpCode     = 1 // 1: Up
	MonitorDownCode   = 2 // 2: Down
)

// Common metric/label keys
const (
	// Collection metric value: null placeholder for empty value
	NullValue = "&nbsp;"

	// Common metric keys
	ErrorMsg     = "errorMsg"
	ResponseTime = "responseTime"
	StatusCode   = "statusCode"

	// Label keys (Aligned with CommonConstants.java)
	LabelInstance      = "instance"
	LabelDefineID      = "defineid"
	LabelAlertName     = "alertname"
	LabelInstanceHost  = "instancehost"
	LabelInstanceName  = "instancename"
	LabelAlertSeverity = "severity"
)

// Service specific constants
const (
	MongoDBAtlasModel = "mongodb-atlas"

	// PostgreSQLUnReachAbleCode Specific SQLState for connection failure
	PostgreSQLUnReachAbleCode = "08001"
	// ZookeeperApp App name for Zookeeper
	ZookeeperApp = "zookeeper"
	// ZookeeperEnviHeader Header string in Zookeeper 'envi' command output
	ZookeeperEnviHeader = "Environment:"
)

// Function related constants
const (
	CollectorModule = "collector"
)

// Legacy or alias constants
// These are kept for compatibility with previous Go code logic
const (
	// FieldTypeString Alias for TypeString
	FieldTypeString = TypeString
	// KeyWord Deprecated placeholder
	KeyWord = "keyword"
)
