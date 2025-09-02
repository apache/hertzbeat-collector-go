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
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/basic/database"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
)

// RegisterBuiltinCollectors registers all built-in collectors with the service
// This function should be called during application initialization to avoid circular dependencies
func RegisterBuiltinCollectors(service *CollectService, logger logger.Logger) {
	// Register JDBC collector
	jdbcCollector := database.NewJDBCCollector(logger)
	service.RegisterCollector(jdbcCollector.SupportProtocol(), jdbcCollector)

	// TODO: Register other built-in collectors here
	// httpCollector := http.NewHTTPCollector(logger)
	// service.RegisterCollector(httpCollector.SupportProtocol(), httpCollector)

	// sshCollector := ssh.NewSSHCollector(logger)
	// service.RegisterCollector(sshCollector.SupportProtocol(), sshCollector)
}

// NewCollectServiceWithBuiltins creates a new collect service and registers all built-in collectors
// This is a convenience function for typical usage
func NewCollectServiceWithBuiltins(logger logger.Logger) *CollectService {
	service := NewCollectService(logger)
	RegisterBuiltinCollectors(service, logger)
	return service
}
