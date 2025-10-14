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

package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
	clrserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/collector"
	configtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/config"
	loggertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	config "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/config"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/transport"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	// DefaultManagerAddr is the default manager server address (Java Netty default port)
	DefaultManagerAddr = "127.0.0.1:1158"
	// DefaultProtocol is the default communication protocol for Java compatibility
	DefaultProtocol = "netty"
	// DefaultMode is the default operation mode
	DefaultMode = "public"
	// DefaultIdentity is the default collector identity
	DefaultIdentity = "collector-go"
)

type Runner struct {
	Config       *configtypes.CollectorConfig
	client       transport.TransportClient
	jobScheduler transport.JobScheduler
	clrserver.Server
}

func New(cfg *configtypes.CollectorConfig) *Runner {
	return &Runner{
		Config: cfg,
		Server: clrserver.Server{
			Logger: logger.Logger{}, // Will be initialized properly in Start method
		},
	}
}

// SetJobScheduler sets the job scheduler for the transport runner
func (r *Runner) SetJobScheduler(scheduler transport.JobScheduler) {
	r.jobScheduler = scheduler
}

// NewFromConfig creates a new transport runner from collector configuration
func NewFromConfig(cfg *configtypes.CollectorConfig) *Runner {
	if cfg == nil {
		return nil
	}
	return New(cfg)
}

// NewFromEnv creates a new transport runner from environment variables
func NewFromEnv() *Runner {
	envLoader := config.NewEnvConfigLoader()
	cfg := envLoader.LoadFromEnv()
	return NewFromConfig(cfg)
}

// NewFromUnifiedConfig creates a new transport runner using unified configuration loading
// It loads from file first, then overrides with environment variables
func NewFromUnifiedConfig(cfgPath string) (*Runner, error) {
	unifiedLoader := config.NewUnifiedConfigLoader(cfgPath)
	cfg, err := unifiedLoader.Load()
	if err != nil {
		return nil, err
	}
	return NewFromConfig(cfg), nil
}

func (r *Runner) Start(ctx context.Context) error {
	// 初始化 Logger 如果它还没有被设置
	if r.Logger.IsZero() {
		r.Logger = logger.DefaultLogger(os.Stdout, loggertypes.LogLevelInfo)
	}
	r.Logger = r.Logger.WithName(r.Info().Name).WithValues("runner", r.Info().Name)
	r.Logger.Info("Starting transport client")

	// 构建 server 地址
	addr := fmt.Sprintf("%s:%s", r.Config.Collector.Manager.Host, r.Config.Collector.Manager.Port)
	if addr == ":" {
		// 如果配置为空，使用环境变量或默认值
		if v := os.Getenv("MANAGER_ADDR"); v != "" {
			addr = v
		} else {
			addr = DefaultManagerAddr
		}
	}

	// 确定协议
	protocol := r.Config.Collector.Manager.Protocol
	if protocol == "" {
		if v := os.Getenv("MANAGER_PROTOCOL"); v != "" {
			protocol = v
		} else {
			protocol = DefaultProtocol
		}
	}

	r.Logger.Info("Connecting to manager server", "addr", addr, "protocol", protocol)

	// 创建客户端
	factory := &transport.TransportClientFactory{}
	client, err := factory.CreateClient(protocol, addr)
	if err != nil {
		r.Logger.Error(err, "Failed to create transport client")
		return err
	}

	// Set the identity on the client if it supports it
	identity := r.Config.Collector.Identity
	if identity == "" {
		identity = DefaultIdentity
	}

	if nettyClient, ok := client.(*transport.NettyClient); ok {
		nettyClient.SetIdentity(identity)
	}

	r.client = client

	// 设置事件处理器
	switch c := client.(type) {
	case *transport.GrpcClient:
		c.SetEventHandler(func(event transport.Event) {
			switch event.Type {
			case transport.EventConnected:
				r.Logger.Info("Connected to manager gRPC server", "addr", event.Address)
				go r.sendOnlineMessage()
			case transport.EventDisconnected:
				r.Logger.Info("Disconnected from manager gRPC server", "addr", event.Address)
			case transport.EventConnectFailed:
				r.Logger.Error(event.Error, "Failed to connect to manager gRPC server", "addr", event.Address)
			}
		})
		// Register processors with job scheduler
		if r.jobScheduler != nil {
			transport.RegisterDefaultProcessors(c, r.jobScheduler)
			r.Logger.Info("Registered gRPC processors with job scheduler")
		} else {
			transport.RegisterDefaultProcessors(c, nil)
			r.Logger.Info("Registered gRPC processors without job scheduler")
		}
	case *transport.NettyClient:
		c.SetEventHandler(func(event transport.Event) {
			switch event.Type {
			case transport.EventConnected:
				r.Logger.Info("Connected to manager netty server", "addr", event.Address)
				go r.sendOnlineMessage()
			case transport.EventDisconnected:
				r.Logger.Info("Disconnected from manager netty server", "addr", event.Address)
			case transport.EventConnectFailed:
				r.Logger.Error(event.Error, "Failed to connect to manager netty server", "addr", event.Address)
			}
		})
		// Register processors with job scheduler
		if r.jobScheduler != nil {
			transport.RegisterDefaultNettyProcessors(c, r.jobScheduler)
			r.Logger.Info("Registered netty processors with job scheduler")
		} else {
			transport.RegisterDefaultNettyProcessors(c, nil)
			r.Logger.Info("Registered netty processors without job scheduler")
		}
	}

	if err := r.client.Start(); err != nil {
		r.Logger.Error(err, "Failed to start transport client")
		return err
	}

	// 创建新的context用于监控关闭信号
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 监听 ctx.Done 优雅关闭
	go func() {
		<-ctx.Done()
		r.Logger.Info("Shutting down transport client...")
		_ = r.client.Shutdown()
	}()

	// 阻塞直到 ctx.Done
	<-ctx.Done()
	return nil
}

func (r *Runner) sendOnlineMessage() {
	if r.client != nil && r.client.IsStarted() {
		// Use the configured identity
		identity := r.Config.Collector.Identity
		if identity == "" {
			identity = DefaultIdentity
		}

		// Create CollectorInfo JSON structure as expected by Java server
		mode := r.Config.Collector.Mode
		if mode == "" {
			mode = DefaultMode // Default mode as in Java version
		}

		collectorInfo := map[string]interface{}{
			"name":    identity,
			"ip":      "", // Let server detect IP
			"version": "1.0.0",
			"mode":    mode,
		}

		// Convert to JSON bytes
		jsonData, err := json.Marshal(collectorInfo)
		if err != nil {
			r.Logger.Error(err, "Failed to marshal collector info to JSON")
			return
		}

		onlineMsg := &pb.Message{
			Type:      pb.MessageType_GO_ONLINE,
			Direction: pb.Direction_REQUEST,
			Identity:  identity,
			Msg:       jsonData,
		}

		r.Logger.Info("Sending online message", "identity", identity, "type", onlineMsg.Type)

		if err := r.client.SendMsg(onlineMsg); err != nil {
			r.Logger.Error(err, "Failed to send online message", "identity", identity)
		} else {
			r.Logger.Info("Online message sent successfully", "identity", identity)
		}
	}
}

func (r *Runner) Info() collector.Info {
	return collector.Info{
		Name: "transport",
	}
}

func (r *Runner) Close() error {
	r.Logger.Info("transport close...")
	if r.client != nil {
		_ = r.client.Shutdown()
	}
	return nil
}

// GetClient returns the transport client (for testing and advanced usage)
func (r *Runner) GetClient() transport.TransportClient {
	return r.client
}

// IsConnected returns whether the client is connected and started
func (r *Runner) IsConnected() bool {
	return r.client != nil && r.client.IsStarted()
}
