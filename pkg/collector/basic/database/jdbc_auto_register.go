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

package database

import (
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/basic"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/registry"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

// init 函数会在包被导入时自动执行，完成JDBC采集器的自动注册
func init() {
	// 注册JDBC采集器工厂函数到全局注册中心
	registry.RegisterCollectorFactory(
		"jdbc", // 协议名称
		func(logger logger.Logger) basic.AbstractCollector {
			return NewJDBCCollector(logger)
		},
		registry.WithPriority(10), // 高优先级，数据库采集很重要
	)
}
