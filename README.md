# HertzBeat Collector Go

[![License](https://img.shields.io/badge/license-Apache%202-blue)](LICENSE)

HertzBeat-Collector-Go is the Go implementation of the collector for [Apache HertzBeat](https://github.com/apache/hertzbeat). It supports multi-protocol and multi-type monitoring data collection, featuring high performance, easy extensibility, and seamless integration.

## ✨ Features

- Supports various protocols (HTTP, JDBC, SNMP, SSH, etc.) for monitoring data collection
- Flexible and extensible task scheduling, job management, and collection strategies
- Clean architecture, easy for secondary development and integration
- Rich development, testing, and deployment scripts
- Comprehensive documentation and community support

## 📂 Project Structure

```text
.
├── cmd/                # Main entry point
├── internal/           # Core collector implementation and common modules
│   ├── collector/      # Various collectors
│   ├── common/         # Common modules (scheduling, jobs, types, logging, etc.)
│   └── util/           # Utilities
├── api/                # Protocol definitions (protobuf)
├── examples/           # Example code
├── docs/               # Architecture and development docs
├── tools/              # Build, CI, scripts, and tools
├── Makefile            # Build entry
├── Dockerfile          # Containerization
└── README.md           # Project description
```

## 🚀 Quick Start

### 1. Build and Run

```bash
# Install dependencies
go mod tidy

# Build
make build

# Run
./bin/collector server --config etc/hertzbeat-collector.yaml
```

### 2. Example

See `examples/main_simulation.go` for a local simulation test.

## 🛠️ Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details, including code, documentation, tests, and discussions.

## 📄 License

This project is licensed under the [Apache 2.0 License](LICENSE).
