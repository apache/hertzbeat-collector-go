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

package logger

// hertzbeat logger related types

type LogLevel string

const (
	// LogLevelTrace defines the "Trace" logger level.
	LogLevelTrace LogLevel = "trace"

	// LogLevelDebug defines the "debug" logger level.
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo defines the "Info" logger level.
	LogLevelInfo LogLevel = "info"

	// LogLevelWarn defines the "Warn" logger level.
	LogLevelWarn LogLevel = "warn"

	// LogLevelError defines the "Error" logger level.
	LogLevelError LogLevel = "error"
)

type HertzBeatLogging struct {
	Level map[HertzbeatLogComponent]LogLevel `json:"level,omitempty"`
}

type HertzbeatLogComponent string

const (
	LogComponentHertzbeatDefault HertzbeatLogComponent = "default"

	LogComponentHertzbeatCollector HertzbeatLogComponent = "collector"
)

func DefaultHertzbeatLogging() *HertzBeatLogging {

	return &HertzBeatLogging{
		Level: map[HertzbeatLogComponent]LogLevel{
			LogComponentHertzbeatDefault: LogLevelInfo,
		},
	}
}

func (logging *HertzBeatLogging) DefaultHertzBeatLoggingLevel(level LogLevel) LogLevel {

	if level != "" {
		return level
	}

	if logging.Level[LogComponentHertzbeatDefault] != "" {

		return logging.Level[LogComponentHertzbeatDefault]
	}

	return LogLevelInfo
}

func (logging *HertzBeatLogging) SetHertzBeatLoggingDefaults() {

	if logging != nil && logging.Level != nil && logging.Level[LogComponentHertzbeatDefault] == "" {

		logging.Level[LogComponentHertzbeatDefault] = LogLevelInfo
	}
}
