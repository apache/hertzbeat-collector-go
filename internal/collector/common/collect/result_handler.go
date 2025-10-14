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
	logger        logger.Logger
	messageRouter MessageRouter // Message router for sending results back to Manager
}

// ResultHandler interface for handling collection results
type ResultHandler interface {
	HandleCollectData(data *jobtypes.CollectRepMetricsData, job *jobtypes.Job) error
}

// NewResultHandler creates a new result handler
func NewResultHandler(logger logger.Logger, messageRouter MessageRouter) ResultHandler {
	return &ResultHandlerImpl{
		logger:        logger.WithName("result-handler"),
		messageRouter: messageRouter,
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

	// Send collection result back to Manager via MessageRouter
	if rh.messageRouter != nil {
		if err := rh.messageRouter.SendResult(data, job); err != nil {
			rh.logger.Error(err, "failed to send collection result to Manager",
				"jobID", job.ID,
				"metricsName", data.Metrics)
			return fmt.Errorf("failed to send result to Manager: %w", err)
		}

		// Log successful result sending
		if data.Code == http.StatusOK {
			rh.logger.V(1).Info("successfully sent collection result to Manager",
				"metricsName", data.Metrics)
		} else {
			rh.logger.Info("sent failed collection result to Manager",
				"jobID", job.ID,
				"metricsName", data.Metrics,
				"code", data.Code,
				"message", data.Msg)
		}
	} else {
		rh.logger.Error(nil, "messageRouter is nil, cannot send result to Manager",
			"jobID", job.ID,
			"metricsName", data.Metrics)
		return fmt.Errorf("messageRouter is nil")
	}

	return nil
}
