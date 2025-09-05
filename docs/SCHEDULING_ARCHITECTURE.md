# HertzBeat Go Collector 调度架构设计文档

## 📋 概述

本文档详细说明了HertzBeat Go版本Collector的调度架构设计，包括与Java版本的对比分析、核心组件说明、调度流程详解等内容。

## 🏗️ 整体架构对比

### Java版本架构

```
Manager调度器 → 一致性哈希 → 网络通信 → Collector
    ↓
TimerDispatcher → WheelTimerTask → CommonDispatcher
    ↓
MetricsCollectorQueue → WorkerPool → MetricsCollect
    ↓
CollectStrategyFactory(SPI) → 具体采集器 → 返回结果
```

### Go版本架构

```
Manager调度器 → 网络通信(待完善) → CollectServer
    ↓
TimerDispatcher → WheelTimerTask → DispatchMetricsTask
    ↓
WorkerPool → MetricsCollect → CollectService
    ↓
CollectorRegistry(手动注册) → 具体采集器 → 返回结果
```

## 🔄 调度流程详细对比

### 1. 任务接收阶段

#### Java版本

```java
// Manager发送任务
ClusterMsg.Message message = ClusterMsg.Message.newBuilder()
    .setType(ClusterMsg.MessageType.ISSUE_CYCLIC_TASK)
    .setDirection(ClusterMsg.Direction.REQUEST)
    .setMsg(ByteString.copyFromUtf8(JsonUtil.toJson(job)))
    .build();
manageServer.sendMsg(node.getIdentity(), message);

// Collector接收任务
@Override
public void response(ClusterMsg.Message message) {
    Job job = JsonUtil.fromJson(message.getMsg().toStringUtf8(), Job.class);
    collectJobService.addAsyncCollectJob(job);
}
```

#### Go版本

```go
// Manager发送任务 (目前为模拟方式)
func (cs *CollectServer) ReceiveJob(job *jobtypes.Job, eventListener timer.CollectResponseEventListener) error {
    if !cs.isStarted {
        return fmt.Errorf("collect server is not started")
    }
    
    // 直接添加到调度器
    return cs.timerDispatcher.AddJob(job, eventListener)
}
```

**主要差异**:

- Java: 完整的网络通信协议栈
- Go: 网络层待完善，目前使用模拟接口

### 2. 任务调度阶段

#### Java版本

```java
// TimerDispatcher调度
@Override
public void addJob(Job addJob, CollectResponseEventListener eventListener) {
    WheelTimerTask timerJob = new WheelTimerTask(addJob);
    if (addJob.isCyclic()) {
        Timeout timeout = wheelTimer.newTimeout(timerJob, addJob.getInterval(), TimeUnit.SECONDS);
        currentCyclicTaskMap.put(addJob.getId(), timeout);
    }
}

// WheelTimerTask执行
@Override
public void run(Timeout timeout) throws Exception {
    // 分发到CommonDispatcher
    metricsTaskDispatch.dispatchMetricsTask(this);
}
```

#### Go版本

```go
// TimerDispatcher调度
func (td *TimerDispatcher) AddJob(job *jobtypes.Job, eventListener CollectResponseEventListener) error {
    timerTask := NewWheelTimerTask(job, td.metricsDispatcher, td.logger)
    
    var delay time.Duration
    if job.DefaultInterval > 0 {
        delay = time.Duration(job.DefaultInterval) * time.Second
    }
    
    timeout := td.wheelTimer.NewTimeout(timerTask, delay)
    
    if job.IsCyclic {
        td.cyclicTasks.Store(job.ID, timeout)
    } else {
        td.tempTasks.Store(job.ID, timeout)
    }
}

// DispatchMetricsTask执行
func (td *TimerDispatcher) DispatchMetricsTask(timeout *jobtypes.Timeout) error {
    if task, ok := timeout.Task().(*WheelTimerTask); ok {
        job := task.GetJob()
        
        // 为每个metric创建采集任务
        for _, metric := range job.Metrics {
            metricsCollect := worker.NewMetricsCollect(
                &metric, timeout, td.collectDispatcher,
                "collector-go", td.collectService, td.logger,
            )
            td.workerPool.Submit(metricsCollect)
        }
    }
}
```

**主要差异**:

- Java: 使用CommonDispatcher作为中间层
- Go: TimerDispatcher直接分发任务到WorkerPool

### 3. 任务分发阶段

#### Java版本

```java
// CommonDispatcher处理
public void run() {
    while (!Thread.currentThread().isInterrupted()) {
        MetricsCollect metricsCollect = jobRequestQueue.getJob();
        if (metricsCollect != null) {
            workerPool.executeJob(metricsCollect);
        }
    }
}

// 使用优先级队列
private final MetricsCollectorQueue jobRequestQueue;
```

#### Go版本

```go
// WorkerPool直接处理
func (wp *WorkerPool) Submit(task Task) error {
    select {
    case wp.taskQueue <- task:
        atomic.AddInt64(&wp.stats.QueuedTasks, 1)
        return nil
    default:
        return fmt.Errorf("task queue is full")
    }
}

// Worker执行任务
func (w *worker) executeTask(task Task) {
    timeout := task.Timeout()
    ctx, cancel := context.WithTimeout(w.ctx, timeout)
    defer cancel()
    
    done := make(chan error, 1)
    go func() {
        done <- task.Execute()
    }()
    
    select {
    case err := <-done:
        // 任务完成
    case <-ctx.Done():
        // 任务超时
    }
}
```

**主要差异**:

- Java: 专门的优先级队列 + 分发线程
- Go: 直接的通道队列 + Goroutine池

### 4. 采集器调用阶段

#### Java版本 (SPI自动发现)

```java
// CollectStrategyFactory使用SPI
@Override
public void run(String... args) throws Exception {
    ServiceLoader<AbstractCollect> loader = ServiceLoader.load(AbstractCollect.class);
    for (AbstractCollect collect : loader) {
        COLLECT_STRATEGY.put(collect.supportProtocol(), collect);
    }
}

// META-INF/services/org.apache.hertzbeat.collector.collect.AbstractCollect
org.apache.hertzbeat.collector.collect.database.JdbcCommonCollect
org.apache.hertzbeat.collector.collect.http.HttpCollectImpl
// ... 其他采集器
```

#### Go版本 (手动注册 + 新的自动注册机制)

```go
// 传统手动注册方式
func RegisterBuiltinCollectors(service *CollectService, logger logger.Logger) {
    jdbcCollector := database.NewJDBCCollector(logger)
    service.RegisterCollector(jdbcCollector.SupportProtocol(), jdbcCollector)
}

// 新的自动注册机制
// internal/collector/basic/database/jdbc_auto_register.go
func init() {
    registry.RegisterCollectorFactory(
        "jdbc",
        func(logger logger.Logger) basic.AbstractCollector {
            return NewJDBCCollector(logger)
        },
        registry.WithPriority(10),
    )
}
```

**主要差异**:

- Java: 使用Java SPI机制自动发现
- Go: 使用init()函数 + 注册中心的自动注册机制

## 🔧 核心组件详解

### 1. TimerDispatcher (时间轮调度器)

#### 共同特性

- 都使用HashedWheelTimer实现
- 支持循环任务和一次性任务
- 任务超时管理

#### 差异对比

| 特性 | Java版本 | Go版本 |
|------|---------|--------|
| 超时监控 | 独立的ScheduledExecutor | 集成在TimerDispatcher中 |
| 任务存储 | ConcurrentHashMap | sync.Map |
| 重试机制 | 基于优先级队列 | 指数退避策略 |
| 事件监听 | EventListener接口 | CollectResponseEventListener |

### 2. WorkerPool (工作线程池)

#### Java版本特性

```java
// 基于ThreadPoolExecutor
private ThreadPoolExecutor workerExecutor;

// 动态线程池配置
int coreSize = Math.max(2, Runtime.getRuntime().availableProcessors());
int maxSize = Runtime.getRuntime().availableProcessors() * 16;
workerExecutor = new ThreadPoolExecutor(coreSize, maxSize, 10, TimeUnit.SECONDS,
    new SynchronousQueue<>(), threadFactory, new ThreadPoolExecutor.AbortPolicy());
```

#### Go版本特性

```go
// 基于Goroutine池
type WorkerPool struct {
    config     WorkerPoolConfig
    taskQueue  chan Task
    workers    sync.Map
    ctx        context.Context
    cancel     context.CancelFunc
}

// 动态Goroutine管理
func (wp *WorkerPool) adjustWorkerCount() {
    queuedTasks := atomic.LoadInt64(&wp.stats.QueuedTasks)
    currentWorkers := wp.workerCount.Load()
    
    if queuedTasks > int64(currentWorkers)*2 && currentWorkers < int32(wp.config.MaxSize) {
        wp.addWorker(false) // 添加非核心worker
    }
}
```

#### 差异对比

| 特性 | Java版本 | Go版本 |
|------|---------|--------|
| 并发模型 | 线程池 | Goroutine池 |
| 队列类型 | SynchronousQueue | 带缓冲的Channel |
| 拒绝策略 | AbortPolicy | 阻塞等待 |
| 资源管理 | JVM线程管理 | Go运行时管理 |

### 3. 采集器注册机制

#### Java版本 (SPI机制)

```java
// 优点：
- 自动发现，无需手动注册
- 标准Java机制，成熟稳定
- 支持插件化扩展

// 缺点：
- 启动时全量加载
- 难以动态控制
- 依赖类路径配置
```

#### Go版本 (注册中心机制)

```go
// 优点：
- 支持优先级控制
- 运行时动态启用/禁用
- 避免循环依赖
- 更好的错误处理

// 缺点：
- 需要手动导入包
- 相对复杂的注册流程
```

## 📊 性能对比分析

### 1. 内存使用

#### Java版本

- **优势**: JVM成熟的内存管理
- **劣势**: 较大的基础内存占用
- **特点**: GC压力，但有成熟的调优工具

#### Go版本

- **优势**: 更小的内存占用
- **劣势**: 需要手动管理某些资源
- **特点**: GC延迟低，内存效率高

### 2. 并发性能

#### Java版本

- **线程开销**: 每个线程约2MB栈空间
- **上下文切换**: 相对较重
- **适用场景**: CPU密集型任务

#### Go版本

- **Goroutine开销**: 每个约2KB栈空间
- **上下文切换**: 非常轻量
- **适用场景**: IO密集型任务

### 3. 启动速度

#### Java版本

- **JVM启动**: 相对较慢
- **类加载**: SPI机制需要扫描classpath
- **预热时间**: JIT编译需要时间

#### Go版本

- **编译启动**: 非常快速
- **静态链接**: 无需额外依赖
- **即时性能**: 无需预热

## 🔮 发展路线图

### 短期目标 (已完成)

- ✅ 基础调度框架
- ✅ JDBC采集器实现
- ✅ 自动注册机制
- ✅ 超时监控和重试

### 中期目标 (进行中)

- 🔄 完善网络通信层
- 🔄 实现更多协议采集器 (HTTP, SSH, SNMP等)
- 🔄 集群模式支持
- 🔄 配置文件驱动的采集器管理

### 长期目标 (规划中)

- 📋 插件化架构
- 📋 热更新能力
- 📋 云原生部署支持
- 📋 AI驱动的智能调度

## 🛠️ 开发指南

### 1. 添加新的采集器

```go
// 步骤1: 实现采集器接口
type RedisCollector struct {
    logger logger.Logger
}

func (rc *RedisCollector) SupportProtocol() string {
    return "redis"
}

func (rc *RedisCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
    // 实现采集逻辑
}

// 步骤2: 添加自动注册
func init() {
    registry.RegisterCollectorFactory("redis", 
        func(logger logger.Logger) basic.AbstractCollector {
            return NewRedisCollector(logger)
        },
        registry.WithPriority(15),
    )
}

// 步骤3: 在registry.go中添加导入
_ "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/basic/redis"
```

### 2. 调试和监控

```go
// 查看调度器状态
stats := timerDispatcher.GetStats()
logger.Info("调度器状态", 
    "cyclicJobs", stats.CyclicJobs,
    "tempJobs", stats.TempJobs,
    "executedJobs", stats.ExecutedJobs)

// 查看工作池状态
poolStats, queueStats := workerPool.GetStats()
logger.Info("工作池状态",
    "activeWorkers", poolStats.ActiveWorkers,
    "queuedTasks", poolStats.QueuedTasks)
```

### 3. 性能优化建议

#### 调度器优化

```go
// 1. 合理设置时间轮参数
timerDispatcher := timer.NewTimerDispatcher(logger)
timerDispatcher.SetWheelSize(512)  // 根据任务数量调整
timerDispatcher.SetTickDuration(100 * time.Millisecond)

// 2. 批量处理任务
func (td *TimerDispatcher) BatchDispatch(jobs []*jobtypes.Job) error {
    for _, job := range jobs {
        td.AddJob(job, nil)
    }
}
```

#### 工作池优化

```go
// 1. 根据任务特性调整池大小
config := worker.WorkerPoolConfig{
    CoreSize:    runtime.NumCPU(),           // CPU密集型
    MaxSize:     runtime.NumCPU() * 4,      // IO密集型可以更大
    QueueSize:   1000,                      // 根据内存情况调整
    IdleTimeout: 60 * time.Second,
}

// 2. 任务优先级处理
type PriorityTask struct {
    Priority int
    Task     Task
}
```

## 📝 最佳实践

### 1. 错误处理

```go
// 采集器中的错误处理
func (jc *JDBCCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
    defer func() {
        if r := recover(); r != nil {
            jc.logger.Error(fmt.Errorf("panic in JDBC collect: %v", r), "采集器panic")
        }
    }()
    
    // 具体采集逻辑...
}
```

### 2. 资源管理

```go
// 连接池管理
type JDBCCollector struct {
    connectionPool map[string]*sql.DB
    poolMutex      sync.RWMutex
}

func (jc *JDBCCollector) getConnection(dsn string) (*sql.DB, error) {
    jc.poolMutex.RLock()
    if conn, exists := jc.connectionPool[dsn]; exists {
        jc.poolMutex.RUnlock()
        return conn, nil
    }
    jc.poolMutex.RUnlock()
    
    // 创建新连接...
}
```

### 3. 配置管理

```go
// 支持配置文件和环境变量
type CollectorConfig struct {
    JDBC struct {
        MaxConnections    int           `yaml:"max_connections" env:"JDBC_MAX_CONNECTIONS"`
        ConnectionTimeout time.Duration `yaml:"connection_timeout" env:"JDBC_CONNECTION_TIMEOUT"`
        QueryTimeout      time.Duration `yaml:"query_timeout" env:"JDBC_QUERY_TIMEOUT"`
    } `yaml:"jdbc"`
}
```

## 🎯 总结

HertzBeat Go版本的调度架构在保持与Java版本核心理念一致的同时，充分利用了Go语言的特性优势：

### 核心优势

1. **高并发性能**: Goroutine模型适合IO密集的采集任务
2. **低资源占用**: 更小的内存占用和更快的启动速度
3. **简洁架构**: 减少了中间层，调度链路更直接
4. **类型安全**: 编译时类型检查，减少运行时错误

### 主要差异

1. **网络层**: 待完善，目前使用模拟接口
2. **采集器注册**: 从SPI改为init()函数 + 注册中心
3. **并发模型**: 从线程池改为Goroutine池
4. **错误处理**: 更显式的错误处理机制

### 发展方向

Go版本将在保持高性能和低资源占用优势的基础上，逐步完善网络通信、集群支持等企业级特性，最终实现与Java版本功能对等但性能更优的目标。

---

*文档版本: v1.0*  
*最后更新: 2024年1月*  
*维护者: HertzBeat Go Team*
