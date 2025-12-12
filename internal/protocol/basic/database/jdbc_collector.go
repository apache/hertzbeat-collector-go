/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package database implements JDBC (sql.DB) based database collection.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
	"golang.org/x/crypto/ssh"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/job/collect/strategy"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	protocol2 "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job/protocol"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/param"
)

func init() {
	strategy.RegisterFactory("jdbc", func(logger logger.Logger) strategy.Collector {
		return NewJDBCCollector(logger)
	})
}

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
	PlatformClickHouse = "clickhouse" // Placeholder, Go driver DSN TBD
	PlatformDM         = "dm"         // Placeholder, Go driver DSN TBD

	// Response codes
	CodeSuccess       = constants.CollectSuccess
	CodeFail          = constants.CollectFail
	CodeUnReachable   = constants.CollectUnReachable
	CodeUnConnectable = constants.CollectUnConnectable

	// PostgreSQL specific SQLState for unreachable
	// See: https://www.postgresql.org/docs/current/errcodes-appendix.html#ERRCODES-TABLE
	pqUnreachableCode = "08001" // SQLCLIENT_UNABLE_TO_ESTABLISH_SQLCONNECTION
)

// --- Security Check Constants ---
// Ported from Java version (JdbcCommonCollect)

var (
	// VULNERABLE_KEYWORDS Dangerous parameters prohibited in the URL
	vulnerableKeywords = []string{"allowLoadLocalInfile", "allowLoadLocalInfileInPath", "useLocalInfile"}

	// BLACK_LIST Dangerous keywords prohibited in the URL
	blackList = []string{
		// Dangerous SQL commands
		"create trigger", "create alias", "runscript from", "shutdown", "drop table",
		"drop database", "create function", "alter system", "grant all", "revoke all",

		// File IO
		"allowloadlocalinfile", "allowloadlocalinfileinpath", "uselocalinfile",

		// Code execution
		"init=", "javaobjectserializer=", "runscript", "serverstatusdiffinterceptor",
		"queryinterceptors=", "statementinterceptors=", "exceptioninterceptors=",
		"xp_cmdshell", "create function", "dbms_java", "sp_sysexecute", "load_file",

		// Multiple statements
		"allowmultiqueries",

		// Deserialization
		"autodeserialize", "detectcustomcollations",
	}

	// universalBypassPatterns Universal bypass patterns (simplified version, focusing on whitespace bypass)
	universalBypassPatterns = []string{
		`(?i)create\s+trigger`,
		`(?i)create\s+function`,
		`(?i)drop\s+table`,
		`(?i)drop\s+database`,
		`(?i)run\s+script`,
		`(?i)alter\s+system`,
		`(?i)grant\s+all`,
		`(?i)revoke\s+all`,
		`(?i)xp\s*cmdshell`,
		`(?i)load\s+file`,
	}

	// platformBypassPatterns Platform-specific bypass patterns (simplified version)
	platformBypassPatterns = map[string][]string{
		"mysql": {
			`(?i)allow\s+load\s+local\s+infile`,
			`(?i)allow\s+multi\s+queries`,
			`(?i)query\s+interceptors`,
			`(?i)auto\s+deserialize`,
		},
		"mariadb": {
			`(?i)allow\s+load\s+local\s+infile`,
			`(?i)allow\s+multi\s+queries`,
			`(?i)query\s+interceptors`,
			`(?i)auto\s+deserialize`,
		},
		"postgresql": {
			`(?i)socket\s+factory`,
			`(?i)logger\s+file`,
			`(?i)logger\s+level`,
		},
	}

	// Compiled regular expressions
	compiledUniversalBypassPatterns []*regexp.Regexp
	compiledPlatformBypassPatterns  map[string][]*regexp.Regexp

	// Used to clean invalid characters from URL
	invalidCharRegex = regexp.MustCompile(`[\x00-\x1F\x7F\xA0]`)

	// Ensures regex patterns are compiled only once
	compilePatternsOnce sync.Once
)

// initPatterns compiles all security check regular expressions
func initPatterns() {
	compiledUniversalBypassPatterns = make([]*regexp.Regexp, len(universalBypassPatterns))
	for i, pattern := range universalBypassPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Failed to compile universal regex pattern %q: %v", pattern, err)
		}
		compiledUniversalBypassPatterns[i] = compiled
	}

	compiledPlatformBypassPatterns = make(map[string][]*regexp.Regexp)
	for platform, patterns := range platformBypassPatterns {
		compiledPlatformBypassPatterns[platform] = make([]*regexp.Regexp, len(patterns))
		for i, pattern := range patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				log.Fatalf("Failed to compile platform regex pattern %q for %s: %v", pattern, platform, err)
			}
			compiledPlatformBypassPatterns[platform][i] = compiled
		}
	}
}

// --- SSH Tunnel ---

// sshTunnelHelper manages an SSH tunnel connection
type sshTunnelHelper struct {
	client   *ssh.Client
	listener net.Listener
	logger   logger.Logger
}

// startSSHTunnel starts an SSH tunnel
// It connects to the SSH bastion, listens on a local random port,
// and forwards traffic to the target database.
func startSSHTunnel(config *protocol2.SSHTunnel, remoteHost, remotePort string, timeout time.Duration, log logger.Logger) (*sshTunnelHelper, string, string, error) {
	sshHost := config.Host
	sshPort, err := strconv.Atoi(config.Port)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid SSH port: %v", err)
	}
	sshUser := config.Username
	sshPassword := config.Password

	// 1. Set up SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Ignore host key validation
		Timeout:         timeout,
	}

	sshAddr := fmt.Sprintf("%s:%d", sshHost, sshPort)
	log.V(1).Info("starting SSH tunnel", "sshAddr", sshAddr, "remoteHost", remoteHost, "remotePort", remotePort)

	// 2. Connect to the SSH bastion
	client, err := ssh.Dial("tcp", sshAddr, sshConfig)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to dial SSH server: %w", err)
	}

	// 3. Listen on a random local port on 127.0.0.1
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Close()
		return nil, "", "", fmt.Errorf("failed to listen on local port: %w", err)
	}

	localAddr := listener.Addr().String()
	localHost, localPort, err := net.SplitHostPort(localAddr)
	if err != nil {
		client.Close()
		listener.Close()
		return nil, "", "", fmt.Errorf("failed to parse local address: %w", err)
	}

	helper := &sshTunnelHelper{
		client:   client,
		listener: listener,
		logger:   log,
	}

	// 4. Start a goroutine to accept local connections and forward them
	go helper.runForwarder(remoteHost, remotePort)

	log.V(1).Info("SSH tunnel started", "localAddr", localAddr, "remote", fmt.Sprintf("%s:%s", remoteHost, remotePort))
	return helper, localHost, localPort, nil
}

// runForwarder loops to accept and forward local connections
func (t *sshTunnelHelper) runForwarder(remoteHost, remotePort string) {
	remoteAddr := fmt.Sprintf("%s:%s", remoteHost, remotePort)
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			// If the listener is closed, exit the loop
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			t.logger.Error(err, "failed to accept local connection")
			continue
		}

		// Start a goroutine to handle forwarding
		go t.forwardConnection(localConn, remoteAddr)
	}
}

// forwardConnection handles forwarding for a single connection
func (t *sshTunnelHelper) forwardConnection(localConn net.Conn, remoteAddr string) {
	defer localConn.Close()

	// 3. Connect to the remote database via the SSH tunnel
	remoteConn, err := t.client.Dial("tcp", remoteAddr)
	if err != nil {
		t.logger.Error(err, "failed to dial remote address via SSH", "remoteAddr", remoteAddr)
		return
	}
	defer remoteConn.Close()

	// 4. Bidirectionally copy data
	go func() {
		_, err := io.Copy(remoteConn, localConn)
		if err != nil && err != io.EOF {
			// t.logger.V(1).Error(err, "error copying local to remote")
		}
		remoteConn.Close()
		localConn.Close()
	}()
	go func() {
		_, err := io.Copy(localConn, remoteConn)
		if err != nil && err != io.EOF {
			// t.logger.V(1).Error(err, "error copying remote to local")
		}
		remoteConn.Close()
		localConn.Close()
	}()
}

// Close shuts down the SSH tunnel
func (t *sshTunnelHelper) Close() {
	t.listener.Close()
	t.client.Close()
	t.logger.V(1).Info("SSH tunnel closed")
}

// --- JDBC Collector ---

// JDBCCollector implements JDBC database collection
type JDBCCollector struct {
	logger logger.Logger
}

// NewJDBCCollector creates a new JDBC collector
func NewJDBCCollector(logger logger.Logger) *JDBCCollector {
	// Ensure security patterns are compiled only once
	compilePatternsOnce.Do(initPatterns)
	return &JDBCCollector{
		logger: logger.WithName("jdbc-collector"),
	}
}

// extractJDBCConfig extracts JDBC configuration from interface{} type
func extractJDBCConfig(jdbcInterface interface{}) (*protocol2.JDBCProtocol, error) {
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

	// 1. Validate SSH tunnel configuration
	if err := jc.checkTunnelParam(jdbcConfig.SSHTunnel); err != nil {
		return err
	}

	// 2. Validate URL
	if jdbcConfig.URL != "" {
		// If a full URL is provided, perform strict security checks
		if err := validateURL(jdbcConfig.URL, jdbcConfig.Platform); err != nil {
			return err
		}
	} else {
		// If URL is not provided, check basic host/port/platform
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

	// 3. Validate platform (Platform can be empty if URL is provided,
	//    but required if URL needs to be auto-built)
	if jdbcConfig.Platform != "" {
		switch jdbcConfig.Platform {
		case PlatformMySQL, PlatformMariaDB, PlatformPostgreSQL, PlatformSQLServer, PlatformOracle:
			// Valid platforms
		case PlatformClickHouse, PlatformDM:
			// Platforms not yet fully supported
			jc.logger.Info("warning: platform %s is not fully supported in Go collector yet", jdbcConfig.Platform)
		default:
			return fmt.Errorf("unsupported database platform: %s", jdbcConfig.Platform)
		}
	}

	// 4. Validate query type
	if jdbcConfig.QueryType == "" {
		// Default to oneRow
		jdbcConfig.QueryType = QueryTypeOneRow
	}
	switch jdbcConfig.QueryType {
	case QueryTypeOneRow, QueryTypeMultiRow, QueryTypeColumns, QueryTypeRunScript:
		// Valid query types
	default:
		return fmt.Errorf("unsupported query type: %s", jdbcConfig.QueryType)
	}

	// 5. Validate SQL (The SQL field for runScript contains the script content and must also be checked)
	if jdbcConfig.SQL == "" {
		return fmt.Errorf("SQL/Script content is required for query type: %s", jdbcConfig.QueryType)
	}

	return nil
}

// checkTunnelParam validates SSH tunnel configuration
func (jc *JDBCCollector) checkTunnelParam(config *protocol2.SSHTunnel) error {
	if config == nil {
		return nil
	}
	enable, _ := strconv.ParseBool(config.Enable)
	if !enable {
		return nil
	}
	if config.Host == "" || config.Port == "" || config.Username == "" {
		return fmt.Errorf("ssh tunnel host, port, and username are required when enabled")
	}
	return nil
}

// Collect performs JDBC metrics collection
func (jc *JDBCCollector) Collect(metrics *jobtypes.Metrics) *jobtypes.CollectRepMetricsData {
	startTime := time.Now()

	// 1. Extract configuration
	jdbcConfig, err := extractJDBCConfig(metrics.JDBC)
	if err != nil {
		jc.logger.Error(err, "failed to extract JDBC config")
		return jc.createFailResponse(metrics, CodeFail, fmt.Sprintf("JDBC config error: %v", err))
	}

	// 2. Get timeout
	timeout := jc.getTimeout(jdbcConfig.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 3. Set up SSH tunnel (if needed)
	localHost := jdbcConfig.Host
	localPort := jdbcConfig.Port

	if jdbcConfig.SSHTunnel != nil {
		enableTunnel, _ := strconv.ParseBool(jdbcConfig.SSHTunnel.Enable)
		if enableTunnel {
			tunnel, lHost, lPort, err := startSSHTunnel(jdbcConfig.SSHTunnel, jdbcConfig.Host, jdbcConfig.Port, timeout, jc.logger)
			if err != nil {
				jc.logger.Error(err, "failed to start SSH tunnel")
				return jc.createFailResponse(metrics, CodeUnConnectable, fmt.Sprintf("SSH tunnel error: %v", err))
			}
			defer tunnel.Close()
			localHost = lHost
			localPort = lPort
		}
	}

	// 4. Construct database URL
	databaseURL, err := jc.constructDatabaseURL(jdbcConfig, localHost, localPort)
	if err != nil {
		jc.logger.Error(err, "failed to construct database URL")
		return jc.createFailResponse(metrics, CodeFail, fmt.Sprintf("Database URL error: %v", err))
	}

	// 5. Create database connection
	db, err := jc.getConnection(ctx, databaseURL, jdbcConfig.Platform, timeout)
	if err != nil {
		jc.logger.Error(err, "failed to connect to database")
		// Ping failed inside getConnection, return UnConnectable
		return jc.createFailResponse(metrics, CodeUnConnectable, fmt.Sprintf("Connection error: %v", err))
	}
	defer db.Close()

	// 6. Execute query
	response := jc.createSuccessResponse(metrics)
	switch jdbcConfig.QueryType {
	case QueryTypeOneRow:
		err = jc.queryOneRow(ctx, db, jdbcConfig.SQL, metrics.AliasFields, response)
	case QueryTypeMultiRow:
		err = jc.queryMultiRow(ctx, db, jdbcConfig.SQL, metrics.AliasFields, response)
	case QueryTypeColumns:
		err = jc.queryColumns(ctx, db, jdbcConfig.SQL, metrics.AliasFields, response)
	case QueryTypeRunScript:
		err = jc.runScript(ctx, db, jdbcConfig.SQL, response)
	default:
		err = fmt.Errorf("unsupported query type: %s", jdbcConfig.QueryType)
	}

	// 7. Process query results
	duration := time.Since(startTime)
	if err != nil {
		jc.logger.Error(err, "query execution failed", "queryType", jdbcConfig.QueryType)
		// Check for specific errors
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == pqUnreachableCode {
				return jc.createFailResponse(metrics, CodeUnReachable, fmt.Sprintf("Query error: %v (Code: %s)", err, pqErr.Code))
			}
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return jc.createFailResponse(metrics, CodeUnReachable, fmt.Sprintf("Query timeout: %v", err))
		}
		return jc.createFailResponse(metrics, CodeFail, fmt.Sprintf("Query error: %v", err))
	}

	// Inject response time (if response_time field exists)
	jc.addResponseTime(response, metrics.AliasFields, duration)

	jc.logger.V(1).Info("JDBC collection completed", "duration", duration, "rowCount", len(response.Values))
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

	// Try parsing as Go duration string (e.g., "10s", "5m", "500ms")
	if duration, err := time.ParseDuration(timeoutStr); err == nil {
		return duration
	}

	// Try parsing as Java version's milliseconds
	if timeout, err := strconv.Atoi(timeoutStr); err == nil {
		return time.Duration(timeout) * time.Millisecond
	}

	return 30 * time.Second // fallback to default
}

// recursiveDecode recursively decodes a URL (max 5 times)
func recursiveDecode(s string) (string, error) {
	prev := s
	for i := 0; i < 5; i++ {
		decoded, err := url.QueryUnescape(prev)
		if err != nil {
			// If decoding fails, return the last successfully decoded version
			return prev, err
		}
		if decoded == prev {
			// If no change after decoding, decoding is complete
			return decoded, nil
		}
		prev = decoded
	}
	return prev, nil
}

// validateURL performs strict URL security validation
func validateURL(rawURL string, platform string) error {
	// 1. Length limit
	if len(rawURL) > 2048 {
		return fmt.Errorf("JDBC URL length exceeds maximum limit of 2048 characters")
	}

	// 2. Clean invisible characters
	cleanedUrl := invalidCharRegex.ReplaceAllString(rawURL, "")

	// 3. Recursive decoding
	decodedUrl, err := recursiveDecode(cleanedUrl)
	if err != nil {
		// We can't use jc.logger here as it's a static function
		// log.Printf("URL decoding error: %v", err)
		// Continue checking with the pre-decoded string even if decoding fails
		decodedUrl = cleanedUrl
	}

	urlLowerCase := strings.ToLower(decodedUrl)

	// 4. Check VULNERABLE_KEYWORDS
	for _, keyword := range vulnerableKeywords {
		if strings.Contains(urlLowerCase, keyword) {
			return fmt.Errorf("JDBC URL prohibit contains vulnerable param %s", keyword)
		}
	}

	// 5. Check BLACK_LIST
	for _, keyword := range blackList {
		if strings.Contains(urlLowerCase, keyword) {
			return fmt.Errorf("invalid JDBC URL: contains potentially malicious parameter: %s", keyword)
		}
	}

	// 6. Check JNDI/Deserialization (simplified)
	if strings.Contains(urlLowerCase, "jndi:") || strings.Contains(urlLowerCase, "ldap:") || strings.Contains(urlLowerCase, "rmi:") {
		return fmt.Errorf("invalid JDBC URL: contains potentially malicious JNDI or deserialization parameter")
	}

	// 7. Check universal bypass patterns
	for _, pattern := range compiledUniversalBypassPatterns {
		if pattern.MatchString(urlLowerCase) {
			return fmt.Errorf("invalid JDBC URL: contains potentially malicious bypass pattern: %s", pattern.String())
		}
	}

	// 8. Check platform-specific bypass patterns
	if platform != "" {
		if patterns, ok := compiledPlatformBypassPatterns[strings.ToLower(platform)]; ok {
			for _, pattern := range patterns {
				if pattern.MatchString(urlLowerCase) {
					return fmt.Errorf("invalid %s JDBC URL: contains potentially malicious bypass pattern: %s", platform, pattern.String())
				}
			}
		}
	}

	return nil
}

// constructDatabaseURL constructs the database connection URL/DSN
func (jc *JDBCCollector) constructDatabaseURL(jdbc *protocol2.JDBCProtocol, host, port string) (string, error) {
	// 1. If user provided a full URL, use it (already validated in PreCheck)
	if jdbc.URL != "" {
		// Validate again to prevent bypassing PreCheck
		if err := validateURL(jdbc.URL, jdbc.Platform); err != nil {
			return "", err
		}
		return jdbc.URL, nil
	}

	// 2. Auto-build DSN based on platform
	database := jdbc.Database
	username := jdbc.Username
	password := jdbc.Password

	switch jdbc.Platform {
	case PlatformMySQL, PlatformMariaDB:
		// MySQL DSN: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&timeout=30s&readTimeout=30s&writeTimeout=30s",
			username, password, host, port, database)
		// runScript requires multiStatements support
		if jdbc.QueryType == QueryTypeRunScript {
			dsn += "&multiStatements=true"
		}
		return dsn, nil
	case PlatformPostgreSQL:
		// PostgreSQL DSN: postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&connect_timeout=30",
			username, password, host, port, database), nil
	case PlatformSQLServer:
		// SQL Server DSN: sqlserver://username:password@host:port?database=dbname&...
		return fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s&trustServerCertificate=true&encrypt=disable&connection+timeout=30",
			username, password, host, port, database), nil
	case PlatformOracle:
		// go-ora DSN: oracle://user:pass@host:port/service_name
		// Note: Oracle's 'database' field usually corresponds to 'Service Name'
		return fmt.Sprintf("oracle://%s:%s@%s:%s/%s",
			username, password, host, port, database), nil
	default:
		return "", fmt.Errorf("unsupported database platform for URL construction: %s", jdbc.Platform)
	}
}

// getConnection creates a database connection with timeout
func (jc *JDBCCollector) getConnection(ctx context.Context, databaseURL string, platform string, timeout time.Duration) (*sql.DB, error) {
	// 1. Select driver based on platform
	var driverName string
	switch platform {
	case PlatformMySQL, PlatformMariaDB:
		driverName = "mysql"
	case PlatformPostgreSQL:
		driverName = "postgres"
	case PlatformSQLServer:
		driverName = "sqlserver"
	case PlatformOracle:
		driverName = "oracle"
	default:
		// Try to infer from URL (for user-customized URLs)
		if strings.HasPrefix(databaseURL, "postgres:") {
			driverName = "postgres"
		} else if strings.HasPrefix(databaseURL, "sqlserver:") {
			driverName = "sqlserver"
		} else if strings.HasPrefix(databaseURL, "oracle:") {
			driverName = "oracle"
		} else if strings.Contains(databaseURL, "@tcp(") { // MySQL DSN format
			driverName = "mysql"
		} else {
			return nil, fmt.Errorf("unsupported platform or ambiguous URL: %s", platform)
		}
	}

	// 2. Open database connection
	db, err := sql.Open(driverName, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 3. Set connection pool parameters
	db.SetConnMaxLifetime(timeout)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(30 * time.Second)

	// 4. Test connection (Ping)
	// Use the incoming context which has the overall timeout
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// convertRowValues converts raw database values (interface{}) to a string slice
// Deprecated: Using sql.NullString is preferred.
func (jc *JDBCCollector) convertRowValues(rawValues []interface{}, columns []sql.NullString) []string {
	stringValues := make([]string, len(rawValues))
	for i, rawValue := range rawValues {
		if rawValue == nil {
			stringValues[i] = constants.NULL_VALUE // Use "NULL" defined in Java version
			continue
		}

		// Try using NullString (although Scan already handled it)
		if columns[i].Valid {
			stringValues[i] = columns[i].String
		} else if rawValue != nil {
			// Fallback to fmt.Sprintf
			switch v := rawValue.(type) {
			case []byte:
				stringValues[i] = string(v) // Handle byte arrays
			case time.Time:
				stringValues[i] = v.Format(time.RFC3339) // Unify time format
			default:
				stringValues[i] = fmt.Sprintf("%v", v)
			}
		} else {
			stringValues[i] = constants.NULL_VALUE
		}
	}
	return stringValues
}

// queryOneRow executes a query and returns only the first row
func (jc *JDBCCollector) queryOneRow(ctx context.Context, db *sql.DB, sqlQuery string, aliasFields []string, response *jobtypes.CollectRepMetricsData) error {
	// Java version uses setMaxRows(1), which Go's standard library doesn't support.
	// We simulate this by only reading the first row.
	rows, err := db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Build fields (using aliasFields or original column names)
	fieldNames := jc.buildFields(response, aliasFields, columns)

	// Get the first row
	if rows.Next() {
		// Use sql.NullString to handle NULL values correctly
		values := make([]interface{}, len(fieldNames))
		scanValues := make([]sql.NullString, len(fieldNames))
		for i := range values {
			values[i] = &scanValues[i]
		}

		if err := rows.Scan(values...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert
		stringValues := make([]string, len(fieldNames))
		for i, sv := range scanValues {
			if sv.Valid {
				stringValues[i] = sv.String
			} else {
				stringValues[i] = constants.NULL_VALUE
			}
		}

		response.Values = append(response.Values, jobtypes.ValueRow{
			Columns: stringValues,
		})
	}

	return rows.Err()
}

// queryMultiRow executes a query and returns multiple rows
func (jc *JDBCCollector) queryMultiRow(ctx context.Context, db *sql.DB, sqlQuery string, aliasFields []string, response *jobtypes.CollectRepMetricsData) error {
	rows, err := db.QueryContext(ctx, sqlQuery)
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
	fieldNames := jc.buildFields(response, aliasFields, columns)

	// Get all rows
	for rows.Next() {
		values := make([]interface{}, len(fieldNames))
		scanValues := make([]sql.NullString, len(fieldNames))
		for i := range values {
			values[i] = &scanValues[i]
		}

		if err := rows.Scan(values...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert
		stringValues := make([]string, len(fieldNames))
		for i, sv := range scanValues {
			if sv.Valid {
				stringValues[i] = sv.String
			} else {
				stringValues[i] = constants.NULL_VALUE
			}
		}

		response.Values = append(response.Values, jobtypes.ValueRow{
			Columns: stringValues,
		})
	}

	return rows.Err()
}

// queryColumns executes a query and matches two columns (similar to the Java implementation)
func (jc *JDBCCollector) queryColumns(ctx context.Context, db *sql.DB, sqlQuery string, aliasFields []string, response *jobtypes.CollectRepMetricsData) error {
	rows, err := db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names (ensure at least two columns)
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}
	if len(columns) < 2 {
		return fmt.Errorf("columns query type requires at least 2 columns, got %d", len(columns))
	}

	// Build fields (must use aliasFields)
	if len(aliasFields) == 0 {
		return fmt.Errorf("columns query type requires aliasFields to be defined")
	}
	fieldNames := jc.buildFields(response, aliasFields, nil)

	// Create K-V map
	keyValueMap := make(map[string]string)
	for rows.Next() {
		var key, value sql.NullString

		// Scan only the first two columns
		// Handle varying number of columns in result set gracefully
		scanArgs := make([]interface{}, len(columns))
		scanArgs[0] = &key
		scanArgs[1] = &value
		for i := 2; i < len(columns); i++ {
			scanArgs[i] = new(sql.RawBytes) // Discard extra columns
		}

		if err := rows.Scan(scanArgs...); err != nil {
			// If scan fails (e.g., column mismatch), log warning and continue
			jc.logger.Error(err, "failed to scan row in columns query")
			continue
		}

		if key.Valid {
			// Corresponds to Java: values.put(resultSet.getString(1).toLowerCase().trim(), resultSet.getString(2));
			keyString := strings.ToLower(strings.TrimSpace(key.String))
			if value.Valid {
				keyValueMap[keyString] = value.String
			} else {
				keyValueMap[keyString] = constants.NULL_VALUE
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Map to the result row
	valueRow := jobtypes.ValueRow{
		Columns: make([]string, len(fieldNames)),
	}
	for i, fieldName := range fieldNames {
		// Corresponds to Java: String value = values.get(column.toLowerCase());
		if value, exists := keyValueMap[strings.ToLower(fieldName)]; exists {
			valueRow.Columns[i] = value
		} else {
			// Check if it's response_time
			if fieldName == constants.RESPONSE_TIME {
				// This value will be populated at the end of Collect by addResponseTime
				valueRow.Columns[i] = "0"
			} else {
				valueRow.Columns[i] = constants.NULL_VALUE
			}
		}
	}

	response.Values = append(response.Values, valueRow)
	return nil
}

// runScript executes a SQL script (as content, not file path)
func (jc *JDBCCollector) runScript(ctx context.Context, db *sql.DB, scriptContent string, response *jobtypes.CollectRepMetricsData) error {
	// The Java version uses ScriptUtils to execute a file; here we execute the content from the SQL field.
	// Note: DSN must have multiStatements=true enabled (e.g., for MySQL).
	result, err := db.ExecContext(ctx, scriptContent)
	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	// Script execution usually doesn't return rows, but we can return the affected rows.
	// We define a "rows_affected" field.
	jc.buildFields(response, nil, []string{"rows_affected"})
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Some drivers (like PostgreSQL) might not support RowsAffected for scripts
		jc.logger.V(1).Info("could not get rows affected from script execution", "error", err)
		response.Values = append(response.Values, jobtypes.ValueRow{
			Columns: []string{"0"},
		})
	} else {
		response.Values = append(response.Values, jobtypes.ValueRow{
			Columns: []string{strconv.FormatInt(rowsAffected, 10)},
		})
	}

	return nil
}

// buildFields populates the response.Fields slice based on aliasFields or columns
// It returns the list of field names that should be used for value mapping.
func (jc *JDBCCollector) buildFields(response *jobtypes.CollectRepMetricsData, aliasFields []string, columns []string) []string {
	var fieldNames []string
	if len(aliasFields) > 0 {
		fieldNames = aliasFields
	} else {
		fieldNames = columns
	}

	response.Fields = make([]jobtypes.Field, len(fieldNames))
	for i, fieldName := range fieldNames {
		response.Fields[i] = jobtypes.Field{
			Field:    fieldName,
			Type:     constants.TYPE_STRING,
			Label:    false,
			Unit:     "",
			Instance: i == 0, // Default first as instance
		}
	}
	return fieldNames
}

// addResponseTime checks if response_time is requested and adds it to all value rows
func (jc *JDBCCollector) addResponseTime(response *jobtypes.CollectRepMetricsData, aliasFields []string, duration time.Duration) {
	rtIndex := -1
	for i, field := range aliasFields {
		if field == constants.RESPONSE_TIME {
			rtIndex = i
			break
		}
	}

	if rtIndex == -1 {
		return
	}

	responseTimeMs := strconv.FormatInt(duration.Milliseconds(), 10)
	for i := range response.Values {
		if len(response.Values[i].Columns) > rtIndex {
			response.Values[i].Columns[rtIndex] = responseTimeMs
		}
	}
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
		Code:      CodeSuccess,
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
