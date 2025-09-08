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

// init 函数在包被导入时自动执行
// 集中注册所有协议的工厂函数
func init() {
	// 注册所有协议的工厂函数
	// 新增协议时只需要在这里添加一行
	strategy.RegisterFactory("jdbc", func(logger logger.Logger) strategy.Collector {
		return database.NewJDBCCollector(logger)
	})

	// 未来可以在这里添加更多协议：
	// strategy.RegisterFactory("http", func(logger logger.Logger) strategy.Collector {
	//     return http.NewHTTPCollector(logger)
	// })
	// strategy.RegisterFactory("redis", func(logger logger.Logger) strategy.Collector {
	//     return redis.NewRedisCollector(logger)
	// })
}

// InitializeAllCollectors 初始化所有已注册的采集器
// 此时init()函数已经注册了所有工厂函数
// 这个函数创建实际的采集器实例
func InitializeAllCollectors(logger logger.Logger) {
	logger.Info("initializing all collectors")

	// 使用已注册的工厂函数创建采集器实例
	strategy.InitializeCollectors(logger)

	logger.Info("all collectors initialized successfully")
}
