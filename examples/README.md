# HertzBeat Collector Go - Examples

这个目录包含了HertzBeat Collector Go的使用示例。

## 快速开始

### 1. 直接运行

```bash
# 设置环境变量
export IDENTITY=hertzbeat-collector-go
export MODE=public
export MANAGER_HOST=127.0.0.1
export MANAGER_PORT=1158
export MANAGER_PROTOCOL=netty

# 运行collector
go run examples/main.go
```

### 2. 使用Docker

```bash
# 构建Docker镜像
docker build -t hertzbeat-collector-go:latest examples/

# 运行容器
docker run -d \
  -e IDENTITY=hertzbeat-collector-go \
  -e MODE=public \
  -e MANAGER_HOST=host.docker.internal \
  -e MANAGER_PORT=1158 \
  -e MANAGER_PROTOCOL=netty \
  --name hertzbeat-collector-go \
  hertzbeat-collector-go:latest
```

### 3. 使用Docker Compose

```bash
# 启动服务
docker-compose -f examples/docker-compose.yml up -d

# 查看日志
docker-compose -f examples/docker-compose.yml logs -f
```

## 环境变量配置

| 变量名 | 描述 | 默认值 | 必需 |
|--------|------|--------|------|
| `IDENTITY` | 采集器标识符 | - | 是 |
| `MODE` | 运行模式 (public/private) | public | 否 |
| `MANAGER_HOST` | 管理服务器主机 | 127.0.0.1 | 否 |
| `MANAGER_PORT` | 管理服务器端口 | 1158 | 否 |
| `MANAGER_PROTOCOL` | 通信协议 (netty/grpc) | netty | 否 |
| `MANAGER_TIMEOUT` | 连接超时时间（毫秒） | 5000 | 否 |
| `MANAGER_HEARTBEAT_INTERVAL` | 心跳间隔（秒） | 10 | 否 |

## 配置文件

可以使用 `examples/hertzbeat-collector.yaml` 文件进行配置，或使用环境变量覆盖配置。

## 功能特性

- ✅ 支持Netty和gRPC双协议
- ✅ 自动重连机制
- ✅ 心跳检测
- ✅ 优雅关闭
- ✅ 完整的错误处理
- ✅ Docker容器化支持
- ✅ 环境变量配置
- ✅ 信号处理

## 支持的消息类型

- `HEARTBEAT` - 心跳消息
- `GO_ONLINE` - 上线消息
- `GO_OFFLINE` - 下线消息
- `GO_CLOSE` - 关闭消息
- `ISSUE_CYCLIC_TASK` - 周期性任务
- `ISSUE_ONE_TIME_TASK` - 一次性任务

## 文件结构

```text
examples/
├── main.go              # 主要示例文件
├── Dockerfile           # Docker构建文件
├── docker-compose.yml   # Docker Compose配置
├── hertzbeat-collector.yaml  # 配置文件示例
└── README.md           # 本文档
```

## 故障排除

如果遇到连接问题，请检查：

1. 确保管理服务器正在运行
2. 检查网络连通性
3. 验证端口是否开放
4. 确认协议版本兼容性
5. 检查日志输出

### 常见问题

**连接被拒绝**
```bash
# 检查管理服务器状态
telnet $MANAGER_HOST $MANAGER_PORT

# 检查防火墙设置
sudo ufw status
```

**心跳超时**
```bash
# 增加心跳间隔
export MANAGER_HEARTBEAT_INTERVAL=30

# 检查网络延迟
ping $MANAGER_HOST
```

## 开发指南

### 本地开发

```bash
# 克隆项目
git clone https://github.com/apache/hertzbeat-collector-go.git
cd hertzbeat-collector-go

# 安装依赖
go mod tidy

# 运行示例
go run examples/main.go
```

### 自定义配置

```go
// 在代码中直接配置
config := &transport.Config{
    Server: clrServer.Server{
        Logger: logger,
    },
    ServerAddr: "custom-host:1158",
    Protocol:   "netty",
}

runner := transport.New(config)
```

### 添加自定义消息处理器

```go
// 注册自定义处理器
runner.RegisterProcessor(100, func(msg interface{}) (interface{}, error) {
    if pbMsg, ok := msg.(*pb.Message); ok {
        // 处理消息
        log.Printf("收到自定义消息： %s", string(pbMsg.Msg))
        return &pb.Message{
            Type:      pb.MessageType_HEARTBEAT,
            Direction: pb.Direction_RESPONSE,
            Identity:  pbMsg.Identity,
            Msg:       []byte("custom response"),
        }, nil
    }
    return nil, nil
})
```

更多详细信息请参考主项目的README.md文件。

---

# HertzBeat Collector Go - Examples

This directory contains usage examples for HertzBeat Collector Go.

## Quick Start

### 1. Direct Run

```bash
# Set environment variables
export IDENTITY=hertzbeat-collector-go
export MODE=public
export MANAGER_HOST=127.0.0.1
export MANAGER_PORT=1158
export MANAGER_PROTOCOL=netty

# Run collector
go run examples/main.go
```

### 2. Using Docker

```bash
# Build Docker image
docker build -t hertzbeat-collector-go:latest examples/

# Run container
docker run -d \
  -e IDENTITY=hertzbeat-collector-go \
  -e MODE=public \
  -e MANAGER_HOST=host.docker.internal \
  -e MANAGER_PORT=1158 \
  -e MANAGER_PROTOCOL=netty \
  --name hertzbeat-collector-go \
  hertzbeat-collector-go:latest
```

### 3. Using Docker Compose

```bash
# Start services
docker-compose -f examples/docker-compose.yml up -d

# View logs
docker-compose -f examples/docker-compose.yml logs -f
```

## Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `IDENTITY` | Collector identifier | - | Yes |
| `MODE` | Operation mode (public/private) | public | No |
| `MANAGER_HOST` | Manager server host | 127.0.0.1 | No |
| `MANAGER_PORT` | Manager server port | 1158 | No |
| `MANAGER_PROTOCOL` | Communication protocol (netty/grpc) | netty | No |
| `MANAGER_TIMEOUT` | Connection timeout (milliseconds) | 5000 | No |
| `MANAGER_HEARTBEAT_INTERVAL` | Heartbeat interval (seconds) | 10 | No |

## Configuration File

You can use the `examples/hertzbeat-collector.yaml` file for configuration, or override configuration with environment variables.

## Features

- ✅ Dual protocol support (Netty and gRPC)
- ✅ Auto-reconnection mechanism
- ✅ Heartbeat detection
- ✅ Graceful shutdown
- ✅ Complete error handling
- ✅ Docker containerization support
- ✅ Environment variable configuration
- ✅ Signal handling

## Supported Message Types

- `HEARTBEAT` - Heartbeat message
- `GO_ONLINE` - Online message
- `GO_OFFLINE` - Offline message
- `GO_CLOSE` - Close message
- `ISSUE_CYCLIC_TASK` - Cyclic task
- `ISSUE_ONE_TIME_TASK` - One-time task

## File Structure

```text
examples/
├── main.go              # Main example file
├── Dockerfile           # Docker build file
├── docker-compose.yml   # Docker Compose configuration
├── hertzbeat-collector.yaml  # Configuration file example
└── README.md           # This document
```

## Troubleshooting

If you encounter connection issues, please check:

1. Ensure the manager server is running
2. Check network connectivity
3. Verify the port is open
4. Confirm protocol version compatibility
5. Check log output

### Common Issues

**Connection Refused**
```bash
# Check manager server status
telnet $MANAGER_HOST $MANAGER_PORT

# Check firewall settings
sudo ufw status
```

**Heartbeat Timeout**
```bash
# Increase heartbeat interval
export MANAGER_HEARTBEAT_INTERVAL=30

# Check network latency
ping $MANAGER_HOST
```

## Development Guide

### Local Development

```bash
# Clone project
git clone https://github.com/apache/hertzbeat-collector-go.git
cd hertzbeat-collector-go

# Install dependencies
go mod tidy

# Run example
go run examples/main.go
```

### Custom Configuration

```go
// Configure directly in code
config := &transport.Config{
    Server: clrServer.Server{
        Logger: logger,
    },
    ServerAddr: "custom-host:1158",
    Protocol:   "netty",
}

runner := transport.New(config)
```

### Add Custom Message Processor

```go
// Register custom processor
runner.RegisterProcessor(100, func(msg interface{}) (interface{}, error) {
    if pbMsg, ok := msg.(*pb.Message); ok {
        // Process message
        log.Printf("Received custom message: %s", string(pbMsg.Msg))
        return &pb.Message{
            Type:      pb.MessageType_HEARTBEAT,
            Direction: pb.Direction_RESPONSE,
            Identity:  pbMsg.Identity,
            Msg:       []byte("custom response"),
        }, nil
    }
    return nil, nil
})
```

For more detailed information, please refer to the main project's README.md file.