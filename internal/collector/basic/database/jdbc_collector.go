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

package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/param"
)

const (
	ProtocolJDBC = "jdbc"

	// Query types
	QueryTypeOneRow    = "oneRow"
	QueryTypeMultiRow  = "multiRow"
	QueryTypeColumns   = "columns"
	QueryTypeRunScript = "runScript"

	// Supported platforms
	PlatformMySQL      = "mysql"
	PlatformMariaDB    = "mariadb"
	PlatformPostgreSQL = "postgresql"
	PlatformSQLServer  = "sqlserver"
	PlatformOracle     = "oracle"

	// Response codes
	CodeSuccess       = 200
	CodeFail          = 500
	CodeUnReachable   = 503
	CodeUnConnectable = 502
)

// JDBCCollector implements JDBC database collection
type JDBCCollector struct {
	logger logger.Logger
}

// NewJDBCCollector creates a new JDBC collector
func NewJDBCCollector(logger logger.Logger) *JDBCCollector {
	return &JDBCCollector{
		logger: logger.WithName("jdbc-collector"),
	}
}

// extractJDBCConfig extracts JDBC configuration from interface{} type
// This function uses the parameter replacer for consistent configuration extraction
func extractJDBCConfig(jdbcInterface interface{}) (*jobtypes.JDBCProtocol, error) {
	replacer := param.NewReplacer()
	return replacer.ExtractJDBCConfig(jdbcInterface)
}

// PreCheck validates the JDBC metrics configuration
func (jc *JDBCCollector) PreCheck(metrics *jobtypes.Metrics) error {
	if metrics == nil {
		return fmt.Errorf("metrics is nil")
	}

	if metrics.JDBC == nil {
		return fmt.Errorf("JDBC protocol configuration is required")
	}

	// Extract JDBC configuration
	jdbcConfig, err := extractJDBCConfig(metrics.JDBC)
	if err != nil {
		return fmt.Errorf("invalid JDBC configuration: %w", err)
	}
	if jdbcConfig == nil {
		return fmt.Errorf("JDBC configuration is required")
	}

	// Validate required fields when URL is not provided
	if jdbcConfig.URL == "" {
		if jdbcConfig.Host == "" {
			return fmt.Errorf("host is required when URL is not provided")
		}
		if jdbcConfig.Port == "" {
			return fmt.Errorf("port is required when URL is not provided")
		}
		if jdbcConfig.Platform == "" {
			return fmt.Errorf("platform is required when URL is not provided")
		}
	}

	// Validate platform
	if jdbcConfig.Platform != "" {
		switch jdbcConfig.Platform {
		case PlatformMySQL, PlatformMariaDB, PlatformPostgreSQL, PlatformSQLServer, PlatformOracle:
			// Valid platforms
		default:
			return fmt.Errorf("unsupported database platform: %s", jdbcConfig.Platform)
		}
	}

	// Validate query type
	if jdbcConfig.QueryType != "" {
		switch jdbcConfig.QueryType {
		case QueryTypeOneRow, QueryTypeMultiRow, QueryTypeColumns, QueryTypeRunScript:
			// Valid query types
		default:
			return fmt.Errorf("unsupported query type: %s", jdbcConfig.QueryType)
		}
	}

	// Validate SQL for most query types
	if jdbcConfig.QueryType != QueryTypeRunScript && jdbcConfig.SQL == "" {
		return fmt.Errorf("SQL is required for query type: %s", jdbcConfig.QueryType)
	}

	return nil
}

// Collect performs JDBC metrics collection
func (jc *JDBCCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	startTime := time.Now()

	// Extract JDBC configuration
	jdbcConfig, err := extractJDBCConfig(metrics.JDBC)
	if err != nil {
		jc.logger.Error(err, "failed to extract JDBC config")
		return jc.createFailResponse(metrics, CodeFail, fmt.Sprintf("JDBC config error: %v", err))
	}
	if jdbcConfig == nil {
		return jc.createFailResponse(metrics, CodeFail, "JDBC configuration is required")
	}

	// Debug level only for collection start
	jc.logger.V(1).Info("starting JDBC collection",
		"host", jdbcConfig.Host,
		"platform", jdbcConfig.Platform,
		"queryType", jdbcConfig.QueryType)

	// Get timeout
	timeout := jc.getTimeout(jdbcConfig.Timeout)

	// Get database URL
	databaseURL, err := jc.constructDatabaseURL(jdbcConfig)
	if err != nil {
		jc.logger.Error(err, "failed to construct database URL")
		return jc.createFailResponse(metrics, CodeFail, fmt.Sprintf("Database URL error: %v", err))
	}

	// Create database connection
	db, err := jc.getConnection(databaseURL, jdbcConfig.Username, jdbcConfig.Password, timeout)
	if err != nil {
		jc.logger.Error(err, "failed to connect to database")
		return jc.createFailResponse(metrics, CodeUnConnectable, fmt.Sprintf("Connection error: %v", err))
	}
	defer db.Close()

	// Execute query based on type with context
	response := jc.createSuccessResponse(metrics)

	switch jdbcConfig.QueryType {
	case QueryTypeOneRow:
		err = jc.queryOneRow(db, jdbcConfig.SQL, metrics.Aliasfields, response)
	case QueryTypeMultiRow:
		err = jc.queryMultiRow(db, jdbcConfig.SQL, metrics.Aliasfields, response)
	case QueryTypeColumns:
		err = jc.queryColumns(db, jdbcConfig.SQL, metrics.Aliasfields, response)
	case QueryTypeRunScript:
		err = jc.runScript(db, jdbcConfig.SQL, response)
	default:
		err = fmt.Errorf("unsupported query type: %s", jdbcConfig.QueryType)
	}

	if err != nil {
		jc.logger.Error(err, "query execution failed", "queryType", jdbcConfig.QueryType)
		return jc.createFailResponse(metrics, CodeFail, fmt.Sprintf("Query error: %v", err))
	}

	duration := time.Since(startTime)
	// Debug level only for successful completion
	jc.logger.V(1).Info("JDBC collection completed",
		"duration", duration,
		"rowCount", len(response.Values))

	return response
}

// Protocol returns the protocol this collector supports
func (jc *JDBCCollector) Protocol() string {
	return ProtocolJDBC
}

// getTimeout parses timeout string and returns duration
func (jc *JDBCCollector) getTimeout(timeoutStr string) time.Duration {
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

// constructDatabaseURL constructs the database connection URL
func (jc *JDBCCollector) constructDatabaseURL(jdbc *jobtypes.JDBCProtocol) (string, error) {

	// Construct URL based on platform
	host := jdbc.Host
	port := jdbc.Port
	database := jdbc.Database

	switch jdbc.Platform {
	case PlatformMySQL, PlatformMariaDB:
		// MySQL DSN format: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
			jdbc.Username, jdbc.Password, host, port, database), nil
	case PlatformPostgreSQL:
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			jdbc.Username, jdbc.Password, host, port, database), nil
	case PlatformSQLServer:
		return fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s&trustServerCertificate=true",
			jdbc.Username, jdbc.Password, host, port, database), nil
	default:
		return "", fmt.Errorf("unsupported database platform: %s", jdbc.Platform)
	}
}

// getConnection creates a database connection with timeout
func (jc *JDBCCollector) getConnection(databaseURL, username, password string, timeout time.Duration) (*sql.DB, error) {
	// Extract driver name from URL
	var driverName string
	if strings.HasPrefix(databaseURL, "postgres://") {
		driverName = "postgres"
	} else if strings.HasPrefix(databaseURL, "sqlserver://") {
		driverName = "sqlserver"
	} else if strings.Contains(databaseURL, "@tcp(") {
		// MySQL DSN format (no protocol prefix)
		driverName = "mysql"
	} else {
		return nil, fmt.Errorf("unsupported database URL format: %s", databaseURL)
	}

	// Open database connection
	db, err := sql.Open(driverName, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection timeout and pool settings
	db.SetConnMaxLifetime(timeout)
	db.SetMaxOpenConns(5)                   // Allow more concurrent connections
	db.SetMaxIdleConns(2)                   // Keep some idle connections
	db.SetConnMaxIdleTime(30 * time.Second) // Idle connection timeout

	// Test connection with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// queryOneRow executes a query and returns only the first row
func (jc *JDBCCollector) queryOneRow(db *sql.DB, sqlQuery string, aliasFields []string, response *jobtypes.CollectRepMetricsData) error {
	rows, err := db.Query(sqlQuery)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Build fields
	for i, column := range columns {
		field := jobtypes.Field{
			Field:    column,
			Type:     1, // String type
			Label:    false,
			Unit:     "",
			Instance: i == 0,
		}
		response.Fields = append(response.Fields, field)
	}

	// Get first row
	if rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert to string values
		stringValues := make([]string, len(values))
		for i, v := range values {
			if v != nil {
				stringValues[i] = fmt.Sprintf("%v", v)
			} else {
				stringValues[i] = ""
			}
		}

		response.Values = append(response.Values, jobtypes.ValueRow{
			Columns: stringValues,
		})
	}

	return nil
}

// queryMultiRow executes a query and returns multiple rows
func (jc *JDBCCollector) queryMultiRow(db *sql.DB, sqlQuery string, aliasFields []string, response *jobtypes.CollectRepMetricsData) error {
	rows, err := db.Query(sqlQuery)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Build fields
	for i, column := range columns {
		field := jobtypes.Field{
			Field:    column,
			Type:     1, // String type
			Label:    false,
			Unit:     "",
			Instance: i == 0,
		}
		response.Fields = append(response.Fields, field)
	}

	// Get all rows
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert to string values
		stringValues := make([]string, len(values))
		for i, v := range values {
			if v != nil {
				stringValues[i] = fmt.Sprintf("%v", v)
			} else {
				stringValues[i] = ""
			}
		}

		response.Values = append(response.Values, jobtypes.ValueRow{
			Columns: stringValues,
		})
	}

	return rows.Err()
}

// queryColumns executes a query and matches two columns (similar to the Java implementation)
func (jc *JDBCCollector) queryColumns(db *sql.DB, sqlQuery string, aliasFields []string, response *jobtypes.CollectRepMetricsData) error {
	rows, err := db.Query(sqlQuery)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) < 2 {
		return fmt.Errorf("columns query type requires at least 2 columns, got %d", len(columns))
	}

	// Build fields from alias fields or default columns
	var fieldNames []string
	if len(aliasFields) > 0 {
		fieldNames = aliasFields
	} else {
		fieldNames = columns
	}

	for i, fieldName := range fieldNames {
		field := jobtypes.Field{
			Field:    fieldName,
			Type:     1, // String type
			Label:    false,
			Unit:     "",
			Instance: i == 0,
		}
		response.Fields = append(response.Fields, field)
	}

	// Create value row with column mappings
	valueRow := jobtypes.ValueRow{
		Columns: make([]string, len(fieldNames)),
	}

	// Process rows to build key-value mapping
	keyValueMap := make(map[string]string)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Use first column as key, second as value
		key := ""
		value := ""
		if values[0] != nil {
			key = fmt.Sprintf("%v", values[0])
		}
		if len(values) > 1 && values[1] != nil {
			value = fmt.Sprintf("%v", values[1])
		}
		keyValueMap[key] = value
	}

	// Map values to alias fields
	for i, fieldName := range fieldNames {
		if value, exists := keyValueMap[fieldName]; exists {
			valueRow.Columns[i] = value
		} else {
			valueRow.Columns[i] = ""
		}
	}

	response.Values = append(response.Values, valueRow)
	return rows.Err()
}

// runScript executes a SQL script (placeholder implementation)
func (jc *JDBCCollector) runScript(db *sql.DB, scriptPath string, response *jobtypes.CollectRepMetricsData) error {
	// For security reasons, we'll just return success without actually running scripts
	jc.logger.Info("script execution requested but disabled for security", "scriptPath", scriptPath)
	return nil
}

// createSuccessResponse creates a successful metrics data response
func (jc *JDBCCollector) createSuccessResponse(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	return &jobtypes.CollectRepMetricsData{
		ID:        0,  // Will be set by the calling context
		MonitorID: 0,  // Will be set by the calling context
		App:       "", // Will be set by the calling context
		Metrics:   metrics.Name,
		Priority:  0,
		Time:      time.Now().UnixMilli(),
		Code:      200, // Success
		Msg:       "success",
		Fields:    make([]jobtypes.Field, 0),
		Values:    make([]jobtypes.ValueRow, 0),
		Labels:    make(map[string]string),
		Metadata:  make(map[string]string),
	}
}

// createFailResponse creates a failed metrics data response
func (jc *JDBCCollector) createFailResponse(metrics *jobtypes.Metrics, code int, message string) *jobtypes.CollectRepMetricsData {
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
