# Transport Module Documentation

## 概述

传输模块为 HertzBeat 采集器提供了全面的通信层，支持与 Java 版本管理服务器的无缝集成。该模块同时支持 gRPC 和 Netty 协议，利用 Go 的并发特性实现高性能通信。

## 主要特性

### 1. **完整的消息类型支持**
- HEARTBEAT (0) - 心跳消息
- GO_ONLINE (1) - 采集器上线通知
- GO_OFFLINE (2) - 采集器下线通知
- GO_CLOSE (3) - 采集器关闭通知
- ISSUE_CYCLIC_TASK (4) - 周期性任务分配
- DELETE_CYCLIC_TASK (5) - 周期性任务删除
- ISSUE_ONE_TIME_TASK (6) - 一次性任务分配
- RESPONSE_CYCLIC_TASK_DATA (7) - 周期性任务响应
- RESPONSE_ONE_TIME_TASK_DATA (8) - 一次性任务响应
- RESPONSE_CYCLIC_TASK_SD_DATA (9) - 周期性任务服务发现响应

### 2. **事件驱动架构**
- 连接事件（已连接、断开连接、连接失败）
- 自定义事件处理器
- 连接丢失时自动重连

### 3. **消息处理**
- 异步消息处理
- 同步请求-响应模式
- 按类型注册消息处理器
- 所有消息类型的默认处理器

### 4. **连接管理**
- 自动连接监控
- 心跳机制（10秒间隔）
- 优雅关闭处理
- 连接状态跟踪

## 架构

### 核心组件

1. **TransportClient** - 传输客户端接口
2. **ProcessorRegistry** - 管理消息处理器
3. **ResponseFuture** - 处理同步响应
4. **Event System** - 连接事件处理
5. **Message Processors** - 类型特定的消息处理器

### 消息流

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Collector     │    │   Transport     │    │   Manager       │
│                 │    │                 │    │                 │
│  Application    │───▶│ TransportClient │───▶│  Server         │
│                 │    │                 │    │                 │
│                 │◀───│                 │◀───│                 │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 使用方法

### 基本使用

```go
import (
    "hertzbeat.apache.org/hertzbeat-collector-go/internal/transport"
    pb "hertzbeat.apache.org/hertzbeat-collector-go/api/cluster_msg"
)

// 创建 Netty 客户端
factory := &transport.TransportClientFactory{}
client, err := factory.CreateClient("netty", "127.0.0.1:1158")
if err != nil {
    log.Fatal("创建客户端失败：", err)
}

// 设置事件处理器
client.SetEventHandler(func(event transport.Event) {
    switch event.Type {
    case transport.EventConnected:
        log.Println("已连接到服务器")
    case transport.EventDisconnected:
        log.Println("与服务器断开连接")
    }
})

// 注册处理器
client.RegisterProcessor(int32(pb.MessageType_HEARTBEAT), func(msg interface{}) (interface{}, error) {
    if pbMsg, ok := msg.(*pb.Message); ok {
        log.Printf("收到心跳消息： %s", string(pbMsg.Msg))
        return &pb.Message{
            Type:      pb.MessageType_HEARTBEAT,
            Direction: pb.Direction_RESPONSE,
            Identity:  pbMsg.Identity,
            Msg:       []byte("heartbeat response"),
        }, nil
    }
    return nil, nil
})

// 启动客户端
if err := client.Start(); err != nil {
    log.Fatal("启动客户端失败：", err)
}

// 发送消息
msg := &pb.Message{
    Type:      pb.MessageType_HEARTBEAT,
    Direction: pb.Direction_REQUEST,
    Identity:  "collector-1",
    Msg:       []byte("heartbeat"),
}

// 异步发送
err := client.SendMsg(msg)

// 同步发送
resp, err := client.SendMsgSync(msg, 5000)
```

### 与采集器集成

传输模块通过 `transport.Runner` 集成到采集器中：

```go
config := &transport.Config{
    Server: clrServer.Server{
        Logger: logger,
    },
    ServerAddr: "127.0.0.1:1158",
    Protocol:   "netty",
}

runner := transport.New(config)
if err := runner.Start(ctx); err != nil {
    log.Fatal("启动传输失败：", err)
}
```

### 配置

#### 环境变量
- `MANAGER_HOST`: 管理服务器主机（默认：127.0.0.1）
- `MANAGER_PORT`: 管理服务器端口（默认：1158）
- `MANAGER_PROTOCOL`: 通信协议（默认：netty）

#### 配置结构
```go
type Config struct {
    clrServer.Server
    ServerAddr string // 服务器地址
    Protocol   string // 协议类型 (netty/grpc)
}
```

## 消息类型和处理器

### 内置处理器

1. **HeartbeatProcessor** - 处理心跳消息
2. **GoOnlineProcessor** - 处理采集器上线通知
3. **GoOfflineProcessor** - 处理采集器下线通知
4. **GoCloseProcessor** - 处理采集器关闭请求
5. **CollectCyclicDataProcessor** - 处理周期性任务分配
6. **DeleteCyclicTaskProcessor** - 处理周期性任务删除
7. **CollectOneTimeDataProcessor** - 处理一次性任务分配

### 自定义处理器

```go
// 注册自定义处理器
client.RegisterProcessor(100, func(msg interface{}) (interface{}, error) {
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
    return nil, fmt.Errorf("无效的消息类型")
})
```

## 错误处理

传输模块提供了全面的错误处理：

1. **连接错误** - 自动重连和事件通知
2. **消息发送错误** - 从 SendMsg/SendMsgSync 方法返回
3. **处理错误** - 由各个处理器处理
4. **超时错误** - 基于上下文的超时处理

## 性能考虑

1. **连接池** - 带监控的单连接
2. **并发处理** - 基于 goroutine 的消息处理
3. **心跳优化** - 可配置的心跳间隔
4. **内存管理** - 资源的适当清理

## 测试

运行测试：
```bash
go test ./internal/transport/...
```

## 与 Java 版本比较

| 特性 | Java 版本 | Go 版本 |
|------|----------|---------|
| 传输协议 | Netty | Netty + gRPC |
| 消息类型 | 完整 | 完整 |
| 事件处理 | NettyEventListener | EventHandler |
| 响应处理 | ResponseFuture | ResponseFuture |
| 处理器 | NettyRemotingProcessor | ProcessorFunc |
| 连接管理 | Netty | 自定义实现 |
| 心跳 | 内置 | 内置 |
| 自动重连 | 是 | 是 |

## 未来增强

1. **连接池** - 支持多连接
2. **负载均衡** - 支持多个管理服务器
3. **指标收集** - 传输性能的内置指标
4. **断路器** - 容错模式
5. **TLS 支持** - 安全通信
6. **消息压缩** - 优化的数据传输

## 故障排除

### 常见问题

1. **连接被拒绝**
   - 验证管理服务器正在运行
   - 检查地址和端口配置
   - 验证网络连通性

2. **消息处理错误**
   - 检查消息格式和内容
   - 验证处理器注册
   - 检查处理器实现

3. **性能问题**
   - 监控连接状态
   - 检查心跳间隔
   - 检查消息处理逻辑

### 调试日志

通过设置日志级别启用调试日志：
```go
logger, _ := zap.NewDevelopment()
```

## 贡献

为传输模块贡献时：

1. 遵循 Go 编码标准
2. 添加全面的测试
3. 更新文档
4. 确保向后兼容性
5. 与采集器和管理器组件一起测试

---

# Transport Module Documentation

## Overview

The transport module provides a comprehensive communication layer for the HertzBeat collector, enabling seamless communication with the Java version manager server. The module supports both gRPC and Netty protocols, leveraging Go's concurrency features for high-performance communication.

## Key Features

### 1. **Complete Message Type Support**
- HEARTBEAT (0) - Heartbeat messages
- GO_ONLINE (1) - Collector online notification
- GO_OFFLINE (2) - Collector offline notification
- GO_CLOSE (3) - Collector shutdown notification
- ISSUE_CYCLIC_TASK (4) - Cyclic task assignment
- DELETE_CYCLIC_TASK (5) - Cyclic task deletion
- ISSUE_ONE_TIME_TASK (6) - One-time task assignment
- RESPONSE_CYCLIC_TASK_DATA (7) - Cyclic task response
- RESPONSE_ONE_TIME_TASK_DATA (8) - One-time task response
- RESPONSE_CYCLIC_TASK_SD_DATA (9) - Cyclic task service discovery response

### 2. **Event-Driven Architecture**
- Connection events (Connected, Disconnected, Connect Failed)
- Custom event handlers
- Automatic reconnection on connection loss

### 3. **Message Processing**
- Asynchronous message processing
- Synchronous request-response pattern
- Message processor registration by type
- Default processors for all message types

### 4. **Connection Management**
- Automatic connection monitoring
- Heartbeat mechanism (10-second intervals)
- Graceful shutdown handling
- Connection state tracking

## Architecture

### Core Components

1. **TransportClient** - Transport client interface
2. **ProcessorRegistry** - Manages message processors
3. **ResponseFuture** - Handles synchronous responses
4. **Event System** - Connection event handling
5. **Message Processors** - Type-specific message handlers

### Message Flow

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Collector     │    │   Transport     │    │   Manager       │
│                 │    │                 │    │                 │
│  Application    │───▶│ TransportClient │───▶│  Server         │
│                 │    │                 │    │                 │
│                 │◀───│                 │◀───│                 │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Usage

### Basic Usage

```go
import (
    "hertzbeat.apache.org/hertzbeat-collector-go/internal/transport"
    pb "hertzbeat.apache.org/hertzbeat-collector-go/api/cluster_msg"
)

// Create Netty client
factory := &transport.TransportClientFactory{}
client, err := factory.CreateClient("netty", "127.0.0.1:1158")
if err != nil {
    log.Fatal("Failed to create client:", err)
}

// Set event handler
client.SetEventHandler(func(event transport.Event) {
    switch event.Type {
    case transport.EventConnected:
        log.Println("Connected to server")
    case transport.EventDisconnected:
        log.Println("Disconnected from server")
    }
})

// Register processors
client.RegisterProcessor(int32(pb.MessageType_HEARTBEAT), func(msg interface{}) (interface{}, error) {
    if pbMsg, ok := msg.(*pb.Message); ok {
        log.Printf("Received heartbeat message: %s", string(pbMsg.Msg))
        return &pb.Message{
            Type:      pb.MessageType_HEARTBEAT,
            Direction: pb.Direction_RESPONSE,
            Identity:  pbMsg.Identity,
            Msg:       []byte("heartbeat response"),
        }, nil
    }
    return nil, nil
})

// Start client
if err := client.Start(); err != nil {
    log.Fatal("Failed to start client:", err)
}

// Send message
msg := &pb.Message{
    Type:      pb.MessageType_HEARTBEAT,
    Direction: pb.Direction_REQUEST,
    Identity:  "collector-1",
    Msg:       []byte("heartbeat"),
}

// Async send
err := client.SendMsg(msg)

// Sync send
resp, err := client.SendMsgSync(msg, 5000)
```

### Integration with Collector

The transport module is integrated into the collector through the `transport.Runner`:

```go
config := &transport.Config{
    Server: clrServer.Server{
        Logger: logger,
    },
    ServerAddr: "127.0.0.1:1158",
    Protocol:   "netty",
}

runner := transport.New(config)
if err := runner.Start(ctx); err != nil {
    log.Fatal("Failed to start transport:", err)
}
```

### Configuration

#### Environment Variables
- `MANAGER_HOST`: Manager server host (default: 127.0.0.1)
- `MANAGER_PORT`: Manager server port (default: 1158)
- `MANAGER_PROTOCOL`: Communication protocol (default: netty)

#### Configuration Structure
```go
type Config struct {
    clrServer.Server
    ServerAddr string // Server address
    Protocol   string // Protocol type (netty/grpc)
}
```

## Message Types and Processors

### Built-in Processors

1. **HeartbeatProcessor** - Handles heartbeat messages
2. **GoOnlineProcessor** - Handles collector online notifications
3. **GoOfflineProcessor** - Handles collector offline notifications
4. **GoCloseProcessor** - Handles collector shutdown requests
5. **CollectCyclicDataProcessor** - Handles cyclic task assignments
6. **DeleteCyclicTaskProcessor** - Handles cyclic task deletions
7. **CollectOneTimeDataProcessor** - Handles one-time task assignments

### Custom Processors

```go
// Register custom processor
client.RegisterProcessor(100, func(msg interface{}) (interface{}, error) {
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
    return nil, fmt.Errorf("invalid message type")
})
```

## Error Handling

The transport module provides comprehensive error handling:

1. **Connection Errors** - Automatic reconnection with event notifications
2. **Message Send Errors** - Returned from SendMsg/SendMsgSync methods
3. **Processing Errors** - Handled by individual processors
4. **Timeout Errors** - Context-based timeout handling

## Performance Considerations

1. **Connection Pooling** - Single connection with monitoring
2. **Concurrent Processing** - Goroutine-based message processing
3. **Heartbeat Optimization** - Configurable heartbeat intervals
4. **Memory Management** - Proper cleanup of resources

## Testing

Run tests with:
```bash
go test ./internal/transport/...
```

## Comparison with Java Version

| Feature | Java Version | Go Version |
|---------|-------------|------------|
| Transport Protocol | Netty | Netty + gRPC |
| Message Types | Complete | Complete |
| Event Handling | NettyEventListener | EventHandler |
| Response Handling | ResponseFuture | ResponseFuture |
| Processors | NettyRemotingProcessor | ProcessorFunc |
| Connection Management | Netty | Custom implementation |
| Heartbeat | Built-in | Built-in |
| Auto-reconnect | Yes | Yes |

## Future Enhancements

1. **Connection Pooling** - Support for multiple connections
2. **Load Balancing** - Support for multiple manager servers
3. **Metrics Collection** - Built-in metrics for transport performance
4. **Circuit Breaker** - Fault tolerance patterns
5. **TLS Support** - Secure communication
6. **Message Compression** - Optimized data transfer

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Verify manager server is running
   - Check address and port configuration
   - Verify network connectivity

2. **Message Processing Errors**
   - Check message format and content
   - Verify processor registration
   - Review processor implementation

3. **Performance Issues**
   - Monitor connection state
   - Check heartbeat intervals
   - Review message processing logic

### Debug Logging

Enable debug logging by setting the log level:
```go
logger, _ := zap.NewDevelopment()
```

## Contributing

When contributing to the transport module:

1. Follow Go coding standards
2. Add comprehensive tests
3. Update documentation
4. Ensure backward compatibility
5. Test with both collector and manager components