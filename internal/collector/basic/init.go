/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package basic

import (
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/basic/database"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/collect/strategy"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// InitializeAllCollectors 初始化所有已注册的采集器
// 当logger可用时调用此函数来创建实际的采集器实例
func InitializeAllCollectors(logger logger.Logger) {
	logger.Info("initializing all collectors")

	// 直接注册所有采集器实例
	// 新增协议时在这里添加
	jdbcCollector := database.NewJDBCCollector(logger)
	strategy.Register(jdbcCollector)

	// 未来可以在这里添加更多协议：
	// httpCollector := http.NewHTTPCollector(logger)
	// strategy.Register(httpCollector)

	// 记录初始化结果
	protocols := strategy.SupportedProtocols()
	logger.Info("collectors initialized successfully",
		"protocols", protocols,
		"count", len(protocols))
}
