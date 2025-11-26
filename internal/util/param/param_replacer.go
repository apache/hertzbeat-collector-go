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

package param

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/crypto"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// Replacer is a standalone utility for parameter replacement
// This matches Java's parameter substitution mechanism where ^_^paramName^_^ gets replaced with actual values
type Replacer struct{}

// NewReplacer creates a new parameter replacer
func NewReplacer() *Replacer {
	return &Replacer{}
}

// PreprocessJobPasswords decrypts passwords in job configmap once during job creation
// This permanently replaces encrypted passwords with decrypted ones in the job configmap
func (r *Replacer) PreprocessJobPasswords(job *jobtypes.Job) error {
	if job == nil || job.Configmap == nil {
		return nil
	}

	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug).WithName("password-preprocessor")

	// Decrypt passwords in configmap once and permanently replace them
	for i := range job.Configmap {
		config := &job.Configmap[i] // Get pointer to modify in place
		if config.Type == 2 {       // password type
			if encryptedValue, ok := config.Value.(string); ok {
				log.Sugar().Debugf("preprocessing encrypted password for key: %s", config.Key)
				if decoded, err := r.decryptPassword(encryptedValue); err == nil {
					log.Info("password preprocessing successful", "key", config.Key, "length", len(decoded))
					config.Value = decoded // Permanently replace encrypted value with decrypted value
					config.Type = 1        // Change type to string since it's now decrypted
				} else {
					log.Error(err, "password preprocessing failed, keeping original value", "key", config.Key)
				}
			}
		}
	}
	return nil
}

// ReplaceJobParams replaces all parameter placeholders in job configuration
func (r *Replacer) ReplaceJobParams(job *jobtypes.Job) (*jobtypes.Job, error) {
	if job == nil {
		return nil, fmt.Errorf("job is nil")
	}

	// Create parameter map from configmap (passwords should already be decrypted)
	paramMap := r.createParamMapSimple(job.Configmap)

	// Clone the job to avoid modifying original
	clonedJob := job.Clone()
	if clonedJob == nil {
		return nil, fmt.Errorf("failed to clone job")
	}

	// Replace parameters in all metrics
	for i := range clonedJob.Metrics {
		if err := r.ReplaceMetricsParams(&clonedJob.Metrics[i], paramMap); err != nil {
			return nil, fmt.Errorf("failed to replace params in metrics %s: %w", clonedJob.Metrics[i].Name, err)
		}
	}

	return clonedJob, nil
}

// createParamMapSimple creates a parameter map from configmap entries
// Assumes passwords have already been decrypted by PreprocessJobPasswords
func (r *Replacer) createParamMapSimple(configmap []jobtypes.Configmap) map[string]string {
	paramMap := make(map[string]string)

	for _, config := range configmap {
		// Convert value to string, handle null values as empty strings
		var strValue string
		if config.Value == nil {
			strValue = "" // null values become empty strings
		} else {
			switch config.Type {
			case 0: // number
				if numVal, ok := config.Value.(float64); ok {
					strValue = strconv.FormatFloat(numVal, 'f', -1, 64)
				} else if intVal, ok := config.Value.(int); ok {
					strValue = strconv.Itoa(intVal)
				} else {
					strValue = fmt.Sprintf("%v", config.Value)
				}
			case 1, 2: // string or password (both are now plain strings after preprocessing)
				strValue = fmt.Sprintf("%v", config.Value)
			default:
				strValue = fmt.Sprintf("%v", config.Value)
			}
		}
		paramMap[config.Key] = strValue
	}

	return paramMap
}

// ReplaceMetricsParams replaces parameters in metrics configuration
// Exported method to be called by MetricsCollector
func (r *Replacer) ReplaceMetricsParams(metrics *jobtypes.Metrics, paramMap map[string]string) error {
	// 1. JDBC
	if metrics.JDBC != nil {
		if err := r.replaceProtocolParams(&metrics.JDBC, paramMap); err != nil {
			return fmt.Errorf("failed to replace JDBC params: %w", err)
		}
	}

	// 2. SSH
	if metrics.SSH != nil {
		if err := r.replaceProtocolParams(&metrics.SSH, paramMap); err != nil {
			return fmt.Errorf("failed to replace SSH params: %w", err)
		}
	}

	// 3. HTTP
	if metrics.HTTP != nil {
		r.replaceHTTPParams(metrics.HTTP, paramMap)
	}

	// Replace parameters in basic metrics fields
	if err := r.replaceBasicMetricsParams(metrics, paramMap); err != nil {
		return fmt.Errorf("failed to replace basic metrics params: %w", err)
	}

	return nil
}

// replaceHTTPParams specific replacement logic for HTTPProtocol struct
func (r *Replacer) replaceHTTPParams(http *jobtypes.HTTPProtocol, paramMap map[string]string) {
	http.URL = r.replaceParamPlaceholders(http.URL, paramMap)
	http.Method = r.replaceParamPlaceholders(http.Method, paramMap)
	http.Body = r.replaceParamPlaceholders(http.Body, paramMap)
	http.ParseScript = r.replaceParamPlaceholders(http.ParseScript, paramMap)
	http.ParseType = r.replaceParamPlaceholders(http.ParseType, paramMap)
	http.Keyword = r.replaceParamPlaceholders(http.Keyword, paramMap)
	http.Timeout = r.replaceParamPlaceholders(http.Timeout, paramMap)
	http.SSL = r.replaceParamPlaceholders(http.SSL, paramMap)

	// Headers
	if http.Headers != nil {
		newHeaders := make(map[string]string)
		for k, v := range http.Headers {
			newKey := r.replaceParamPlaceholders(k, paramMap)
			// Filter out keys that are empty or still contain placeholders
			if newKey != "" && !strings.Contains(newKey, "^_^") {
				newHeaders[newKey] = r.replaceParamPlaceholders(v, paramMap)
			}
		}
		http.Headers = newHeaders
	}

	// Params
	if http.Params != nil {
		newParams := make(map[string]string)
		for k, v := range http.Params {
			newKey := r.replaceParamPlaceholders(k, paramMap)
			if newKey != "" && !strings.Contains(newKey, "^_^") {
				newParams[newKey] = r.replaceParamPlaceholders(v, paramMap)
			}
		}
		http.Params = newParams
	}

	// Authorization
	if http.Authorization != nil {
		auth := http.Authorization
		auth.Type = r.replaceParamPlaceholders(auth.Type, paramMap)
		auth.BasicAuthUsername = r.replaceParamPlaceholders(auth.BasicAuthUsername, paramMap)
		auth.BasicAuthPassword = r.replaceParamPlaceholders(auth.BasicAuthPassword, paramMap)
		auth.DigestAuthUsername = r.replaceParamPlaceholders(auth.DigestAuthUsername, paramMap)
		auth.DigestAuthPassword = r.replaceParamPlaceholders(auth.DigestAuthPassword, paramMap)
		auth.BearerTokenToken = r.replaceParamPlaceholders(auth.BearerTokenToken, paramMap)
	}
}

// replaceProtocolParams replaces parameters in any protocol configuration defined as interface{}
func (r *Replacer) replaceProtocolParams(protocolInterface *interface{}, paramMap map[string]string) error {
	if *protocolInterface == nil {
		return nil
	}

	// Convert protocol interface{} to map for manipulation
	protocolMap, ok := (*protocolInterface).(map[string]interface{})
	if !ok {
		return nil
	}

	return r.replaceParamsInMap(protocolMap, paramMap)
}

// replaceParamsInMap recursively replaces parameters in a map structure
func (r *Replacer) replaceParamsInMap(data map[string]interface{}, paramMap map[string]string) error {
	for key, value := range data {
		switch v := value.(type) {
		case string:
			data[key] = r.replaceParamPlaceholders(v, paramMap)
		case map[string]interface{}:
			if err := r.replaceParamsInMap(v, paramMap); err != nil {
				return fmt.Errorf("failed to replace params in nested map %s: %w", key, err)
			}
		case []interface{}:
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if err := r.replaceParamsInMap(itemMap, paramMap); err != nil {
						return fmt.Errorf("failed to replace params in array item %d: %w", i, err)
					}
				} else if itemStr, ok := item.(string); ok {
					v[i] = r.replaceParamPlaceholders(itemStr, paramMap)
				}
			}
		}
	}
	return nil
}

// replaceBasicMetricsParams replaces parameters in basic metrics fields
func (r *Replacer) replaceBasicMetricsParams(metrics *jobtypes.Metrics, paramMap map[string]string) error {
	metrics.Host = r.replaceParamPlaceholders(metrics.Host, paramMap)
	metrics.Port = r.replaceParamPlaceholders(metrics.Port, paramMap)
	metrics.Timeout = r.replaceParamPlaceholders(metrics.Timeout, paramMap)
	metrics.Range = r.replaceParamPlaceholders(metrics.Range, paramMap)

	if metrics.ConfigMap != nil {
		for key, value := range metrics.ConfigMap {
			metrics.ConfigMap[key] = r.replaceParamPlaceholders(value, paramMap)
		}
	}
	return nil
}

// replaceParamPlaceholders replaces ^_^paramName^_^ placeholders with actual values
func (r *Replacer) replaceParamPlaceholders(template string, paramMap map[string]string) string {
	if template == "" {
		return ""
	}
	if !strings.Contains(template, "^_^") {
		return template
	}
	result := template
	for paramName, paramValue := range paramMap {
		if paramName == "" {
			continue
		}
		placeholder := fmt.Sprintf("^_^%s^_^", paramName)
		result = strings.ReplaceAll(result, placeholder, paramValue)
	}
	return result
}

// ExtractProtocolConfig extracts and processes protocol configuration after parameter replacement
func (r *Replacer) ExtractProtocolConfig(protocolInterface interface{}, targetStruct interface{}) error {
	if protocolInterface == nil {
		return fmt.Errorf("protocol interface is nil")
	}

	if protocolMap, ok := protocolInterface.(map[string]interface{}); ok {
		jsonData, err := json.Marshal(protocolMap)
		if err != nil {
			return fmt.Errorf("failed to marshal protocol config: %w", err)
		}

		if err := json.Unmarshal(jsonData, targetStruct); err != nil {
			return fmt.Errorf("failed to unmarshal protocol config: %w", err)
		}

		return nil
	}

	return fmt.Errorf("unsupported protocol config type: %T", protocolInterface)
}

// ExtractJDBCConfig extracts and processes JDBC configuration
func (r *Replacer) ExtractJDBCConfig(jdbcInterface interface{}) (*jobtypes.JDBCProtocol, error) {
	if jdbcInterface == nil {
		return nil, nil
	}
	if jdbcConfig, ok := jdbcInterface.(*jobtypes.JDBCProtocol); ok {
		return jdbcConfig, nil
	}
	var jdbcConfig jobtypes.JDBCProtocol
	if err := r.ExtractProtocolConfig(jdbcInterface, &jdbcConfig); err != nil {
		return nil, fmt.Errorf("failed to extract JDBC config: %w", err)
	}
	return &jdbcConfig, nil
}

// ExtractHTTPConfig extracts and processes HTTP configuration
func (r *Replacer) ExtractHTTPConfig(httpInterface interface{}) (*jobtypes.HTTPProtocol, error) {
	if httpInterface == nil {
		return nil, nil
	}
	if httpConfig, ok := httpInterface.(*jobtypes.HTTPProtocol); ok {
		return httpConfig, nil
	}
	var httpConfig jobtypes.HTTPProtocol
	if err := r.ExtractProtocolConfig(httpInterface, &httpConfig); err != nil {
		return nil, fmt.Errorf("failed to extract HTTP config: %w", err)
	}
	return &httpConfig, nil
}

// ExtractSSHConfig extracts and processes SSH configuration
func (r *Replacer) ExtractSSHConfig(sshInterface interface{}) (*jobtypes.SSHProtocol, error) {
	if sshInterface == nil {
		return nil, nil
	}
	if sshConfig, ok := sshInterface.(*jobtypes.SSHProtocol); ok {
		return sshConfig, nil
	}
	var sshConfig jobtypes.SSHProtocol
	if err := r.ExtractProtocolConfig(sshInterface, &sshConfig); err != nil {
		return nil, fmt.Errorf("failed to extract SSH config: %w", err)
	}
	return &sshConfig, nil
}

// decryptPassword decrypts an encrypted password using AES
func (r *Replacer) decryptPassword(encryptedPassword string) (string, error) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug).WithName("password-decrypt")
	if result, err := crypto.AesDecode(encryptedPassword); err == nil {
		log.Info("password decrypted successfully", "length", len(result))
		return result, nil
	} else {
		log.Sugar().Debugf("primary decryption failed: %v", err)
	}
	defaultKey := "tomSun28HaHaHaHa"
	if result, err := crypto.AesDecodeWithKey(encryptedPassword, defaultKey); err == nil {
		log.Info("password decrypted with default key", "length", len(result))
		return result, nil
	}
	log.Info("all decryption attempts failed, using original value")
	return encryptedPassword, nil
}

// DoubleAndUnit represents a numeric value with its unit
type DoubleAndUnit struct {
	Value float64
	Unit  string
}

var unitSymbols = []string{
	"%", "Gi", "Mi", "Ki", "G", "g", "M", "m", "K", "k", "B", "b",
}

// ExtractDoubleAndUnitFromStr extracts a double value and unit from a string
func (r *Replacer) ExtractDoubleAndUnitFromStr(str string) *DoubleAndUnit {
	if str == "" {
		return nil
	}
	str = strings.TrimSpace(str)
	doubleAndUnit := &DoubleAndUnit{}
	if value, err := strconv.ParseFloat(str, 64); err == nil {
		doubleAndUnit.Value = value
		return doubleAndUnit
	}
	for _, unitSymbol := range unitSymbols {
		index := strings.Index(str, unitSymbol)
		if index == 0 {
			doubleAndUnit.Value = 0
			doubleAndUnit.Unit = strings.TrimSpace(str)
			return doubleAndUnit
		}
		if index > 0 {
			numericPart := str[:index]
			if value, err := strconv.ParseFloat(strings.TrimSpace(numericPart), 64); err == nil {
				doubleAndUnit.Value = value
				doubleAndUnit.Unit = strings.TrimSpace(str[index:])
				return doubleAndUnit
			}
		}
	}
	return nil
}
