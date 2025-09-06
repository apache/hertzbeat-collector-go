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

package main

//import (
//	"context"
//	"flag"
//	"fmt"
//	"os"
//	"os/signal"
//	"syscall"
//	"time"
//
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector"
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/common/dispatcher/entrance"
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
//	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
//	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
//)
//
//// go build -ldflags "-X main.Version=x.y.z"
//var (
//	ConfPath   string
//	Version    string
//	Simulation bool // 新增：是否启用模拟模式
//)
//
//func init() {
//	flag.StringVar(&ConfPath, "conf", "hertzbeat-collector.yaml", "path to config file")
//	flag.BoolVar(&Simulation, "simulation", true, "enable manager task simulation mode")
//}
//
//func main() {
//	flag.Parse()
//
//	// 初始化日志
//	log := logger.DefaultLogger(os.Stdout, types.LogLevelInfo)
//	log.Info("🚀 启动HertzBeat Collector", "version", Version, "simulation", Simulation)
//	Simulation = true
//	if Simulation {
//		// 模拟模式：启动完整的CollectServer并模拟Manager发送任务
//		runSimulationMode(log)
//	} else {
//		// 正常模式：启动标准的collector服务
//		if err := collector.Bootstrap(ConfPath, Version); err != nil {
//			log.Error(err, "❌ 启动collector失败")
//			os.Exit(1)
//		}
//	}
//}
//
//// runSimulationMode 运行模拟Manager发送任务的模式
//func runSimulationMode(log logger.Logger) {
//	log.Info("🎭 启动模拟模式：模拟Manager发送JDBC采集任务")
//
//	// 1. 创建CollectServer (完整的采集器架构)
//	config := entrance.DefaultServerConfig()
//	collectServer, err := entrance.NewCollectServer(config, log)
//	if err != nil {
//		log.Error(err, "❌ 创建CollectServer失败")
//		return
//	}
//
//	// 2. 启动CollectServer
//	if err := collectServer.Start(); err != nil {
//		log.Error(err, "❌ 启动CollectServer失败")
//		return
//	}
//	defer collectServer.Stop()
//
//	log.Info("✅ CollectServer已启动，准备接收Manager任务")
//
//	// 3. 创建模拟的Manager任务
//	jdbcJob := createManagerJDBCTask()
//
//	// 4. 启动任务模拟器
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	// 启动模拟Manager发送任务的goroutine
//	go simulateManagerTasks(ctx, collectServer, jdbcJob, log)
//
//	// 5. 等待中断信号
//	log.Info("🔄 模拟器运行中 (每60秒模拟Manager发送任务)，按Ctrl+C停止...")
//	sigCh := make(chan os.Signal, 1)
//	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
//	<-sigCh
//
//	log.Info("📴 收到停止信号，正在关闭...")
//	cancel()
//	time.Sleep(2 * time.Second)
//	log.Info("👋 模拟器已停止")
//}
//
//// createManagerJDBCTask 创建模拟从Manager接收的JDBC采集任务
//func createManagerJDBCTask() *jobtypes.Job {
//	return &jobtypes.Job{
//		ID:              1001,
//		TenantID:        1,
//		MonitorID:       2001,
//		App:             "mysql",
//		Category:        "database",
//		IsCyclic:        true,
//		DefaultInterval: 30, // 30秒间隔
//		Timestamp:       time.Now().Unix(),
//
//		// Manager发送的任务元数据
//		Metadata: map[string]string{
//			"instancename":       "生产MySQL实例",
//			"instancehost":       "localhost:43306",
//			"manager_node":       "hertzbeat-manager-001",
//			"task_source":        "manager_scheduler",
//			"collector_assigned": "auto",
//		},
//
//		Labels: map[string]string{
//			"env":        "production",
//			"database":   "mysql",
//			"region":     "localhost",
//			"managed_by": "hertzbeat",
//			"priority":   "high",
//		},
//
//		Annotations: map[string]string{
//			"description":   "MySQL数据库性能监控",
//			"created_by":    "manager-scheduler",
//			"task_type":     "cyclic_monitoring",
//			"alert_enabled": "true",
//		},
//
//		// JDBC采集指标配置
//		Metrics: []jobtypes.Metrics{
//			{
//				Name:     "mysql_basic_info",
//				Priority: 0,
//				Protocol: "jdbc",
//				Host:     "localhost",
//				Port:     "3306",
//				Timeout:  "15s",
//				Interval: 30,
//				Fields: []jobtypes.Field{
//					{Field: "database_name", Type: 1, Label: true},
//					{Field: "version", Type: 1, Label: false},
//					{Field: "uptime_seconds", Type: 0, Label: false},
//					{Field: "server_id", Type: 0, Label: false},
//				},
//				JDBC: &jobtypes.JDBCProtocol{
//					Host:      "localhost",
//					Port:      "3306",
//					Platform:  "mysql",
//					Username:  "root",
//					Password:  "password", // 请修改为实际密码
//					Database:  "mysql",
//					Timeout:   "15",
//					QueryType: "oneRow",
//					SQL: `SELECT
//						DATABASE() as database_name,
//						VERSION() as version,
//						(SELECT VARIABLE_VALUE FROM INFORMATION_SCHEMA.GLOBAL_STATUS WHERE VARIABLE_NAME='Uptime') as uptime_seconds,
//						@@server_id as server_id`,
//				},
//			},
//		},
//	}
//}
//
//// simulateManagerTasks 模拟Manager定期发送采集任务
//func simulateManagerTasks(ctx context.Context, collectServer *entrance.CollectServer, job *jobtypes.Job, log logger.Logger) {
//	log.Info("📡 开始模拟Manager任务调度", "jobId", job.ID, "interval", "60s")
//
//	// 创建任务响应监听器 (模拟发送结果回Manager)
//	eventListener := &ManagerResponseSimulator{
//		logger:    log,
//		jobID:     job.ID,
//		monitorID: job.MonitorID,
//	}
//
//	// 立即发送第一个任务 (模拟Manager初始调度)
//	log.Info("📤 Manager发送初始任务", "timestamp", time.Now().Format("15:04:05"))
//	if err := collectServer.ReceiveJob(job, eventListener); err != nil {
//		log.Error(err, "❌ 接收初始任务失败")
//		return
//	}
//
//	// 模拟Manager定期发送任务更新/重新调度
//	ticker := time.NewTicker(60 * time.Second) // 每60秒模拟一次Manager调度
//	defer ticker.Stop()
//
//	taskSequence := 1
//
//	for {
//		select {
//		case <-ticker.C:
//			taskSequence++
//
//			// 创建任务更新 (模拟Manager重新调度或配置更新)
//			updatedJob := *job
//			updatedJob.Timestamp = time.Now().Unix()
//			updatedJob.ID = job.ID + int64(taskSequence)
//
//			// 更新任务元数据 (模拟Manager的管理信息)
//			updatedJob.Metadata["task_sequence"] = fmt.Sprintf("%d", taskSequence)
//			updatedJob.Metadata["last_scheduled"] = time.Now().Format(time.RFC3339)
//			updatedJob.Metadata["scheduler_version"] = "v1.0.0"
//
//			log.Info("📤 Manager发送任务更新",
//				"sequence", taskSequence,
//				"jobId", updatedJob.ID,
//				"timestamp", time.Now().Format("15:04:05"))
//
//			// 发送更新任务到Collector
//			if err := collectServer.ReceiveJob(&updatedJob, eventListener); err != nil {
//				log.Error(err, "❌ 接收任务更新失败", "sequence", taskSequence)
//			}
//
//		case <-ctx.Done():
//			log.Info("🛑 停止模拟Manager任务调度")
//			return
//		}
//	}
//}
//
//// ManagerResponseSimulator 模拟Manager接收采集结果的响应处理器
//type ManagerResponseSimulator struct {
//	logger    logger.Logger
//	jobID     int64
//	monitorID int64
//}
//
//// Response 处理采集结果 (模拟发送给Manager的过程)
//func (mrs *ManagerResponseSimulator) Response(metricsData []interface{}) {
//	mrs.logger.Info("📨 Collector采集完成，准备发送结果到Manager",
//		"jobId", mrs.jobID,
//		"monitorId", mrs.monitorID,
//		"metricsCount", len(metricsData))
//
//	// 模拟处理每个采集指标的结果
//	for i, data := range metricsData {
//		if collectData, ok := data.(*jobtypes.CollectRepMetricsData); ok {
//			mrs.logger.Info("📊 处理采集指标",
//				"metricIndex", i+1,
//				"metricName", collectData.Metrics,
//				"code", collectData.Code,
//				"time", collectData.Time)
//
//			if collectData.Code == 200 {
//				mrs.logger.Info("✅ 指标采集成功",
//					"metric", collectData.Metrics,
//					"fieldsCount", len(collectData.Fields),
//					"valuesCount", len(collectData.Values))
//
//				// 打印采集数据 (模拟发送到Manager的数据)
//				for rowIndex, valueRow := range collectData.Values {
//					mrs.logger.Info("📈 采集数据",
//						"metric", collectData.Metrics,
//						"row", rowIndex+1,
//						"values", valueRow.Columns)
//				}
//			} else {
//				mrs.logger.Error(fmt.Errorf("采集失败"), "❌ 指标采集错误",
//					"metric", collectData.Metrics,
//					"code", collectData.Code,
//					"message", collectData.Msg)
//			}
//		}
//	}
//
//	// 模拟发送HTTP响应到Manager
//	mrs.simulateHTTPResponseToManager(metricsData)
//}
//
//// simulateHTTPResponseToManager 模拟通过HTTP将采集结果发送回Manager
//func (mrs *ManagerResponseSimulator) simulateHTTPResponseToManager(metricsData []interface{}) {
//	// 模拟HTTP请求参数
//	managerEndpoint := "http://hertzbeat-manager:1157/api/collector/collect"
//	collectorIdentity := "collector-go-001"
//
//	mrs.logger.Info("🚀 模拟HTTP请求发送采集结果",
//		"endpoint", managerEndpoint,
//		"collectorId", collectorIdentity,
//		"jobId", mrs.jobID,
//		"dataCount", len(metricsData),
//		"timestamp", time.Now().Format("15:04:05"))
//
//	// 这里可以添加真实的HTTP客户端代码
//	// httpClient.Post(managerEndpoint, jsonData)
//
//	mrs.logger.Info("📤 采集结果已发送到Manager (模拟成功)",
//		"status", "200 OK",
//		"responseTime", "15ms")
//}
