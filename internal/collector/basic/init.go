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

// init function is automatically executed when the package is imported
// It centrally registers factory functions for all protocols
func init() {
	// Register factory functions for all protocols
	// To add a new protocol, simply add a line here
	strategy.RegisterFactory("jdbc", func(logger logger.Logger) strategy.Collector {
		return database.NewJDBCCollector(logger)
	})

	// More protocols can be added here in the future:
	// strategy.RegisterFactory("http", func(logger logger.Logger) strategy.Collector {
	//     return http.NewHTTPCollector(logger)
	// })
	// strategy.RegisterFactory("redis", func(logger logger.Logger) strategy.Collector {
	//     return redis.NewRedisCollector(logger)
	// })
}

// InitializeAllCollectors initializes all registered collectors
// At this point, the init() function has already registered all factory functions
// This function creates the actual collector instances
func InitializeAllCollectors(logger logger.Logger) {
	logger.Info("initializing all collectors")

	// Create collector instances using registered factory functions
	strategy.InitializeCollectors(logger)

	logger.Info("all collectors initialized successfully")
}
