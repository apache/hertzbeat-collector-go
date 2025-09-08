# HertzBeat Collector Go

[![License](https://img.shields.io/badge/license-Apache%202-blue)](LICENSE)

HertzBeat-Collector-Go 是 [Apache HertzBeat](https://github.com/apache/hertzbeat) 的 Go 语言采集器实现，支持多协议、多类型的监控数据采集，具备高性能、易扩展、易集成等特性。

## ✨ 特性

- 支持多种协议（如 HTTP、JDBC、SNMP、SSH 等）采集监控数据
- 任务调度、作业管理、采集策略灵活可扩展
- 结构清晰，易于二次开发和集成
- 丰富的开发、测试和部署脚本
- 完善的文档和社区支持

## 📂 目录结构

```text
.
├── cmd/                # 主程序入口
├── internal/           # 采集器核心实现与通用组件
│   ├── collector/      # 各类采集器
│   ├── common/         # 公共模块（调度、作业、类型、日志等）
│   └── util/           # 工具类
├── api/                # 协议定义（protobuf）
├── examples/           # 示例代码
├── docs/               # 架构与开发文档
├── tools/              # 构建、CI、脚本等工具
├── Makefile            # 构建入口
├── Dockerfile          # 容器化部署
└── README.md           # 项目说明
```

## 🚀 快速开始

### 1. 构建与运行

```bash
# 安装依赖
go mod tidy

# 构建
make build

# 运行
./bin/collector server --config etc/hertzbeat-collector.yaml
```

### 2. 示例

可参考 `examples/main_simulation.go` 进行本地模拟采集测试。

## 🛠️ 贡献

欢迎通过 [CONTRIBUTING.md](CONTRIBUTING.md) 参与贡献，包括代码、文档、测试、讨论等。

## 📄 许可证

本项目基于 [Apache 2.0 License](LICENSE) 开源。
