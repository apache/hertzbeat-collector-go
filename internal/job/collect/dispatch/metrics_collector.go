/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/job/collect/strategy"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/param"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/unit"
)

// MetricsCollector handles metrics collection using goroutines and channels
type MetricsCollector struct {
	logger logger.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger logger.Logger) *MetricsCollector {
	return &MetricsCollector{
		logger: logger.WithName("metrics-collector"),
	}
}

// CalculateFields calculates fields in metrics by configured expressions
// This method processes the collected data and applies calculations, unit conversions, and filters
func (mc *MetricsCollector) CalculateFields(metrics *jobtypes.Metrics, result *jobtypes.CollectRepMetricsData) {
	if result == nil || metrics == nil {
		return
	}
	result.Priority = metrics.Priority
	fields := metrics.Fields
	result.Fields = fields
	aliasRowList := result.Values
	if len(aliasRowList) == 0 {
		return
	}
	result.Values = make([]jobtypes.ValueRow, 0)
	aliasFields := metrics.AliasFields
	if len(aliasFields) == 0 {
		return
	}
	replacer := param.NewReplacer()
	fieldExpressionMap, fieldAliasMap := mc.parseCalculates(metrics.Calculates)
	fieldUnitConversionMap := mc.parseUnits(metrics.Units)

	for _, aliasRow := range aliasRowList {
		aliasFieldValueMap := make(map[string]string)
		fieldValueMap := make(map[string]interface{})
		stringTypeFieldValueMap := make(map[string]interface{})
		aliasFieldUnitMap := make(map[string]string)

		for aliasIndex, aliasField := range aliasFields {
			if aliasIndex >= len(aliasRow.Columns) {
				break
			}
			aliasFieldValue := aliasRow.Columns[aliasIndex]
			if aliasFieldValue == constants.NULL_VALUE {
				fieldValueMap[aliasField] = nil
				stringTypeFieldValueMap[aliasField] = nil
				continue
			}
			aliasFieldValueMap[aliasField] = aliasFieldValue
			// Try to extract numeric value and unit
			doubleAndUnit := replacer.ExtractDoubleAndUnitFromStr(aliasFieldValue)
			if doubleAndUnit != nil {
				fieldValueMap[aliasField] = doubleAndUnit.Value
				if doubleAndUnit.Unit != "" {
					aliasFieldUnitMap[aliasField] = doubleAndUnit.Unit
				}
			} else {
				fieldValueMap[aliasField] = aliasFieldValue
			}
			stringTypeFieldValueMap[aliasField] = aliasFieldValue
		}

		// Build the real value row based on configured fields
		realValueRow := jobtypes.ValueRow{
			Columns: make([]string, 0, len(fields)),
		}

		for _, field := range fields {
			realField := field.Field
			var value string

			if program, exists := fieldExpressionMap[realField]; exists {
				var context map[string]interface{}
				if field.Type == constants.TYPE_STRING {
					context = stringTypeFieldValueMap
				} else {
					context = fieldValueMap
				}
				output, err := expr.Run(program, context)
				if err != nil {
					mc.logger.V(1).Info("expression evaluation failed, using original value",
						"field", realField,
						"error", err)
					// Fallback to alias field value
					aliasField := fieldAliasMap[realField]
					if aliasField != "" {
						value = aliasFieldValueMap[aliasField]
					} else {
						value = aliasFieldValueMap[realField]
					}
				} else if output != nil {
					value = fmt.Sprintf("%v", output)
				}
			} else {
				// No expression, do simple field mapping
				aliasField := fieldAliasMap[realField]
				if aliasField != "" {
					value = aliasFieldValueMap[aliasField]
				} else {
					value = aliasFieldValueMap[realField]
				}
			}

			// Process based on field type if value exists
			if value != "" && value != constants.NULL_VALUE {
				switch field.Type {
				case constants.TYPE_NUMBER:
					// Extract numeric value and format
					doubleAndUnit := replacer.ExtractDoubleAndUnitFromStr(value)
					if doubleAndUnit != nil {
						numericValue := doubleAndUnit.Value
						// Apply unit conversion if configured for this field
						if conversion, exists := fieldUnitConversionMap[realField]; exists {
							// Get the original unit from extracted value or use configured origin unit
							originUnit := doubleAndUnit.Unit
							if originUnit == "" {
								originUnit = conversion.OriginUnit
							}
							// Perform unit conversion
							convertedValue, err := unit.Convert(numericValue, originUnit, conversion.NewUnit)
							if err != nil {
								mc.logger.V(1).Info("unit conversion failed, using original value",
									"field", realField,
									"originUnit", originUnit,
									"newUnit", conversion.NewUnit,
									"error", err)
							} else {
								numericValue = convertedValue
								mc.logger.V(2).Info("unit conversion applied",
									"field", realField,
									"originalValue", doubleAndUnit.Value,
									"originUnit", originUnit,
									"convertedValue", convertedValue,
									"newUnit", conversion.NewUnit)
							}
						}
						value = formatNumber(numericValue)
					} else {
						value = constants.NULL_VALUE
					}

				case constants.TYPE_TIME:
					// TODO: Implement time parsing
					// For now, keep original value
				}
			}

			if value == "" {
				value = constants.NULL_VALUE
			}

			realValueRow.Columns = append(realValueRow.Columns, value)

			// Add the calculated field value to context maps for use in subsequent expressions
			// This allows later expressions to reference earlier calculated fields
			if value != constants.NULL_VALUE {
				doubleAndUnit := replacer.ExtractDoubleAndUnitFromStr(value)
				if doubleAndUnit != nil {
					fieldValueMap[realField] = doubleAndUnit.Value
				} else {
					fieldValueMap[realField] = value
				}
				stringTypeFieldValueMap[realField] = value
			} else {
				fieldValueMap[realField] = nil
				stringTypeFieldValueMap[realField] = nil
			}
		}

		// TODO: Apply filters based on metrics.Filters configuration

		result.Values = append(result.Values, realValueRow)
	}
}

// parseCalculates parses the calculates configuration and compiles expressions
func (mc *MetricsCollector) parseCalculates(calculates interface{}) (map[string]*vm.Program, map[string]string) {
	fieldExpressionMap := make(map[string]*vm.Program)
	fieldAliasMap := make(map[string]string)

	if calculates == nil {
		return fieldExpressionMap, fieldAliasMap
	}

	switch v := calculates.(type) {
	case []string:
		mc.parseCalculatesFromStringArray(v, fieldExpressionMap, fieldAliasMap)

	case []jobtypes.Calculate:
		mc.parseCalculatesFromStructArray(v, fieldExpressionMap, fieldAliasMap)

	case []interface{}:
		// Process each item individually to support mixed formats
		for _, item := range v {
			if str, ok := item.(string); ok {
				// Handle string format: "field=expression" or "field=aliasField"
				parts := splitCalculateString(str)
				if len(parts) == 2 {
					field := parts[0]
					expression := parts[1]

					if expression == field || isSimpleFieldName(expression) {
						fieldAliasMap[field] = expression
					} else {
						program, err := expr.Compile(expression)
						if err != nil {
							mc.logger.Error(err, "failed to compile expression",
								"field", field,
								"expression", expression)
							fieldAliasMap[field] = expression
							continue
						}
						fieldExpressionMap[field] = program
					}
				}
			} else if calcMap, ok := item.(map[string]interface{}); ok {
				// Handle map format: {"field": "fieldName", "aliasField": "alias", "script": "expression"}
				calc := jobtypes.Calculate{}
				if field, ok := calcMap["field"].(string); ok {
					calc.Field = field
				}
				if script, ok := calcMap["script"].(string); ok {
					calc.Script = script
				}
				if aliasField, ok := calcMap["aliasField"].(string); ok {
					calc.AliasField = aliasField
				}

				if calc.Field != "" {
					if calc.Script != "" {
						program, err := expr.Compile(calc.Script)
						if err != nil {
							mc.logger.Error(err, "failed to compile expression",
								"field", calc.Field,
								"script", calc.Script)
							continue
						}
						fieldExpressionMap[calc.Field] = program
					}
					if calc.AliasField != "" {
						fieldAliasMap[calc.Field] = calc.AliasField
					}
				}
			}
		}

	default:
		// Try to unmarshal from JSON as last resort
		if jsonData, err := json.Marshal(calculates); err == nil {
			var stringArray []string
			if err := json.Unmarshal(jsonData, &stringArray); err == nil {
				mc.parseCalculatesFromStringArray(stringArray, fieldExpressionMap, fieldAliasMap)
			} else {
				var calculateList []jobtypes.Calculate
				if err := json.Unmarshal(jsonData, &calculateList); err == nil {
					mc.parseCalculatesFromStructArray(calculateList, fieldExpressionMap, fieldAliasMap)
				}
			}
		}
	}

	return fieldExpressionMap, fieldAliasMap
}

// parseUnits parses the units configuration for unit conversions
func (mc *MetricsCollector) parseUnits(units interface{}) map[string]*unit.Conversion {
	fieldUnitConversionMap := make(map[string]*unit.Conversion)

	if units == nil {
		return fieldUnitConversionMap
	}

	switch v := units.(type) {
	case []string:
		mc.parseUnitsFromStringArray(v, fieldUnitConversionMap)

	case []jobtypes.Unit:
		mc.parseUnitsFromStructArray(v, fieldUnitConversionMap)

	case []interface{}:
		// Process each item individually to support mixed formats
		for _, item := range v {
			if str, ok := item.(string); ok {
				// Handle string format: "fieldName=originUnit->newUnit"
				conversion, err := unit.ParseConversion(str)
				if err != nil {
					mc.logger.V(1).Info("invalid unit conversion format, skipping",
						"unitStr", str,
						"error", err)
					continue
				}
				fieldUnitConversionMap[conversion.Field] = conversion
			} else if unitMap, ok := item.(map[string]interface{}); ok {
				// Handle map format: {"field": "fieldName", "unit": "originUnit->newUnit"}
				unitStruct := jobtypes.Unit{}
				if field, ok := unitMap["field"].(string); ok {
					unitStruct.Field = field
				}
				if unitStr, ok := unitMap["unit"].(string); ok {
					unitStruct.Unit = unitStr
				}

				if unitStruct.Field != "" && unitStruct.Unit != "" {
					// Parse the unit conversion string
					conversionStr := unitStruct.Field + "=" + unitStruct.Unit
					conversion, err := unit.ParseConversion(conversionStr)
					if err != nil {
						mc.logger.V(1).Info("invalid unit conversion format, skipping",
							"field", unitStruct.Field,
							"unit", unitStruct.Unit,
							"error", err)
						continue
					}
					fieldUnitConversionMap[conversion.Field] = conversion
				}
			}
		}

	default:
		// Try to unmarshal from JSON as last resort
		if jsonData, err := json.Marshal(units); err == nil {
			var stringArray []string
			if err := json.Unmarshal(jsonData, &stringArray); err == nil {
				mc.parseUnitsFromStringArray(stringArray, fieldUnitConversionMap)
			} else {
				var unitList []jobtypes.Unit
				if err := json.Unmarshal(jsonData, &unitList); err == nil {
					mc.parseUnitsFromStructArray(unitList, fieldUnitConversionMap)
				}
			}
		}
	}

	return fieldUnitConversionMap
}

// parseUnitsFromStringArray parses units from string array format
func (mc *MetricsCollector) parseUnitsFromStringArray(units []string, fieldUnitConversionMap map[string]*unit.Conversion) {
	for _, unitStr := range units {
		conversion, err := unit.ParseConversion(unitStr)
		if err != nil {
			mc.logger.V(1).Info("invalid unit conversion format, skipping",
				"unitStr", unitStr,
				"error", err)
			continue
		}
		fieldUnitConversionMap[conversion.Field] = conversion
	}
}

// parseUnitsFromStructArray parses units from Unit struct array
func (mc *MetricsCollector) parseUnitsFromStructArray(units []jobtypes.Unit, fieldUnitConversionMap map[string]*unit.Conversion) {
	for _, u := range units {
		if u.Field == "" || u.Unit == "" {
			continue
		}
		// Parse the unit conversion string
		conversionStr := u.Field + "=" + u.Unit
		conversion, err := unit.ParseConversion(conversionStr)
		if err != nil {
			mc.logger.V(1).Info("invalid unit conversion format, skipping",
				"field", u.Field,
				"unit", u.Unit,
				"error", err)
			continue
		}
		fieldUnitConversionMap[conversion.Field] = conversion
	}
}

// parseCalculatesFromStringArray parses calculates from string array format
func (mc *MetricsCollector) parseCalculatesFromStringArray(calculates []string,
	fieldExpressionMap map[string]*vm.Program, fieldAliasMap map[string]string) {

	for _, calc := range calculates {
		parts := splitCalculateString(calc)
		if len(parts) != 2 {
			mc.logger.V(1).Info("invalid calculate format, skipping",
				"calculate", calc)
			continue
		}

		field := parts[0]
		expression := parts[1]

		// Check if expression is just a simple field name (alias mapping)
		// If expression equals field name, it's a direct mapping
		if expression == field || isSimpleFieldName(expression) {
			fieldAliasMap[field] = expression
		} else {
			program, err := expr.Compile(expression)
			if err != nil {
				mc.logger.Error(err, "failed to compile expression",
					"field", field,
					"expression", expression)
				fieldAliasMap[field] = expression
				continue
			}
			fieldExpressionMap[field] = program
		}
	}
}

// parseCalculatesFromStructArray parses calculates from Calculate struct array
func (mc *MetricsCollector) parseCalculatesFromStructArray(calculates []jobtypes.Calculate,
	fieldExpressionMap map[string]*vm.Program, fieldAliasMap map[string]string) {

	for _, calc := range calculates {
		if calc.Script != "" {
			program, err := expr.Compile(calc.Script)
			if err != nil {
				mc.logger.Error(err, "failed to compile expression",
					"field", calc.Field,
					"script", calc.Script)
				continue
			}
			fieldExpressionMap[calc.Field] = program
		}
		if calc.AliasField != "" {
			fieldAliasMap[calc.Field] = calc.AliasField
		}
	}
}

// splitCalculateString splits "field=expression" into [field, expression]
func splitCalculateString(calc string) []string {
	idx := strings.Index(calc, "=")
	if idx == -1 {
		return []string{calc}
	}

	field := calc[:idx]
	expression := calc[idx+1:]
	return []string{field, expression}
}

// isSimpleFieldName checks if a string is a simple field name (alphanumeric and underscore only)
func isSimpleFieldName(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// formatNumber formats a float64 to string with up to 4 decimal places
func formatNumber(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 4, 64)

	// Remove trailing zeros after decimal point
	if len(formatted) > 0 && (formatted[len(formatted)-1] == '0' || formatted[len(formatted)-1] == '.') {
		formatted = strconv.FormatFloat(value, 'f', -1, 64)
	}

	return formatted
}

// CollectMetrics collects metrics using the appropriate collector and returns results via channel
// This method uses Go's channel-based concurrency instead of Java's thread pools
func (mc *MetricsCollector) CollectMetrics(metrics *jobtypes.Metrics, job *jobtypes.Job, _ *jobtypes.Timeout) chan *jobtypes.CollectRepMetricsData {
	resultChan := make(chan *jobtypes.CollectRepMetricsData, 1)

	// Start collection in a goroutine (Go's lightweight thread)
	go func() {
		defer close(resultChan)

		// Debug level only for collection start
		mc.logger.V(1).Info("starting metrics collection",
			"metricsName", metrics.Name,
			"protocol", metrics.Protocol)

		startTime := time.Now()

		// Populate metrics.ConfigMap from job.Configmap if missing
		if metrics.ConfigMap == nil {
			metrics.ConfigMap = make(map[string]string)
		}
		// Merge global job config params into metrics config map
		if job.Configmap != nil {
			for _, param := range job.Configmap {
				// Fix: Handle nil value gracefully to avoid "<nil>" literal in string
				var valStr string
				if param.Value == nil {
					valStr = ""
				} else {
					valStr = fmt.Sprintf("%v", param.Value)
				}
				metrics.ConfigMap[param.Key] = valStr
			}
		}

		// Perform parameter replacement using the centralized Replacer
		// This handles all protocols (HTTP, SSH, JDBC, etc.)
		replacer := param.NewReplacer()
		if err := replacer.ReplaceMetricsParams(metrics, metrics.ConfigMap); err != nil {
			mc.logger.Error(err, "failed to replace params", "metricsName", metrics.Name)
			resultChan <- mc.createErrorResponse(metrics, job, constants.CollectFail, fmt.Sprintf("Param replacement error: %v", err))
			return
		}

		// Get the appropriate collector based on protocol
		collector, err := strategy.CollectorFor(metrics.Protocol)
		if err != nil {
			mc.logger.Error(err, "failed to get collector",
				"protocol", metrics.Protocol,
				"metricsName", metrics.Name)

			result := mc.createErrorResponse(metrics, job, constants.CollectUnavailable, fmt.Sprintf("Collector not found: %v", err))
			resultChan <- result
			return
		}

		// Perform the actual collection
		result := collector.Collect(metrics)

		// calculate fields and convert units by metrics.Calculates and metrics.Units
		if result != nil && result.Code == constants.CollectSuccess {
			mc.CalculateFields(metrics, result)
		}

		// Enrich result with job information
		if result != nil {
			result.ID = job.MonitorID // Use MonitorID for both ID and MonitorID fields
			result.MonitorID = job.MonitorID
			result.App = job.App
			result.TenantID = job.TenantID

			// Add labels from job
			if result.Labels == nil {
				result.Labels = make(map[string]string)
			}
			for k, v := range job.Labels {
				result.Labels[k] = v
			}

			// Add metadata from job
			if result.Metadata == nil {
				result.Metadata = make(map[string]string)
			}
			for k, v := range job.Metadata {
				result.Metadata[k] = v
			}
		}

		duration := time.Since(startTime)

		// Only log failures at INFO level, success at debug level
		if result != nil && result.Code == constants.CollectSuccess {
			mc.logger.V(1).Info("metrics collection completed successfully",
				"metricsName", metrics.Name,
				"protocol", metrics.Protocol,
				"duration", duration,
				"valuesCount", len(result.Values))
		} else {
			mc.logger.Info("metrics collection failed",
				"jobID", job.ID,
				"metricsName", metrics.Name,
				"protocol", metrics.Protocol,
				"duration", duration,
				"code", result.Code,
				"message", result.Msg)
		}

		resultChan <- result
	}()

	return resultChan
}

// createErrorResponse creates an error response for failed collections
func (mc *MetricsCollector) createErrorResponse(metrics *jobtypes.Metrics, job *jobtypes.Job, code int, message string) *jobtypes.CollectRepMetricsData {
	return &jobtypes.CollectRepMetricsData{
		ID:        job.MonitorID, // Use MonitorID for both ID and MonitorID fields
		MonitorID: job.MonitorID,
		TenantID:  job.TenantID,
		App:       job.App,
		Metrics:   metrics.Name,
		Priority:  metrics.Priority,
		Time:      time.Now().UnixMilli(),
		Code:      code,
		Msg:       message,
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}
}
