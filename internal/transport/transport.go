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
	"time"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
	clrserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/server"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/types/collector"
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
	client       TransportClient
	jobScheduler JobScheduler
	cfg          clrserver.Server
	tlog         logger.Logger
}

func New(cfg *clrserver.Server) *Runner {

	return &Runner{
		cfg: *cfg,
		// init logger
		tlog: cfg.Logger.WithName("transport"),
	}
}

// SetJobScheduler sets the job scheduler for the transport runner
func (r *Runner) SetJobScheduler(scheduler JobScheduler) {
	r.jobScheduler = scheduler
}

func (r *Runner) Start(ctx context.Context) error {

	r.tlog.Info("Starting transport client")

	// 构建 server 地址
	addr := fmt.Sprintf("%s:%s", r.cfg.Config.Collector.Manager.Host, r.cfg.Config.Collector.Manager.Port)
	if addr == ":" {
		// 如果配置为空，使用环境变量或默认值
		if v := os.Getenv("MANAGER_ADDR"); v != "" {
			addr = v
		} else {
			addr = DefaultManagerAddr
		}
	}

	// 确定协议
	protocol := r.cfg.Config.Collector.Manager.Protocol
	if protocol == "" {
		if v := os.Getenv("MANAGER_PROTOCOL"); v != "" {
			protocol = v
		} else {
			protocol = DefaultProtocol
		}
	}

	r.tlog.Info("Connecting to manager server", "addr", addr, "protocol", protocol)

	// 创建客户端
	factory := &TransportClientFactory{}
	client, err := factory.CreateClient(protocol, addr)
	if err != nil {
		r.tlog.Error(err, "Failed to create transport client, will retry in background")
		// 不直接返回错误，而是继续启动，后续会在后台重试连接
	} else {
		// Set the identity on the client if it supports it
		identity := r.cfg.Config.Collector.Identity
		if identity == "" {
			identity = DefaultIdentity
		}

		if nettyClient, ok := client.(*NettyClient); ok {
			nettyClient.SetIdentity(identity)
		}

		r.client = client

		// 设置事件处理器
		switch c := client.(type) {
		case *GrpcClient:
			c.SetEventHandler(func(event Event) {
				switch event.Type {
				case EventConnected:
					r.tlog.Info("Connected to manager gRPC server", "addr", event.Address)
					go r.sendOnlineMessage()
				case EventDisconnected:
					r.tlog.Info("Disconnected from manager gRPC server", "addr", event.Address)
				case EventConnectFailed:
					r.tlog.Error(event.Error, "Failed to connect to manager gRPC server, will retry", "addr", event.Address)
				}
			})
			// Register processors with job scheduler
			if r.jobScheduler != nil {
				RegisterDefaultProcessors(c, r.jobScheduler)
				r.tlog.Info("Registered gRPC processors with job scheduler")
			} else {
				RegisterDefaultProcessors(c, nil)
				r.tlog.Info("Registered gRPC processors without job scheduler")
			}
		case *NettyClient:
			c.SetEventHandler(func(event Event) {
				switch event.Type {
				case EventConnected:
					r.tlog.Info("Connected to manager netty server", "addr", event.Address)
					go r.sendOnlineMessage()
				case EventDisconnected:
					r.tlog.Info("Disconnected from manager netty server", "addr", event.Address)
				case EventConnectFailed:
					r.tlog.Error(event.Error, "Failed to connect to manager netty server, will retry", "addr", event.Address)
				}
			})
			// Register processors with job scheduler
			if r.jobScheduler != nil {
				RegisterDefaultNettyProcessors(c, r.jobScheduler)
				r.tlog.Info("Registered netty processors with job scheduler")
			} else {
				RegisterDefaultNettyProcessors(c, nil)
				r.tlog.Info("Registered netty processors without job scheduler")
			}
		}

		// 尝试启动客户端，如果失败则启动重连循环
		if err := r.client.Start(); err != nil {
			r.tlog.Error(err, "Failed to start transport client on first attempt, starting retry loop")

			// 在后台启动重连循环
			go func() {
				attempt := 0
				for {
					attempt++
					r.tlog.Info("Reconnection attempt", "attempt", attempt, "wait", "3s")
					time.Sleep(3 * time.Second)

					// 检查是否应该停止
					select {
					case <-ctx.Done():
						r.tlog.Info("Reconnection loop stopped due to context cancellation")
						return
					default:
					}

					r.tlog.Info("Trying to connect to manager", "attempt", attempt, "addr", addr)
					if err := r.client.Start(); err == nil {
						r.tlog.Info("Successfully connected to manager", "attempt", attempt)
						return
					} else {
						r.tlog.Error(err, "Connection failed, will retry", "attempt", attempt)
					}
				}
			}()
		}
	}

	r.tlog.Info("Transport runner started successfully, connection will be established in background")

	// 创建新的context用于监控关闭信号
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 监听 ctx.Done 优雅关闭
	go func() {
		<-ctx.Done()
		r.tlog.Info("Shutting down transport client...")
		if r.client != nil {
			_ = r.client.Shutdown()
		}
	}()

	// 阻塞直到 ctx.Done
	<-ctx.Done()
	return nil
}

func (r *Runner) sendOnlineMessage() {
	if r.client != nil && r.client.IsStarted() {
		// Use the configured identity
		identity := r.cfg.Config.Collector.Identity
		if identity == "" {
			identity = DefaultIdentity
		}

		// Create CollectorInfo JSON structure as expected by Java server
		mode := r.cfg.Config.Collector.Mode
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
			r.tlog.Error(err, "Failed to marshal collector info to JSON")
			return
		}

		onlineMsg := &pb.Message{
			Type:      pb.MessageType_GO_ONLINE,
			Direction: pb.Direction_REQUEST,
			Identity:  identity,
			Msg:       jsonData,
		}

		r.tlog.Info("Sending online message", "identity", identity, "type", onlineMsg.Type)

		if err := r.client.SendMsg(onlineMsg); err != nil {
			r.tlog.Error(err, "Failed to send online message", "identity", identity)
		} else {
			r.tlog.Info("Online message sent successfully", "identity", identity)
		}
	}
}

func (r *Runner) Info() collector.Info {
	return collector.Info{
		Name: "transport",
	}
}

func (r *Runner) Close() error {

	r.tlog.Info("transport close...")
	if r.client != nil {
		_ = r.client.Shutdown()
	}

	return nil
}

// GetClient returns the transport client (for testing and advanced usage)
func (r *Runner) GetClient() TransportClient {

	return r.client
}

// IsConnected returns whether the client is connected and started
func (r *Runner) IsConnected() bool {

	return r.client != nil && r.client.IsStarted()
}
