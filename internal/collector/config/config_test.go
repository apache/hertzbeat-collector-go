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

package config

import (
	"os"
	"testing"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/server"
)

func TestLoadConfig(t *testing.T) {

	content := `{
        "collector": {
            "info": {
            	"ip": "127.0.0.1"
			},
        },
        "dispatcher": {}
    }`
	dumpfile, err := os.CreateTemp("", "collector_config_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}(dumpfile.Name())
	if _, err := dumpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	err = dumpfile.Close()
	if err != nil {
		return
	}

	collectorServer := server.NewCollectorServer("0.0.1-DEV")
	loader := New(dumpfile.Name(), collectorServer, nil)

	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Collector.Info.IP != "127.0.0.1" {
		t.Errorf("expected ip '127.0.0.1', got '%s'", cfg.Collector.Info.IP)
	}
}
