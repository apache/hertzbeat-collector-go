// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job/protocol"
	consts "hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/param"
	sshhelper "hertzbeat.apache.org/hertzbeat-collector-go/internal/util/ssh"
)

const (
	ProtocolSSH = "ssh"

	// Special fields
	ResponseTime = "responseTime"
	NullValue    = consts.NULL_VALUE

	// Parse types
	ParseTypeOneRow   = "oneRow"   // Each line corresponds to one field value, in order of aliasFields
	ParseTypeMultiRow = "multiRow" // First line is header with field names, subsequent lines are data rows
)

// SSHCollector implements SSH collection
type SSHCollector struct {
	logger logger.Logger
}

// NewSSHCollector creates a new SSH collector
func NewSSHCollector(logger logger.Logger) *SSHCollector {
	return &SSHCollector{
		logger: logger.WithName("ssh-collector"),
	}
}

// extractSSHConfig extracts SSH configuration from interface{} type
// This function uses the parameter replacer for consistent configuration extraction
func extractSSHConfig(sshInterface interface{}) (*protocol.SSHProtocol, error) {
	replacer := param.NewReplacer()
	return replacer.ExtractSSHConfig(sshInterface)
}

// PreCheck validates the SSH metrics configuration
func (jc *SSHCollector) PreCheck(metrics *jobtypes.Metrics) error {
	if metrics == nil {
		return fmt.Errorf("metrics is nil")
	}

	if metrics.SSH == nil {
		return fmt.Errorf("SSH protocol configuration is required")
	}

	// Extract SSH configuration
	sshConfig, err := extractSSHConfig(metrics.SSH)
	if err != nil {
		return fmt.Errorf("invalid SSH configuration: %w", err)
	}
	if sshConfig == nil {
		return fmt.Errorf("SSH configuration is required")
	}

	// Validate required fields
	if sshConfig.Host == "" {
		return fmt.Errorf("host is required")
	}
	if sshConfig.Port == "" {
		return fmt.Errorf("port is required")
	}
	if sshConfig.Username == "" {
		return fmt.Errorf("username is required")
	}
	// Either password or private key must be provided for authentication
	if sshConfig.Password == "" && sshConfig.PrivateKey == "" {
		return fmt.Errorf("either password or privateKey is required for authentication")
	}
	if sshConfig.Script == "" {
		return fmt.Errorf("script is required")
	}
	// parseScript and timeout are optional, will use defaults if not provided

	return nil
}

// Collect performs SSH metrics collection
func (sshc *SSHCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	startTime := time.Now()

	// Extract SSH configuration
	sshConfig, err := extractSSHConfig(metrics.SSH)
	if err != nil {
		sshc.logger.Error(err, "failed to extract SSH config")
		return sshc.createFailResponse(metrics, consts.CollectFail, fmt.Sprintf("SSH config error: %v", err))
	}
	if sshConfig == nil {
		return sshc.createFailResponse(metrics, consts.CollectFail, "SSH configuration is required")
	}

	// Debug level only for collection start
	sshc.logger.V(1).Info("starting SSH collection",
		"host", sshConfig.Host,
		"port", sshConfig.Port)

	// Get timeout
	timeout := sshc.getTimeout(sshConfig.Timeout)

	// Execute SSH script
	result, err := sshc.executeSSHScript(sshConfig, timeout)
	if err != nil {
		sshc.logger.Error(err, "failed to execute SSH script")
		return sshc.createFailResponse(metrics, consts.CollectUnConnectable, fmt.Sprintf("SSH execution error: %v", err))
	}

	// Calculate response time
	responseTime := time.Since(startTime).Milliseconds()

	// Create success response with result
	response := sshc.createSuccessResponseWithResult(metrics, result, responseTime)

	return response
}

// Protocol returns the protocol this collector supports
func (sshc *SSHCollector) Protocol() string {
	return ProtocolSSH
}

// getTimeout parses timeout string and returns duration
func (sshc *SSHCollector) getTimeout(timeoutStr string) time.Duration {
	if timeoutStr == "" {
		return 30 * time.Second // default timeout
	}

	// First try parsing as duration string (e.g., "10s", "5m", "500ms")
	if duration, err := time.ParseDuration(timeoutStr); err == nil {
		return duration
	}

	if timeout, err := strconv.Atoi(timeoutStr); err == nil {
		return time.Duration(timeout) * time.Millisecond
	}

	return 30 * time.Second // fallback to default
}

// executeSSHScript executes the SSH script and returns the result
func (sshc *SSHCollector) executeSSHScript(config *protocol.SSHProtocol, timeout time.Duration) (string, error) {
	// Create SSH client configuration using helper function
	clientConfig, err := sshhelper.CreateSSHClientConfig(config, sshc.logger)
	if err != nil {
		return "", fmt.Errorf("failed to create SSH client config: %w", err)
	}

	// Set timeout for the SSH connection
	clientConfig.Timeout = timeout

	// Create context with timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use helper function to dial with context (with proxy support)
	conn, err := sshhelper.DialWithProxy(ctx, config, clientConfig, sshc.logger)
	if err != nil {
		address := net.JoinHostPort(config.Host, config.Port)
		return "", fmt.Errorf("failed to connect to SSH server %s: %w", address, err)
	}
	defer conn.Close()

	// Create a session
	session, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Execute the script and capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Execute the command
	err = session.Run(config.Script)
	if err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("script execution failed: %w, stderr: %s", err, stderrStr)
		}
		return "", fmt.Errorf("script execution failed: %w", err)
	}
	sshc.logger.Info("SSH script executed successfully")
	sshc.logger.Info("SSH script output", "output", stdout.String())

	return stdout.String(), nil
}

// createFailResponse creates a failed metrics data response
func (jc *SSHCollector) createFailResponse(metrics *jobtypes.Metrics, code int, message string) *jobtypes.CollectRepMetricsData {
	return &jobtypes.CollectRepMetricsData{
		ID:        0,  // Will be set by the calling context
		MonitorID: 0,  // Will be set by the calling context
		App:       "", // Will be set by the calling context
		Metrics:   metrics.Name,
		Priority:  0,
		Time:      time.Now().UnixMilli(),
		Code:      code,
		Msg:       message,
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}
}

// createSuccessResponseWithResult creates a successful metrics data response with execution result
func (sshc *SSHCollector) createSuccessResponseWithResult(metrics *jobtypes.Metrics, result string, responseTime int64) *jobtypes.CollectRepMetricsData {
	response := &jobtypes.CollectRepMetricsData{
		ID:        0,  // Will be set by the calling context
		MonitorID: 0,  // Will be set by the calling context
		App:       "", // Will be set by the calling context
		Metrics:   metrics.Name,
		Priority:  0,
		Time:      time.Now().UnixMilli(),
		Code:      consts.CollectSuccess, // Success
		Msg:       "success",
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}

	// Extract SSH configuration to get ParseType
	sshConfig, err := extractSSHConfig(metrics.SSH)
	if err != nil {
		sshc.logger.Error(err, "failed to extract SSH config for parsing")
		return response
	}

	// Parse response data based on ParseType
	if strings.EqualFold(sshConfig.ParseType, ParseTypeOneRow) {
		sshc.parseResponseDataByOne(result, metrics.AliasFields, response, responseTime)
	} else if strings.EqualFold(sshConfig.ParseType, ParseTypeMultiRow) {
		// Default to multi-row format
		sshc.parseResponseDataByMulti(result, metrics.AliasFields, response, responseTime)
	}

	return response
}

// parseResponseDataByOne parses SSH script output in single-row format
// Each line contains a single field value, matching the order of aliasFields
func (sshc *SSHCollector) parseResponseDataByOne(result string, aliasFields []string, response *jobtypes.CollectRepMetricsData, responseTime int64) {
	lines := strings.Split(result, "\n")
	if len(lines)+1 < len(aliasFields) {
		sshc.logger.Error(fmt.Errorf("ssh response data not enough"), "result", result)
		return
	}

	valueRow := jobtypes.ValueRow{
		Columns: make([]string, 0),
	}

	aliasIndex := 0
	lineIndex := 0

	for aliasIndex < len(aliasFields) {
		alias := aliasFields[aliasIndex]

		if strings.EqualFold(alias, ResponseTime) {
			// Add response time for special field
			valueRow.Columns = append(valueRow.Columns, fmt.Sprintf("%d", responseTime))
		} else {
			if lineIndex < len(lines) {
				valueRow.Columns = append(valueRow.Columns, strings.TrimSpace(lines[lineIndex]))
			} else {
				// Add null value if line not available
				valueRow.Columns = append(valueRow.Columns, NullValue)
			}
			lineIndex++
		}
		aliasIndex++
	}

	response.Values = append(response.Values, valueRow)

	sshc.logger.Info("Parsed SSH script data (OneRow format)", "aliasFields", aliasFields, "lineCount", len(lines))

	// Set Fields based on aliasFields
	for _, alias := range aliasFields {
		field := jobtypes.Field{
			Field: alias,
			Type:  1, // Default type
			Label: false,
			Unit:  "",
		}
		response.Fields = append(response.Fields, field)
	}
}

// parseResponseDataByMulti parses SSH script output in multi-row format
// First line contains field names, subsequent lines contain data
func (sshc *SSHCollector) parseResponseDataByMulti(result string, aliasFields []string, response *jobtypes.CollectRepMetricsData, responseTime int64) {
	lines := strings.Split(result, "\n")
	if len(lines) <= 1 {
		sshc.logger.Error(fmt.Errorf("ssh response data only has header"), "result", result)
		return
	}

	// Parse the first line as field names
	fields := strings.Fields(lines[0]) // Use Fields instead of Split to handle multiple spaces
	fieldMapping := make(map[string]int, len(fields))
	for i, field := range fields {
		fieldMapping[strings.ToLower(strings.TrimSpace(field))] = i
	}

	sshc.logger.Info("Parsed SSH script header fields", "fields", fields, "fieldMapping", fieldMapping)

	// Parse subsequent lines as data rows
	dataRowCount := 0
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue // Skip empty lines
		}

		values := strings.Fields(line) // Use Fields to handle multiple spaces
		valueRow := jobtypes.ValueRow{
			Columns: make([]string, 0),
		}

		// Build columns based on aliasFields order
		for _, alias := range aliasFields {
			if strings.EqualFold(alias, ResponseTime) {
				// Add response time for special field
				valueRow.Columns = append(valueRow.Columns, fmt.Sprintf("%d", responseTime))
			} else {
				// Find the field index in the mapping
				index, exists := fieldMapping[strings.ToLower(alias)]
				if exists && index < len(values) {
					valueRow.Columns = append(valueRow.Columns, values[index])
				} else {
					// Add null value if field not found or index out of range
					valueRow.Columns = append(valueRow.Columns, NullValue)
					sshc.logger.V(1).Info("Field not found or index out of range",
						"alias", alias, "exists", exists, "index", index, "valuesLength", len(values))
				}
			}
		}

		response.Values = append(response.Values, valueRow)
		dataRowCount++
	}

	sshc.logger.Info("Parsed SSH script data", "dataRows", dataRowCount, "aliasFields", aliasFields)

	// Set Fields based on aliasFields
	for _, alias := range aliasFields {
		field := jobtypes.Field{
			Field: alias,
			Type:  1, // Default type
			Label: false,
			Unit:  "",
		}
		response.Fields = append(response.Fields, field)
	}
}
