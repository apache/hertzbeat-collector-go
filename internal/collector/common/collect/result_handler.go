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

package collect

import (
	"fmt"
	"net/http"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// ResultHandlerImpl implements the ResultHandler interface
type ResultHandlerImpl struct {
	logger logger.Logger
	// TODO: Add data queue or storage interface when needed
}

// ResultHandler interface for handling collection results
type ResultHandler interface {
	HandleCollectData(data *jobtypes.CollectRepMetricsData, job *jobtypes.Job) error
}

// NewResultHandler creates a new result handler
func NewResultHandler(logger logger.Logger) ResultHandler {
	return &ResultHandlerImpl{
		logger: logger.WithName("result-handler"),
	}
}

// HandleCollectData processes the collection results
func (rh *ResultHandlerImpl) HandleCollectData(data *jobtypes.CollectRepMetricsData, job *jobtypes.Job) error {
	if data == nil {
		rh.logger.Error(nil, "collect data is nil")
		return fmt.Errorf("collect data is nil")
	}

	// Debug level only for data handling
	rh.logger.V(1).Info("handling collect data",
		"metricsName", data.Metrics,
		"code", data.Code,
		"valuesCount", len(data.Values))

	// TODO: Implement actual data processing logic
	// This could include:
	// 1. Data validation and transformation
	// 2. Sending to message queue
	// 3. Storing to database
	// 4. Triggering alerts based on thresholds
	// 5. Updating monitoring status

	// Only log failures at INFO level, success at debug level
	if data.Code == http.StatusOK {
		rh.logger.V(1).Info("successfully processed collect data",
			"metricsName", data.Metrics)
	} else {
		rh.logger.Info("received failed collect data",
			"jobID", job.ID,
			"metricsName", data.Metrics,
			"code", data.Code,
			"message", data.Msg)
	}

	return nil
}
