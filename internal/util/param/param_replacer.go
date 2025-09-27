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
		if err := r.replaceMetricsParams(&clonedJob.Metrics[i], paramMap); err != nil {
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

// createParamMap creates a parameter map from configmap entries (legacy version with decryption)
// Deprecated: Use PreprocessJobPasswords + createParamMapSafe instead
func (r *Replacer) createParamMap(configmap []jobtypes.Configmap) map[string]string {
	paramMap := make(map[string]string)

	for _, config := range configmap {
		// Convert value to string based on type, handle null values as empty strings
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
			case 1: // string
				strValue = fmt.Sprintf("%v", config.Value)
			case 2: // password (encrypted)
				if encryptedValue, ok := config.Value.(string); ok {
					log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug).WithName("param-replacer")
					log.Sugar().Debugf("attempting to decrypt password: %s", encryptedValue)
					if decoded, err := r.decryptPassword(encryptedValue); err == nil {
						log.Info("password decryption successful", "length", len(decoded))
						strValue = decoded
					} else {
						log.Error(err, "password decryption failed, using original value")
						// Fallback to original value if decryption fails
						strValue = encryptedValue
					}
				} else {
					strValue = fmt.Sprintf("%v", config.Value)
				}
			default:
				strValue = fmt.Sprintf("%v", config.Value)
			}
		}
		paramMap[config.Key] = strValue
	}

	return paramMap
}

// replaceMetricsParams replaces parameters in metrics configuration
func (r *Replacer) replaceMetricsParams(metrics *jobtypes.Metrics, paramMap map[string]string) error {
	// Replace parameters in protocol-specific configurations
	// Currently only JDBC is defined as interface{} in the struct definition
	// Other protocols are concrete struct pointers and don't contain placeholders from JSON
	if metrics.JDBC != nil {
		if err := r.replaceProtocolParams(&metrics.JDBC, paramMap); err != nil {
			return fmt.Errorf("failed to replace JDBC params: %w", err)
		}
	}

	// TODO: Add other protocol configurations as they are converted to interface{} types
	// For now, other protocols (HTTP, SSH, etc.) are concrete struct pointers and
	// don't require parameter replacement

	// Replace parameters in basic metrics fields
	if err := r.replaceBasicMetricsParams(metrics, paramMap); err != nil {
		return fmt.Errorf("failed to replace basic metrics params: %w", err)
	}

	return nil
}

// replaceProtocolParams replaces parameters in any protocol configuration
func (r *Replacer) replaceProtocolParams(protocolInterface *interface{}, paramMap map[string]string) error {
	if *protocolInterface == nil {
		return nil
	}

	// Convert protocol interface{} to map for manipulation
	protocolMap, ok := (*protocolInterface).(map[string]interface{})
	if !ok {
		// If it's already processed or not a map, skip
		return nil
	}

	// Recursively replace parameters in all string values
	return r.replaceParamsInMap(protocolMap, paramMap)
}

// replaceParamsInMap recursively replaces parameters in a map structure
func (r *Replacer) replaceParamsInMap(data map[string]interface{}, paramMap map[string]string) error {
	for key, value := range data {
		switch v := value.(type) {
		case string:
			// Replace parameters in string values
			data[key] = r.replaceParamPlaceholders(v, paramMap)
		case map[string]interface{}:
			// Recursively handle nested maps
			if err := r.replaceParamsInMap(v, paramMap); err != nil {
				return fmt.Errorf("failed to replace params in nested map %s: %w", key, err)
			}
		case []interface{}:
			// Handle arrays
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
	// Replace parameters in basic string fields
	metrics.Host = r.replaceParamPlaceholders(metrics.Host, paramMap)
	metrics.Port = r.replaceParamPlaceholders(metrics.Port, paramMap)
	metrics.Timeout = r.replaceParamPlaceholders(metrics.Timeout, paramMap)
	metrics.Range = r.replaceParamPlaceholders(metrics.Range, paramMap)

	// Replace parameters in ConfigMap
	if metrics.ConfigMap != nil {
		for key, value := range metrics.ConfigMap {
			metrics.ConfigMap[key] = r.replaceParamPlaceholders(value, paramMap)
		}
	}

	return nil
}

// replaceParamPlaceholders replaces ^_^paramName^_^ placeholders with actual values
func (r *Replacer) replaceParamPlaceholders(template string, paramMap map[string]string) string {
	// Guard against empty templates to prevent strings.ReplaceAll issues
	if template == "" {
		return ""
	}

	result := template

	// Find all ^_^paramName^_^ patterns and replace them
	for paramName, paramValue := range paramMap {
		// Guard against empty parameter names
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

	// If it's a map (from JSON parsing), convert to target struct
	if protocolMap, ok := protocolInterface.(map[string]interface{}); ok {
		// Convert map to JSON and then unmarshal to target struct
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

// ExtractJDBCConfig extracts and processes JDBC configuration after parameter replacement
func (r *Replacer) ExtractJDBCConfig(jdbcInterface interface{}) (*jobtypes.JDBCProtocol, error) {
	if jdbcInterface == nil {
		return nil, nil
	}

	// If it's already a JDBCProtocol struct
	if jdbcConfig, ok := jdbcInterface.(*jobtypes.JDBCProtocol); ok {
		return jdbcConfig, nil
	}

	// Use the generic extraction method
	var jdbcConfig jobtypes.JDBCProtocol
	if err := r.ExtractProtocolConfig(jdbcInterface, &jdbcConfig); err != nil {
		return nil, fmt.Errorf("failed to extract JDBC config: %w", err)
	}

	return &jdbcConfig, nil
}

// ExtractHTTPConfig extracts and processes HTTP configuration after parameter replacement
func (r *Replacer) ExtractHTTPConfig(httpInterface interface{}) (*jobtypes.HTTPProtocol, error) {
	if httpInterface == nil {
		return nil, nil
	}

	// If it's already an HTTPProtocol struct
	if httpConfig, ok := httpInterface.(*jobtypes.HTTPProtocol); ok {
		return httpConfig, nil
	}

	// Use the generic extraction method
	var httpConfig jobtypes.HTTPProtocol
	if err := r.ExtractProtocolConfig(httpInterface, &httpConfig); err != nil {
		return nil, fmt.Errorf("failed to extract HTTP config: %w", err)
	}

	return &httpConfig, nil
}

// ExtractSSHConfig extracts and processes SSH configuration after parameter replacement
func (r *Replacer) ExtractSSHConfig(sshInterface interface{}) (*jobtypes.SSHProtocol, error) {
	if sshInterface == nil {
		return nil, nil
	}

	// If it's already an SSHProtocol struct
	if sshConfig, ok := sshInterface.(*jobtypes.SSHProtocol); ok {
		return sshConfig, nil
	}

	// Use the generic extraction method
	var sshConfig jobtypes.SSHProtocol
	if err := r.ExtractProtocolConfig(sshInterface, &sshConfig); err != nil {
		return nil, fmt.Errorf("failed to extract SSH config: %w", err)
	}

	return &sshConfig, nil
}

// decryptPassword decrypts an encrypted password using AES
// This implements the same algorithm as Java manager's AESUtil
func (r *Replacer) decryptPassword(encryptedPassword string) (string, error) {
	log := logger.DefaultLogger(os.Stdout, loggertype.LogLevelDebug).WithName("password-decrypt")

	// Use the AES utility (matches Java: AesUtil.aesDecode)
	if result, err := crypto.AesDecode(encryptedPassword); err == nil {
		log.Info("password decrypted successfully", "length", len(result))
		return result, nil
	} else {
		log.Sugar().Debugf("primary decryption failed: %v", err)
	}

	// Fallback: try with default Java key if no manager key is set
	defaultKey := "tomSun28HaHaHaHa"
	if result, err := crypto.AesDecodeWithKey(encryptedPassword, defaultKey); err == nil {
		log.Info("password decrypted with default key", "length", len(result))
		return result, nil
	}

	// If all decryption attempts fail, return original value to allow system to continue
	log.Info("all decryption attempts failed, using original value")
	return encryptedPassword, nil
}
