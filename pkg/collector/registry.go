/*
* Licensed to the Apache Software Foundation (ASF) under one or more
* contributor license agreements.  See the NOTICE file distributed with
* this work for additional information regarding copyright ownership.
* The ASF licenses this file to You under the Apache License, Version 2.0
* (the "License"); you may not use this file except in compliance with
* the License.  You may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package collector

import (
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/registry"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"

	// 导入所有采集器以触发自动注册
	_ "hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/basic/database"
)

// RegisterBuiltinCollectors 自动注册所有内置采集器
// 使用新的自动注册机制，无需手动添加每个采集器
func RegisterBuiltinCollectors(service *CollectService, logger logger.Logger) {
	// 从注册中心创建所有启用的采集器
	collectors, err := registry.GetGlobalCenter().CreateCollectors(logger)
	if err != nil {
		logger.Error(err, "创建采集器失败")
		return
	}

	// 注册到CollectService
	for protocol, collector := range collectors {
		service.RegisterCollector(protocol, collector)
		logger.Info("采集器注册到服务", "protocol", protocol)
	}
}

// RegisterBuiltinCollectorsLegacy 传统的手动注册方式 (已废弃)
// 保留用于兼容性，建议使用RegisterBuiltinCollectors
//
// Deprecated: 使用RegisterBuiltinCollectors替代，它会自动注册所有采集器
func RegisterBuiltinCollectorsLegacy(service *CollectService, logger logger.Logger) {
	logger.Info("⚠️  使用已废弃的手动注册方式，建议切换到自动注册")

	// 这里可以保留原来的手动注册逻辑用于兼容
	// jdbcCollector := database.NewJDBCCollector(logger)
	// service.RegisterCollector(jdbcCollector.SupportProtocol(), jdbcCollector)
}

// NewCollectServiceWithBuiltins creates a new collect service and registers all built-in collectors
// This is a convenience function for typical usage
func NewCollectServiceWithBuiltins(logger logger.Logger) *CollectService {
	service := NewCollectService(logger)
	RegisterBuiltinCollectors(service, logger)
	return service
}
