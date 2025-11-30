/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package redis

import (
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	consts "hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

func TestRedisCollector_Protocol(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug)
	collector := NewRedisCollector(log)
	assert.Equal(t, ProtocolRedis, collector.Protocol())
}

func TestRedisCollector_PreCheck(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug)
	collector := NewRedisCollector(log)

	// Test nil metrics
	err := collector.PreCheck(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis configuration is required")

	// Test missing Redis configuration
	metrics := &jobtypes.Metrics{
		Name: "redis_test",
	}
	err = collector.PreCheck(metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis configuration is required")

	// Test missing host
	metrics.Redis = &jobtypes.RedisProtocol{
		Port: "6379",
	}
	err = collector.PreCheck(metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis host is required")

	// Test missing port
	metrics.Redis = &jobtypes.RedisProtocol{
		Host: "localhost",
	}
	err = collector.PreCheck(metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis port is required")

	// Test valid configuration
	metrics.Redis = &jobtypes.RedisProtocol{
		Host: "localhost",
		Port: "6379",
	}
	err = collector.PreCheck(metrics)
	assert.NoError(t, err)
}

func TestRedisCollector_Collect(t *testing.T) {
	// Start miniredis
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Could not start miniredis: %s", err)
	}
	defer s.Close()

	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug)
	collector := NewRedisCollector(log)

	// Parse host and port from miniredis address
	addrParts := strings.Split(s.Addr(), ":")
	host := addrParts[0]
	port := addrParts[1]

	metrics := &jobtypes.Metrics{
		Name: "redis_test",
		Redis: &jobtypes.RedisProtocol{
			Host: host,
			Port: port,
		},
		AliasFields: []string{"responseTime"},
	}

	// Execute Collect
	result := collector.Collect(metrics)

	// Verify result
	assert.Equal(t, consts.CollectSuccess, result.Code)
	assert.Equal(t, "success", result.Msg)
	assert.NotEmpty(t, result.Values)
	assert.Equal(t, 1, len(result.Values))

	// Verify collected values
	values := result.Values[0].Columns
	assert.Equal(t, 1, len(values))

	if values[0] == NullValue {
		t.Logf("Failed to get responseTime. Value is NullValue.")
	}

	// responseTime should be present
	assert.NotEqual(t, NullValue, values[0])
}

func TestRedisCollector_Collect_Auth(t *testing.T) {
	// Start miniredis with auth
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Could not start miniredis: %s", err)
	}
	defer s.Close()
	s.RequireAuth("password123")

	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug)
	collector := NewRedisCollector(log)

	addrParts := strings.Split(s.Addr(), ":")
	host := addrParts[0]
	port := addrParts[1]

	// Test with correct password
	metrics := &jobtypes.Metrics{
		Name: "redis_auth_test",
		Redis: &jobtypes.RedisProtocol{
			Host:     host,
			Port:     port,
			Password: "password123",
		},
		AliasFields: []string{"redis_version"},
	}

	result := collector.Collect(metrics)
	assert.Equal(t, consts.CollectSuccess, result.Code)

	// Test with incorrect password
	metrics.Redis.Password = "wrong_password"
	result = collector.Collect(metrics)
	assert.Equal(t, consts.CollectFail, result.Code)
}

func TestRedisCollector_ParseInfo(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug)
	collector := NewRedisCollector(log)

	info := `# Server
redis_version:6.2.6
redis_git_sha1:00000000
redis_git_dirty:0
redis_build_id:c6f3693d1ac7923d
redis_mode:standalone
os:Linux 5.10.76-linuxkit x86_64
arch_bits:64
multiplexing_api:epoll
atomicvar_api:atomic-builtin
gcc_version:10.2.1
process_id:1
process_supervised:no
run_id:d55354296781d79907a72527854463a2636629d6
tcp_port:6379
server_time_usec:1645518888888888
uptime_in_seconds:3600
uptime_in_days:1

# Clients
connected_clients:1
cluster_enabled:0
maxmemory:0`

	parsed := collector.parseInfo(info)

	assert.Equal(t, "6.2.6", parsed["redis_version"])
	assert.Equal(t, "standalone", parsed["redis_mode"])
	assert.Equal(t, "1", parsed["connected_clients"])
	assert.Equal(t, "0", parsed["cluster_enabled"])
	assert.Equal(t, "1", parsed["uptime_in_days"])
}
