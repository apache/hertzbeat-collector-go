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
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/registry"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"

	// 导入所有采集器以触发自动注册
	_ "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/basic/database"
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

// NewCollectServiceWithBuiltins creates a new collect service and registers all built-in collectors
// This is a convenience function for typical usage
func NewCollectServiceWithBuiltins(logger logger.Logger) *CollectService {
	service := NewCollectService(logger)
	RegisterBuiltinCollectors(service, logger)
	return service
}
