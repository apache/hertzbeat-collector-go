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
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"

	pb "hertzbeat.apache.org/hertzbeat-collector-go/api"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/transport"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// MessageRouter defines the interface for sending collection results
type MessageRouter interface {
	SendResult(data *jobtypes.CollectRepMetricsData, job *jobtypes.Job) error
}

// TransportRunner interface for getting transport client
type TransportRunner interface {
	GetClient() transport.TransportClient
	IsConnected() bool
}

// LazyMessageRouter is a message router that lazily obtains transport client
// Solves the startup order dependency between transport client and job runner
type LazyMessageRouter struct {
	transportRunner TransportRunner
	logger          logger.Logger
	identity        string
}

// NewLazyMessageRouter creates a new lazy message router
func NewLazyMessageRouter(transportRunner TransportRunner, logger logger.Logger, identity string) MessageRouter {
	if identity == "" {
		identity = "collector-go" // Default identity
	}

	return &LazyMessageRouter{
		transportRunner: transportRunner,
		logger:          logger.WithName("lazy-message-router"),
		identity:        identity,
	}
}

// SendResult implements MessageRouter interface
func (l *LazyMessageRouter) SendResult(data *jobtypes.CollectRepMetricsData, job *jobtypes.Job) error {
	// Get transport client
	client := l.transportRunner.GetClient()
	if client == nil || !client.IsStarted() {
		l.logger.V(1).Info("transport client not ready, dropping result",
			"jobID", job.MonitorID,
			"metricsName", data.Metrics,
			"isCyclic", job.IsCyclic)
		return fmt.Errorf("transport client not ready")
	}

	// Send result directly
	return l.sendResultDirectly(data, job, client)
}

// sendResultDirectly sends result directly to manager
func (l *LazyMessageRouter) sendResultDirectly(data *jobtypes.CollectRepMetricsData, job *jobtypes.Job, client transport.TransportClient) error {
	// Determine message type
	var msgType pb.MessageType
	if job.IsCyclic {
		msgType = pb.MessageType_RESPONSE_CYCLIC_TASK_DATA
	} else {
		msgType = pb.MessageType_RESPONSE_ONE_TIME_TASK_DATA
	}

	// Serialize data to Arrow format
	dataBytes, err := l.serializeToArrow([]*jobtypes.CollectRepMetricsData{data})
	if err != nil {
		l.logger.Error(err, "failed to create arrow format",
			"jobID", job.ID,
			"metricsName", data.Metrics)
		return fmt.Errorf("failed to create arrow format: %w", err)
	}

	// Create message - collector sends data to Manager as RESPONSE (response to task)
	msg := &pb.Message{
		Type:      msgType,
		Direction: pb.Direction_RESPONSE,
		Identity:  l.identity,
		Msg:       dataBytes,
	}

	// Send message
	if err := client.SendMsg(msg); err != nil {
		l.logger.Error(err, "failed to send metrics data",
			"jobID", job.ID,
			"metricsName", data.Metrics,
			"messageType", msgType)
		return fmt.Errorf("failed to send metrics data: %w", err)
	}

	l.logger.Info("successfully sent metrics data",
		"jobID", job.ID,
		"metricsName", data.Metrics,
		"isCyclic", job.IsCyclic,
		"messageType", msgType,
		"dataSize", len(dataBytes),
		"direction", msg.Direction,
		"identity", msg.Identity)

	// Add detailed debugging information
	l.logger.Info("message details for debugging",
		"msgType", int(msgType),
		"direction", int(msg.Direction),
		"identity", msg.Identity,
		"dataLength", len(dataBytes))

	return nil
}

// serializeToArrow serializes data using Apache Arrow format, compatible with Java Manager
func (l *LazyMessageRouter) serializeToArrow(dataList []*jobtypes.CollectRepMetricsData) ([]byte, error) {
	var mainBuf bytes.Buffer

	// Write root count (format expected by Java Manager)
	rootCount := int32(len(dataList))
	if err := binary.Write(&mainBuf, binary.BigEndian, rootCount); err != nil {
		return nil, fmt.Errorf("failed to write root count: %w", err)
	}

	mem := memory.NewGoAllocator()

	// Create separate Arrow stream for each data item
	for i, data := range dataList {
		recordBatch, err := l.createArrowRecordBatch(mem, data)
		if err != nil {
			return nil, fmt.Errorf("failed to create record batch for data %d: %w", i, err)
		}

		// Create Arrow stream
		var streamBuf bytes.Buffer
		writer := ipc.NewWriter(&streamBuf, ipc.WithSchema(recordBatch.Schema()))
		if err := writer.Write(recordBatch); err != nil {
			recordBatch.Release()
			writer.Close()
			return nil, fmt.Errorf("failed to write record batch %d: %w", i, err)
		}
		if err := writer.Close(); err != nil {
			recordBatch.Release()
			return nil, fmt.Errorf("failed to close writer for batch %d: %w", i, err)
		}
		recordBatch.Release()

		// Write stream data to main buffer
		streamData := streamBuf.Bytes()
		if _, err := mainBuf.Write(streamData); err != nil {
			return nil, fmt.Errorf("failed to write stream data for batch %d: %w", i, err)
		}
	}

	return mainBuf.Bytes(), nil
}

// createArrowRecordBatch creates Arrow RecordBatch for single MetricsData, compatible with Java Manager
func (l *LazyMessageRouter) createArrowRecordBatch(mem memory.Allocator, data *jobtypes.CollectRepMetricsData) (arrow.Record, error) {
	// Add dynamic fields (based on collected field definitions)
	dynamicFields := make([]arrow.Field, 0, len(data.Fields))
	for _, field := range data.Fields {
		// Ensure unit is not empty
		unitValue := field.Unit
		if unitValue == "" {
			unitValue = "none"
		}

		// Create field metadata
		typeValue := fmt.Sprintf("%d", field.Type)
		labelValue := fmt.Sprintf("%t", field.Label)

		fieldMetadata := arrow.MetadataFrom(map[string]string{
			"type":  typeValue,
			"label": labelValue,
			"unit":  unitValue,
		})

		dynamicFields = append(dynamicFields, arrow.Field{
			Name:     field.Field,
			Type:     arrow.BinaryTypes.String,
			Nullable: true,
			Metadata: fieldMetadata,
		})
	}

	// Merge dynamic fields only, as metadata fields are not needed in final schema
	allFields := make([]arrow.Field, 0, len(dynamicFields))
	allFields = append(allFields, dynamicFields...)

	// Create schema-level metadata
	schemaMetadata := arrow.MetadataFrom(map[string]string{
		"id":          fmt.Sprintf("%d", data.ID),
		"tenantId":    fmt.Sprintf("%d", data.TenantID),
		"app":         data.App,
		"metrics":     data.Metrics,
		"priority":    fmt.Sprintf("%d", data.Priority),
		"time":        fmt.Sprintf("%d", data.Time),
		"labels":      "",
		"annotations": "",
	})

	schema := arrow.NewSchema(allFields, &schemaMetadata)

	// Create builders (all fields are String type)
	builders := make([]array.Builder, len(allFields))
	for i, field := range allFields {
		builders[i] = array.NewBuilder(mem, field.Type)
	}
	defer func() {
		for _, builder := range builders {
			builder.Release()
		}
	}()

	// Determine row count
	rowCount := len(data.Values)
	if rowCount == 0 {
		rowCount = 1 // At least one row of metadata
	}

	// Fill data
	for rowIdx := 0; rowIdx < rowCount; rowIdx++ {
		// Fill dynamic field data
		if rowIdx < len(data.Values) {
			valueRow := data.Values[rowIdx]
			for i := range data.Fields {
				var value string
				if i < len(valueRow.Columns) {
					value = valueRow.Columns[i]
				}
				builders[i].(*array.StringBuilder).Append(value)
			}
		} else {
			// Fill empty values
			for i := range data.Fields {
				builders[i].(*array.StringBuilder).Append("")
			}
		}
	}

	// Build arrays
	arrays := make([]arrow.Array, len(builders))
	for i, builder := range builders {
		arrays[i] = builder.NewArray()
		defer arrays[i].Release()
	}

	// Create Record
	record := array.NewRecord(schema, arrays, int64(rowCount))
	return record, nil
}

// createUnifiedArrowRecordBatch creates a unified RecordBatch containing all MetricsData
func (l *LazyMessageRouter) createUnifiedArrowRecordBatch(mem memory.Allocator, dataList []*jobtypes.CollectRepMetricsData) (arrow.Record, error) {
	if len(dataList) == 0 {
		return nil, fmt.Errorf("empty data list")
	}

	// Use the first data item to define schema
	firstData := dataList[0]

	// Define basic fields - simplified schema, only core fields
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "monitorId", Type: arrow.PrimitiveTypes.Int64},
		{Name: "app", Type: arrow.BinaryTypes.String},
		{Name: "metrics", Type: arrow.BinaryTypes.String},
		{Name: "time", Type: arrow.PrimitiveTypes.Int64},
		{Name: "code", Type: arrow.PrimitiveTypes.Int32},
		{Name: "msg", Type: arrow.BinaryTypes.String},
	}

	// Add value fields - based on first data item's Fields
	if len(firstData.Fields) > 0 {
		for _, field := range firstData.Fields {
			fields = append(fields, arrow.Field{
				Name: field.Field,
				Type: arrow.BinaryTypes.String,
			})
		}
	}

	schema := arrow.NewSchema(fields, nil)

	// Create builders
	builders := make([]array.Builder, len(fields))
	for i, field := range fields {
		switch field.Type {
		case arrow.PrimitiveTypes.Int64:
			builders[i] = array.NewInt64Builder(mem)
		case arrow.PrimitiveTypes.Int32:
			builders[i] = array.NewInt32Builder(mem)
		default:
			builders[i] = array.NewStringBuilder(mem)
		}
	}

	// Calculate total rows
	totalRows := 0
	for _, data := range dataList {
		if len(data.Values) > 0 {
			totalRows += len(data.Values)
		} else {
			totalRows += 1 // At least one row of metadata
		}
	}

	// Fill data
	for _, data := range dataList {
		rowCount := 1
		if len(data.Values) > 0 {
			rowCount = len(data.Values)
		}

		for row := 0; row < rowCount; row++ {
			// Basic fields
			builders[0].(*array.Int64Builder).Append(data.ID)
			builders[1].(*array.Int64Builder).Append(data.MonitorID)
			builders[2].(*array.StringBuilder).Append(data.App)
			builders[3].(*array.StringBuilder).Append(data.Metrics)
			builders[4].(*array.Int64Builder).Append(data.Time)
			builders[5].(*array.Int32Builder).Append(int32(data.Code))
			builders[6].(*array.StringBuilder).Append(data.Msg)

			// Value fields
			if len(data.Values) > row && len(data.Fields) > 0 {
				valueRow := data.Values[row]
				for i := range data.Fields {
					builderIndex := 7 + i // Index after basic fields
					if builderIndex < len(builders) {
						if i < len(valueRow.Columns) {
							builders[builderIndex].(*array.StringBuilder).Append(valueRow.Columns[i])
						} else {
							builders[builderIndex].(*array.StringBuilder).Append("")
						}
					}
				}
			} else {
				// Fill empty values
				for i := 0; i < len(firstData.Fields); i++ {
					builderIndex := 7 + i
					if builderIndex < len(builders) {
						builders[builderIndex].(*array.StringBuilder).Append("")
					}
				}
			}
		}
	}

	// Build arrays
	arrays := make([]arrow.Array, len(builders))
	for i, builder := range builders {
		arrays[i] = builder.NewArray()
		defer arrays[i].Release()
		builder.Release()
	}

	// Create RecordBatch
	record := array.NewRecord(schema, arrays, int64(totalRows))
	return record, nil
}
