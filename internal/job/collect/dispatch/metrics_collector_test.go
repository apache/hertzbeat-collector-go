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

package dispatch

import (
	"testing"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

func TestCalculateFieldsWithUnitConversion(t *testing.T) {
	// Create a logger for testing
	log := logger.DefaultLogger(nil, loggertype.LogLevelInfo)

	// Create metrics collector
	mc := NewMetricsCollector(log)

	tests := []struct {
		name           string
		metrics        *jobtypes.Metrics
		collectResult  *jobtypes.CollectRepMetricsData
		expectedValues [][]string
	}{
		{
			name: "byte to megabyte conversion",
			metrics: &jobtypes.Metrics{
				Name:     "memory",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
					{Field: "free", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"total", "used", "free"},
				Units:       []string{"total=B->MB", "used=B->MB", "free=B->MB"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1048576", "524288", "524288"}}, // 1MB, 0.5MB, 0.5MB in bytes
				},
			},
			expectedValues: [][]string{
				{"1", "0.5", "0.5"}, // Converted to MB
			},
		},
		{
			name: "mixed unit conversion",
			metrics: &jobtypes.Metrics{
				Name:     "disk",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "size", Type: constants.TYPE_NUMBER},
					{Field: "usage", Type: constants.TYPE_STRING},
				},
				AliasFields: []string{"size", "usage"},
				Units:       []string{"size=KB->GB"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1073741824", "50%"}}, // 1GB in KB, 50%
				},
			},
			expectedValues: [][]string{
				{"1024", "50%"}, // size converted to GB, usage unchanged
			},
		},
		{
			name: "with unit in value string",
			metrics: &jobtypes.Metrics{
				Name:     "memory",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "committed", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"committed"},
				Units:       []string{"committed=B->MB"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"2097152B"}}, // 2MB in bytes with unit
				},
			},
			expectedValues: [][]string{
				{"2"}, // Converted to MB
			},
		},
		{
			name: "no unit conversion",
			metrics: &jobtypes.Metrics{
				Name:     "cpu",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "usage", Type: constants.TYPE_NUMBER},
					{Field: "cores", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"usage", "cores"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"75.5", "4"}},
				},
			},
			expectedValues: [][]string{
				{"75.5", "4"}, // No conversion
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute CalculateFields
			mc.CalculateFields(tt.metrics, tt.collectResult)

			// Verify results
			if len(tt.collectResult.Values) != len(tt.expectedValues) {
				t.Errorf("CalculateFields() resulted in %d rows, expected %d",
					len(tt.collectResult.Values), len(tt.expectedValues))
				return
			}

			for i, row := range tt.collectResult.Values {
				expectedRow := tt.expectedValues[i]
				if len(row.Columns) != len(expectedRow) {
					t.Errorf("Row %d has %d columns, expected %d",
						i, len(row.Columns), len(expectedRow))
					continue
				}

				for j, value := range row.Columns {
					expected := expectedRow[j]
					if value != expected {
						t.Errorf("Row %d, Column %d = %v, expected %v",
							i, j, value, expected)
					}
				}
			}
		})
	}
}

func TestCalculateFieldsWithCalculates(t *testing.T) {
	// Create a logger for testing
	log := logger.DefaultLogger(nil, loggertype.LogLevelInfo)

	// Create metrics collector
	mc := NewMetricsCollector(log)

	tests := []struct {
		name           string
		metrics        *jobtypes.Metrics
		collectResult  *jobtypes.CollectRepMetricsData
		expectedValues [][]string
	}{
		{
			name: "calculates with string array - simple field mapping",
			metrics: &jobtypes.Metrics{
				Name:     "memory",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"total_bytes", "used_bytes"},
				Calculates:  []string{"total=total_bytes", "used=used_bytes"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1024", "512"}},
				},
			},
			expectedValues: [][]string{
				{"1024", "512"},
			},
		},
		{
			name: "calculates with string array - expression",
			metrics: &jobtypes.Metrics{
				Name:     "memory",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
					{Field: "usage", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"total", "used"},
				Calculates:  []string{"usage=(used / total) * 100"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1024", "512"}},
				},
			},
			expectedValues: [][]string{
				{"1024", "512", "50"},
			},
		},
		{
			name: "calculates with string array - mixed mapping and expression",
			metrics: &jobtypes.Metrics{
				Name:     "disk",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
					{Field: "free", Type: constants.TYPE_NUMBER},
					{Field: "usage_percent", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"total_space", "used_space", "free_space", "usage"},
				Calculates: []string{
					"total=total_space",
					"used=used_space",
					"free=free_space",
					"usage_percent=usage",
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"2048", "1024", "1024", "50"}},
				},
			},
			expectedValues: [][]string{
				{"2048", "1024", "1024", "50"},
			},
		},
		{
			name: "calculates with Calculate struct array",
			metrics: &jobtypes.Metrics{
				Name:     "memory",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
					{Field: "free", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"total_mb", "used_mb", "free_mb"},
				Calculates: []jobtypes.Calculate{
					{Field: "total", AliasField: "total_mb"},
					{Field: "used", AliasField: "used_mb"},
					{Field: "free", AliasField: "free_mb"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"4096", "2048", "2048"}},
				},
			},
			expectedValues: [][]string{
				{"4096", "2048", "2048"},
			},
		},
		{
			name: "calculates with Calculate struct array - complex expressions",
			metrics: &jobtypes.Metrics{
				Name:     "jvm",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "heap_max", Type: constants.TYPE_NUMBER},
					{Field: "heap_used", Type: constants.TYPE_NUMBER},
					{Field: "heap_free", Type: constants.TYPE_NUMBER},
					{Field: "heap_usage", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"max", "used", "free", "usage"},
				Calculates: []jobtypes.Calculate{
					{Field: "heap_max", AliasField: "max"},
					{Field: "heap_used", AliasField: "used"},
					{Field: "heap_free", AliasField: "free"},
					{Field: "heap_usage", AliasField: "usage"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"8192", "6144", "2048", "75"}},
				},
			},
			expectedValues: [][]string{
				{"8192", "6144", "2048", "75"},
			},
		},
		{
			name: "calculates with interface array (string format)",
			metrics: &jobtypes.Metrics{
				Name:     "cpu",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "idle", Type: constants.TYPE_NUMBER},
					{Field: "usage", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"idle_percent", "usage_percent"},
				Calculates:  []interface{}{"idle=idle_percent", "usage=usage_percent"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"25", "75"}},
				},
			},
			expectedValues: [][]string{
				{"25", "75"},
			},
		},
		{
			name: "calculates with interface array (Calculate struct format)",
			metrics: &jobtypes.Metrics{
				Name:     "network",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "rx_bytes", Type: constants.TYPE_NUMBER},
					{Field: "tx_bytes", Type: constants.TYPE_NUMBER},
					{Field: "total_bytes", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"received", "transmitted", "total"},
				Calculates: []interface{}{
					map[string]interface{}{"field": "rx_bytes", "aliasField": "received"},
					map[string]interface{}{"field": "tx_bytes", "aliasField": "transmitted"},
					map[string]interface{}{"field": "total_bytes", "aliasField": "total"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1024", "2048", "3072"}},
				},
			},
			expectedValues: [][]string{
				{"1024", "2048", "3072"},
			},
		},
		{
			name: "calculates with interface array (mixed string and map)",
			metrics: &jobtypes.Metrics{
				Name:     "system",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "load1", Type: constants.TYPE_NUMBER},
					{Field: "load5", Type: constants.TYPE_NUMBER},
					{Field: "load15", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"load_1min", "load_5min", "load_15min"},
				Calculates: []interface{}{
					"load1=load_1min",
					map[string]interface{}{"field": "load5", "aliasField": "load_5min"},
					"load15=load_15min",
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1.5", "2.3", "3.1"}},
				},
			},
			expectedValues: [][]string{
				{"1.5", "2.3", "3.1"},
			},
		},
		{
			name: "calculates with interface array (expression in string)",
			metrics: &jobtypes.Metrics{
				Name:     "database",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "connections", Type: constants.TYPE_NUMBER},
					{Field: "max_connections", Type: constants.TYPE_NUMBER},
					{Field: "usage_rate", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"active", "max"},
				Calculates: []interface{}{
					"connections=active",
					"max_connections=max",
					"usage_rate=(connections / max_connections) * 100",
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"75", "100"}},
				},
			},
			expectedValues: [][]string{
				{"75", "100", "75"},
			},
		},
		{
			name: "calculates with interface array (script in map)",
			metrics: &jobtypes.Metrics{
				Name:     "storage",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
					{Field: "available", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"capacity", "occupied", "free_space"},
				Calculates: []interface{}{
					map[string]interface{}{"field": "total", "aliasField": "capacity"},
					map[string]interface{}{"field": "used", "aliasField": "occupied"},
					map[string]interface{}{"field": "available", "aliasField": "free_space"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"10240", "7168", "3072"}},
				},
			},
			expectedValues: [][]string{
				{"10240", "7168", "3072"},
			},
		},
		{
			name: "no calculates configuration",
			metrics: &jobtypes.Metrics{
				Name:     "simple",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "value", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"value"},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"100"}},
				},
			},
			expectedValues: [][]string{
				{"100"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute CalculateFields
			mc.CalculateFields(tt.metrics, tt.collectResult)

			// Verify results
			if len(tt.collectResult.Values) != len(tt.expectedValues) {
				t.Errorf("CalculateFields() resulted in %d rows, expected %d",
					len(tt.collectResult.Values), len(tt.expectedValues))
				return
			}

			for i, row := range tt.collectResult.Values {
				expectedRow := tt.expectedValues[i]
				if len(row.Columns) != len(expectedRow) {
					t.Errorf("Row %d has %d columns, expected %d",
						i, len(row.Columns), len(expectedRow))
					continue
				}

				for j, value := range row.Columns {
					expected := expectedRow[j]
					if value != expected {
						t.Errorf("Row %d, Column %d = %v, expected %v",
							i, j, value, expected)
					}
				}
			}
		})
	}
}

func TestParseUnits(t *testing.T) {
	// Create a logger for testing
	log := logger.DefaultLogger(nil, loggertype.LogLevelInfo)

	// Create metrics collector
	mc := NewMetricsCollector(log)

	tests := []struct {
		name          string
		units         interface{}
		expectedCount int
		expectedField string
		expectedFrom  string
		expectedTo    string
	}{
		{
			name:          "string array format",
			units:         []string{"committed=B->MB", "max=KB->GB"},
			expectedCount: 2,
			expectedField: "committed",
			expectedFrom:  "B",
			expectedTo:    "MB",
		},
		{
			name:          "interface array format with strings",
			units:         []interface{}{"memory=B->MB"},
			expectedCount: 1,
			expectedField: "memory",
			expectedFrom:  "B",
			expectedTo:    "MB",
		},
		{
			name: "Unit struct array format",
			units: []jobtypes.Unit{
				{Field: "total", Unit: "B->MB"},
				{Field: "used", Unit: "KB->GB"},
			},
			expectedCount: 2,
			expectedField: "total",
			expectedFrom:  "B",
			expectedTo:    "MB",
		},
		{
			name: "interface array format with Unit maps",
			units: []interface{}{
				map[string]interface{}{"field": "heap", "unit": "B->MB"},
				map[string]interface{}{"field": "stack", "unit": "KB->MB"},
			},
			expectedCount: 2,
			expectedField: "heap",
			expectedFrom:  "B",
			expectedTo:    "MB",
		},
		{
			name: "mixed interface array (strings and maps)",
			units: []interface{}{
				"total=B->GB",
				map[string]interface{}{"field": "used", "unit": "KB->MB"},
				"free=MB->GB",
			},
			expectedCount: 3,
			expectedField: "used",
			expectedFrom:  "KB",
			expectedTo:    "MB",
		},
		{
			name:          "nil units",
			units:         nil,
			expectedCount: 0,
		},
		{
			name:          "empty array",
			units:         []string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mc.parseUnits(tt.units)

			if len(result) != tt.expectedCount {
				t.Errorf("parseUnits() resulted in %d conversions, expected %d",
					len(result), tt.expectedCount)
				return
			}

			if tt.expectedCount > 0 && tt.expectedField != "" {
				conversion, exists := result[tt.expectedField]
				if !exists {
					t.Errorf("Expected field %s not found in result", tt.expectedField)
					return
				}

				if conversion.OriginUnit != tt.expectedFrom {
					t.Errorf("OriginUnit = %v, expected %v", conversion.OriginUnit, tt.expectedFrom)
				}

				if conversion.NewUnit != tt.expectedTo {
					t.Errorf("NewUnit = %v, expected %v", conversion.NewUnit, tt.expectedTo)
				}
			}
		})
	}
}

func TestCalculateFieldsWithUnitStructFormat(t *testing.T) {
	// Create a logger for testing
	log := logger.DefaultLogger(nil, loggertype.LogLevelInfo)

	// Create metrics collector
	mc := NewMetricsCollector(log)

	tests := []struct {
		name           string
		metrics        *jobtypes.Metrics
		collectResult  *jobtypes.CollectRepMetricsData
		expectedValues [][]string
	}{
		{
			name: "Unit struct array format",
			metrics: &jobtypes.Metrics{
				Name:     "memory",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "total", Type: constants.TYPE_NUMBER},
					{Field: "used", Type: constants.TYPE_NUMBER},
					{Field: "free", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"total", "used", "free"},
				Units: []jobtypes.Unit{
					{Field: "total", Unit: "B->MB"},
					{Field: "used", Unit: "B->MB"},
					{Field: "free", Unit: "B->MB"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"2097152", "1048576", "1048576"}}, // 2MB, 1MB, 1MB in bytes
				},
			},
			expectedValues: [][]string{
				{"2", "1", "1"}, // Converted to MB
			},
		},
		{
			name: "mixed interface array with Unit maps and strings",
			metrics: &jobtypes.Metrics{
				Name:     "storage",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "disk_total", Type: constants.TYPE_NUMBER},
					{Field: "disk_used", Type: constants.TYPE_NUMBER},
					{Field: "disk_free", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"disk_total", "disk_used", "disk_free"},
				Units: []interface{}{
					map[string]interface{}{"field": "disk_total", "unit": "KB->GB"},
					"disk_used=KB->GB",
					map[string]interface{}{"field": "disk_free", "unit": "KB->GB"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"1048576", "524288", "524288"}}, // 1GB, 0.5GB, 0.5GB in KB
				},
			},
			expectedValues: [][]string{
				{"1", "0.5", "0.5"}, // Converted to GB
			},
		},
		{
			name: "Unit struct with calculates",
			metrics: &jobtypes.Metrics{
				Name:     "jvm",
				Priority: 0,
				Fields: []jobtypes.Field{
					{Field: "heap_max", Type: constants.TYPE_NUMBER},
					{Field: "heap_used", Type: constants.TYPE_NUMBER},
					{Field: "heap_usage", Type: constants.TYPE_NUMBER},
				},
				AliasFields: []string{"max_bytes", "used_bytes"},
				Calculates:  []string{"heap_max=max_bytes", "heap_used=used_bytes", "heap_usage=(heap_used / heap_max) * 100"},
				Units: []jobtypes.Unit{
					{Field: "heap_max", Unit: "B->MB"},
					{Field: "heap_used", Unit: "B->MB"},
				},
			},
			collectResult: &jobtypes.CollectRepMetricsData{
				Code: constants.CollectSuccess,
				Values: []jobtypes.ValueRow{
					{Columns: []string{"4194304", "3145728"}}, // 4MB, 3MB in bytes
				},
			},
			expectedValues: [][]string{
				{"4", "3", "75"}, // Converted to MB, usage calculated
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute CalculateFields
			mc.CalculateFields(tt.metrics, tt.collectResult)

			// Verify results
			if len(tt.collectResult.Values) != len(tt.expectedValues) {
				t.Errorf("CalculateFields() resulted in %d rows, expected %d",
					len(tt.collectResult.Values), len(tt.expectedValues))
				return
			}

			for i, row := range tt.collectResult.Values {
				expectedRow := tt.expectedValues[i]
				if len(row.Columns) != len(expectedRow) {
					t.Errorf("Row %d has %d columns, expected %d",
						i, len(row.Columns), len(expectedRow))
					continue
				}

				for j, value := range row.Columns {
					expected := expectedRow[j]
					if value != expected {
						t.Errorf("Row %d, Column %d = %v, expected %v",
							i, j, value, expected)
					}
				}
			}
		})
	}
}
