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

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/basic"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
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
	*basic.BaseCollector
}

// NewJDBCCollector creates a new JDBC collector
func NewJDBCCollector(logger logger.Logger) *JDBCCollector {
	return &JDBCCollector{
		BaseCollector: basic.NewBaseCollector(logger.WithName("jdbc-collector")),
	}
}

// PreCheck validates the JDBC metrics configuration
func (jc *JDBCCollector) PreCheck(metrics *jobtypes.Metrics) error {
	if metrics == nil {
		return fmt.Errorf("metrics is nil")
	}

	if metrics.JDBC == nil {
		return fmt.Errorf("JDBC protocol configuration is required")
	}

	jdbc := metrics.JDBC

	// Validate required fields when URL is not provided
	if jdbc.URL == "" {
		if jdbc.Host == "" {
			return fmt.Errorf("host is required when URL is not provided")
		}
		if jdbc.Port == "" {
			return fmt.Errorf("port is required when URL is not provided")
		}
		if jdbc.Platform == "" {
			return fmt.Errorf("platform is required when URL is not provided")
		}
	}

	// Validate platform
	if jdbc.Platform != "" {
		switch jdbc.Platform {
		case PlatformMySQL, PlatformMariaDB, PlatformPostgreSQL, PlatformSQLServer, PlatformOracle:
			// Valid platforms
		default:
			return fmt.Errorf("unsupported database platform: %s", jdbc.Platform)
		}
	}

	// Validate query type
	if jdbc.QueryType != "" {
		switch jdbc.QueryType {
		case QueryTypeOneRow, QueryTypeMultiRow, QueryTypeColumns, QueryTypeRunScript:
			// Valid query types
		default:
			return fmt.Errorf("unsupported query type: %s", jdbc.QueryType)
		}
	}

	// Validate SQL for most query types
	if jdbc.QueryType != QueryTypeRunScript && jdbc.SQL == "" {
		return fmt.Errorf("SQL is required for query type: %s", jdbc.QueryType)
	}

	return nil
}

// Collect performs JDBC metrics collection
func (jc *JDBCCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	startTime := time.Now()
	jdbc := metrics.JDBC

	jc.Logger.Info("starting JDBC collection",
		"host", jdbc.Host,
		"port", jdbc.Port,
		"platform", jdbc.Platform,
		"database", jdbc.Database,
		"queryType", jdbc.QueryType)

	// Get timeout
	timeout := jc.getTimeout(jdbc.Timeout)

	// Get database URL
	databaseURL, err := jc.constructDatabaseURL(jdbc)
	if err != nil {
		jc.Logger.Error(err, "failed to construct database URL")
		return jc.CreateFailResponse(metrics, CodeFail, fmt.Sprintf("Database URL error: %v", err))
	}

	// Create database connection
	db, err := jc.getConnection(databaseURL, jdbc.Username, jdbc.Password, timeout)
	if err != nil {
		jc.Logger.Error(err, "failed to connect to database")
		return jc.CreateFailResponse(metrics, CodeUnConnectable, fmt.Sprintf("Connection error: %v", err))
	}
	defer db.Close()

	// Execute query based on type with context
	response := jc.CreateSuccessResponse(metrics)

	switch jdbc.QueryType {
	case QueryTypeOneRow:
		err = jc.queryOneRow(db, jdbc.SQL, metrics.Aliasfields, response)
	case QueryTypeMultiRow:
		err = jc.queryMultiRow(db, jdbc.SQL, metrics.Aliasfields, response)
	case QueryTypeColumns:
		err = jc.queryColumns(db, jdbc.SQL, metrics.Aliasfields, response)
	case QueryTypeRunScript:
		err = jc.runScript(db, jdbc.SQL, response)
	default:
		err = fmt.Errorf("unsupported query type: %s", jdbc.QueryType)
	}

	if err != nil {
		jc.Logger.Error(err, "query execution failed", "queryType", jdbc.QueryType)
		return jc.CreateFailResponse(metrics, CodeFail, fmt.Sprintf("Query error: %v", err))
	}

	duration := time.Since(startTime)
	jc.Logger.Info("JDBC collection completed",
		"duration", duration,
		"rowCount", len(response.Values))

	return response
}

// SupportProtocol returns the protocol this collector supports
func (jc *JDBCCollector) SupportProtocol() string {
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

	// If it's a pure number, treat it as seconds (more reasonable for database connections)
	if timeout, err := strconv.Atoi(timeoutStr); err == nil {
		return time.Duration(timeout) * time.Second
	}

	return 30 * time.Second // fallback to default
}

// constructDatabaseURL constructs the database connection URL
func (jc *JDBCCollector) constructDatabaseURL(jdbc *jobtypes.JDBCProtocol) (string, error) {
	// If URL is provided directly, use it
	if jdbc.URL != "" {
		return jdbc.URL, nil
	}

	// Construct URL based on platform
	host := jdbc.Host
	port := jdbc.Port
	database := jdbc.Database

	switch jdbc.Platform {
	case PlatformMySQL, PlatformMariaDB:
		return fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
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
	if strings.HasPrefix(databaseURL, "mysql://") {
		driverName = "mysql"
		// Remove mysql:// prefix for the driver
		databaseURL = strings.TrimPrefix(databaseURL, "mysql://")
	} else if strings.HasPrefix(databaseURL, "postgres://") {
		driverName = "postgres"
	} else if strings.HasPrefix(databaseURL, "sqlserver://") {
		driverName = "sqlserver"
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
	jc.Logger.Info("script execution requested but disabled for security", "scriptPath", scriptPath)
	return nil
}
