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
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/ssh"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/job/collect/strategy"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	protocol2 "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job/protocol"

	consts "hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	sshhelper "hertzbeat.apache.org/hertzbeat-collector-go/internal/util/ssh"
)

func init() {
	strategy.RegisterFactory(ProtocolRedis, func(logger logger.Logger) strategy.Collector {
		return NewRedisCollector(logger)
	})
}

const (
	ProtocolRedis  = "redis"
	ResponseTime   = "responseTime"
	NullValue      = consts.NULL_VALUE
	ClusterPattern = "3"
	ClusterInfo    = "cluster"
	Identity       = "identity"
)

// RedisCollector implements Redis metrics collection
type RedisCollector struct {
	logger logger.Logger
}

// NewRedisCollector creates a new Redis collector
func NewRedisCollector(logger logger.Logger) *RedisCollector {
	return &RedisCollector{
		logger: logger.WithName("redis-collector"),
	}
}

// Protocol returns the protocol this collector supports
func (rc *RedisCollector) Protocol() string {
	return ProtocolRedis
}

// PreCheck validates the Redis metrics configuration
func (rc *RedisCollector) PreCheck(metrics *jobtypes.Metrics) error {
	if metrics == nil || metrics.Redis == nil {
		return fmt.Errorf("redis configuration is required")
	}
	if metrics.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}
	if metrics.Redis.Port == "" {
		return fmt.Errorf("redis port is required")
	}
	return nil
}

// Collect performs Redis metrics collection
func (rc *RedisCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	startTime := time.Now()

	// Create response structure
	response := &jobtypes.CollectRepMetricsData{
		Metrics: metrics.Name,
		Time:    startTime.UnixMilli(),
		Code:    consts.CollectSuccess,
		Msg:     "success",
		Fields:  make([]jobtypes.Field, 0),
		Values:  make([]jobtypes.ValueRow, 0),
	}

	// Initialize fields
	for _, alias := range metrics.AliasFields {
		response.Fields = append(response.Fields, jobtypes.Field{
			Field: alias,
			Type:  1,
		})
	}

	redisConfig := metrics.Redis

	// Parse timeout
	timeout := 30 * time.Second
	if redisConfig.Timeout != "" {
		if t, err := strconv.Atoi(redisConfig.Timeout); err == nil {
			timeout = time.Duration(t) * time.Millisecond
		} else if d, err := time.ParseDuration(redisConfig.Timeout); err == nil {
			timeout = d
		}
	}

	// Create dialer (handles SSH tunnel if configured)
	dialer, err := rc.createRedisDialer(redisConfig.SSHTunnel)
	if err != nil {
		rc.logger.Error(err, "failed to create redis dialer")
		response.Code = consts.CollectUnConnectable
		response.Msg = err.Error()
		return response
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Check if cluster mode
	isCluster := redisConfig.Pattern == ClusterPattern || strings.HasPrefix(strings.ToLower(redisConfig.Pattern), ClusterInfo)

	if isCluster {
		rc.collectCluster(ctx, metrics, redisConfig, dialer, timeout, response, startTime)
	} else {
		rc.collectSingle(ctx, metrics, redisConfig, dialer, timeout, response, startTime)
	}

	return response
}

func (rc *RedisCollector) collectSingle(ctx context.Context, metrics *jobtypes.Metrics, config *protocol2.RedisProtocol, dialer func(context.Context, string, string) (net.Conn, error), timeout time.Duration, response *jobtypes.CollectRepMetricsData, startTime time.Time) {
	opts := &redis.Options{
		Addr:        fmt.Sprintf("%s:%s", config.Host, config.Port),
		Username:    config.Username,
		Password:    config.Password,
		DialTimeout: timeout,
		ReadTimeout: timeout,
	}
	if dialer != nil {
		opts.Dialer = dialer
	}

	client := redis.NewClient(opts)
	defer client.Close()

	info, err := rc.fetchRedisInfo(ctx, client, config.Pattern)
	if err != nil {
		rc.logger.Error(err, "failed to collect redis metrics")
		response.Code = consts.CollectFail
		response.Msg = err.Error()
		return
	}

	responseTime := time.Since(startTime).Milliseconds()
	parseMap := rc.parseInfo(info)
	parseMap[strings.ToLower(ResponseTime)] = fmt.Sprintf("%d", responseTime)
	parseMap[strings.ToLower(Identity)] = fmt.Sprintf("%s:%s", config.Host, config.Port)

	rc.addValueRow(response, metrics.AliasFields, parseMap)
}

func (rc *RedisCollector) collectCluster(ctx context.Context, metrics *jobtypes.Metrics, config *protocol2.RedisProtocol, dialer func(context.Context, string, string) (net.Conn, error), timeout time.Duration, response *jobtypes.CollectRepMetricsData, startTime time.Time) {
	opts := &redis.ClusterOptions{
		Addrs:       []string{fmt.Sprintf("%s:%s", config.Host, config.Port)},
		Username:    config.Username,
		Password:    config.Password,
		DialTimeout: timeout,
		ReadTimeout: timeout,
	}
	if dialer != nil {
		opts.Dialer = dialer
	}

	client := redis.NewClusterClient(opts)
	defer client.Close()

	// Collect from all masters
	var wg sync.WaitGroup
	var mu sync.Mutex

	collectNode := func(nodeClient *redis.Client) {
		defer wg.Done()
		info, err := rc.fetchRedisInfo(ctx, nodeClient, config.Pattern)
		if err != nil {
			rc.logger.Error(err, "failed to collect redis cluster node metrics", "addr", nodeClient.Options().Addr)
			return
		}

		responseTime := time.Since(startTime).Milliseconds()
		parseMap := rc.parseInfo(info)
		parseMap[strings.ToLower(ResponseTime)] = fmt.Sprintf("%d", responseTime)
		parseMap[strings.ToLower(Identity)] = nodeClient.Options().Addr

		// If collecting cluster info specifically
		if metrics.Name == ClusterInfo {
			clusterInfo, err := nodeClient.ClusterInfo(ctx).Result()
			if err == nil {
				clusterMap := rc.parseInfo(clusterInfo)
				for k, v := range clusterMap {
					parseMap[k] = v
				}
			}
		}

		mu.Lock()
		rc.addValueRow(response, metrics.AliasFields, parseMap)
		mu.Unlock()
	}

	// Iterate over masters
	err := client.ForEachMaster(ctx, func(ctx context.Context, nodeClient *redis.Client) error {
		wg.Add(1)
		go collectNode(nodeClient)
		return nil
	})
	if err != nil {
		rc.logger.Error(err, "failed to iterate masters")
	}

	// Iterate over slaves
	err = client.ForEachSlave(ctx, func(ctx context.Context, nodeClient *redis.Client) error {
		wg.Add(1)
		go collectNode(nodeClient)
		return nil
	})
	if err != nil {
		rc.logger.Error(err, "failed to iterate slaves")
	}

	wg.Wait()

	if len(response.Values) == 0 {
		response.Code = consts.CollectFail
		response.Msg = "no data collected from cluster"
	}
}

func (rc *RedisCollector) fetchRedisInfo(ctx context.Context, client *redis.Client, pattern string) (string, error) {
	if pattern != "" && strings.HasPrefix(strings.ToLower(pattern), ClusterInfo) && pattern != ClusterPattern {
		return client.ClusterInfo(ctx).Result()
	}

	section := pattern
	if section == ClusterPattern {
		section = "" // Default info for pattern "3"
	}

	if section == "" {
		return client.Info(ctx).Result()
	}
	return client.Info(ctx, section).Result()
}

func (rc *RedisCollector) addValueRow(response *jobtypes.CollectRepMetricsData, aliasFields []string, parseMap map[string]string) {
	valueRow := jobtypes.ValueRow{
		Columns: make([]string, len(aliasFields)),
	}

	for i, alias := range aliasFields {
		if val, ok := parseMap[strings.ToLower(alias)]; ok {
			valueRow.Columns[i] = val
		} else {
			valueRow.Columns[i] = NullValue
		}
	}
	response.Values = append(response.Values, valueRow)
}

// parseInfo parses Redis INFO command output into a map
func (rc *RedisCollector) parseInfo(info string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}
	return result
}

// createRedisDialer creates a dialer function that supports SSH tunneling
func (rc *RedisCollector) createRedisDialer(sshTunnel *protocol2.SSHTunnel) (func(context.Context, string, string) (net.Conn, error), error) {
	if sshTunnel == nil || sshTunnel.Enable != "true" {
		return nil, nil
	}

	// Create SSH config
	sshConfig := &protocol2.SSHProtocol{
		Host:     sshTunnel.Host,
		Port:     sshTunnel.Port,
		Username: sshTunnel.Username,
		Password: sshTunnel.Password,
	}

	// Use common/ssh helper to create client config
	clientConfig, err := sshhelper.CreateSSHClientConfig(sshConfig, rc.logger)
	if err != nil {
		return nil, err
	}
	clientConfig.Timeout = 30 * time.Second

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Connect to SSH server
		sshClient, err := sshhelper.DialWithContext(ctx, "tcp", net.JoinHostPort(sshConfig.Host, sshConfig.Port), clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to ssh server: %w", err)
		}

		// Dial target via SSH
		conn, err := sshClient.Dial(network, addr)
		if err != nil {
			sshClient.Close()
			return nil, fmt.Errorf("failed to dial target via ssh: %w", err)
		}

		return &sshConnWrapper{Conn: conn, client: sshClient}, nil
	}, nil
}

type sshConnWrapper struct {
	net.Conn
	client *ssh.Client
}

func (w *sshConnWrapper) Close() error {
	err := w.Conn.Close()
	if w.client != nil {
		w.client.Close()
	}
	return err
}
