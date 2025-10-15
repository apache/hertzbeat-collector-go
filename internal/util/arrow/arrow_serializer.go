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

package arrow

import (
	"bytes"
	"encoding/json"
	"fmt"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// ArrowSerializer handles Arrow format serialization for metrics data
type ArrowSerializer struct {
	logger logger.Logger
}

// NewArrowSerializer creates a new Arrow serializer
func NewArrowSerializer(logger logger.Logger) *ArrowSerializer {
	return &ArrowSerializer{
		logger: logger.WithName("arrow-serializer"),
	}
}

// SerializeMetricsData serializes metrics data to Arrow format
// For now, we'll use a compatible format that Manager can handle
func (as *ArrowSerializer) SerializeMetricsData(dataList []*jobtypes.CollectRepMetricsData) ([]byte, error) {
	if len(dataList) == 0 {
		return nil, fmt.Errorf("empty data list")
	}

	// For compatibility with Java Manager, we need to create a format that
	// ArrowUtil.deserializeMetricsData() can handle

	// First, let's try to create a simple Arrow-compatible format
	// Since we don't have full Arrow library, we'll create a minimal implementation

	// Convert to a format similar to what Java expects
	arrowCompatibleData := make([]map[string]interface{}, len(dataList))

	for i, data := range dataList {
		// Create Arrow-like structure
		arrowData := map[string]interface{}{
			"id":       data.ID,
			"tenantId": data.TenantID,
			"app":      data.App,
			"metrics":  data.Metrics,
			"priority": data.Priority,
			"time":     data.Time,
			"code":     data.Code,
			"msg":      data.Msg,
		}

		// Convert Values to Arrow-compatible format
		if len(data.Values) > 0 {
			// Create column-based structure like Arrow
			columns := make(map[string][]interface{})

			for _, valueRow := range data.Values {
				for i, value := range valueRow.Columns {
					if i < len(data.Fields) {
						fieldName := data.Fields[i].Field
						if columns[fieldName] == nil {
							columns[fieldName] = make([]interface{}, 0, len(data.Values))
						}
						columns[fieldName] = append(columns[fieldName], value)
					}
				}
			}

			arrowData["columns"] = columns
			arrowData["rowCount"] = len(data.Values)
		}

		arrowCompatibleData[i] = arrowData
	}

	// For now, serialize as JSON but with Arrow-like structure
	// TODO: Implement proper Arrow serialization when Arrow library is available
	jsonBytes, err := json.Marshal(arrowCompatibleData)
	if err != nil {
		as.logger.Error(err, "failed to serialize arrow-compatible data")
		return nil, fmt.Errorf("failed to serialize arrow-compatible data: %w", err)
	}

	// Create a minimal Arrow-like binary format
	// This is a temporary solution until we have proper Arrow support
	return as.createArrowLikeBinary(jsonBytes)
}

// createArrowLikeBinary creates a binary format that might be compatible with Arrow reader
func (as *ArrowSerializer) createArrowLikeBinary(jsonData []byte) ([]byte, error) {
	// Create a simple binary format that includes:
	// 1. Magic bytes (Arrow-like)
	// 2. Schema information
	// 3. Data

	var buffer bytes.Buffer

	// Write Arrow-like magic bytes
	// This is a simplified version - real Arrow has specific magic bytes
	magic := []byte("ARROW1\x00\x00")
	buffer.Write(magic)

	// Write schema length (placeholder)
	schemaLen := uint32(0) // No schema for now
	buffer.Write([]byte{byte(schemaLen), byte(schemaLen >> 8), byte(schemaLen >> 16), byte(schemaLen >> 24)})

	// Write data length
	dataLen := uint32(len(jsonData))
	buffer.Write([]byte{byte(dataLen), byte(dataLen >> 8), byte(dataLen >> 16), byte(dataLen >> 24)})

	// Write the JSON data (as a temporary measure)
	buffer.Write(jsonData)

	as.logger.V(1).Info("created arrow-like binary format",
		"originalSize", len(jsonData),
		"binarySize", buffer.Len())

	return buffer.Bytes(), nil
}

// FallbackToJSON provides JSON serialization as fallback
func (as *ArrowSerializer) FallbackToJSON(dataList []*jobtypes.CollectRepMetricsData) ([]byte, error) {
	as.logger.Info("falling back to JSON serialization", "dataCount", len(dataList))

	jsonBytes, err := json.Marshal(dataList)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize to JSON: %w", err)
	}

	return jsonBytes, nil
}
