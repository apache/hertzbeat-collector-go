package database

import (
	"os"
	"testing"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

func TestJDBCCollector_SupportProtocol(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, types.LogLevelDebug)
	collector := NewJDBCCollector(log)

	if collector.SupportProtocol() != ProtocolJDBC {
		t.Errorf("Expected protocol %s, got %s", ProtocolJDBC, collector.SupportProtocol())
	}
}

func TestJDBCCollector_PreCheck(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, "debug")
	collector := NewJDBCCollector(log)

	// Test nil metrics
	err := collector.PreCheck(nil)
	if err == nil {
		t.Error("Expected error for nil metrics")
	}

	// Test missing JDBC configuration
	metrics := &jobtypes.Metrics{
		Name:     "test",
		Protocol: "jdbc",
	}
	err = collector.PreCheck(metrics)
	if err == nil {
		t.Error("Expected error for missing JDBC configuration")
	}

	// Test missing required fields
	metrics.JDBC = &jobtypes.JDBCProtocol{}
	err = collector.PreCheck(metrics)
	if err == nil {
		t.Error("Expected error for missing host")
	}

	// Test valid configuration
	metrics.JDBC = &jobtypes.JDBCProtocol{
		Host:      "localhost",
		Port:      "3306",
		Platform:  "mysql",
		Username:  "root",
		Password:  "password",
		Database:  "test",
		QueryType: "oneRow",
		SQL:       "SELECT 1",
	}
	err = collector.PreCheck(metrics)
	if err != nil {
		t.Errorf("Unexpected error for valid configuration: %v", err)
	}
}

func TestJDBCCollector_ConstructDatabaseURL(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, "debug")
	collector := NewJDBCCollector(log)

	testCases := []struct {
		name        string
		jdbc        *jobtypes.JDBCProtocol
		expectedURL string
		shouldError bool
	}{
		{
			name: "MySQL URL",
			jdbc: &jobtypes.JDBCProtocol{
				Host:     "localhost",
				Port:     "3306",
				Platform: "mysql",
				Username: "root",
				Password: "password",
				Database: "testdb",
			},
			expectedURL: "mysql://root:password@tcp(localhost:3306)/testdb?parseTime=true&charset=utf8mb4",
			shouldError: false,
		},
		{
			name: "PostgreSQL URL",
			jdbc: &jobtypes.JDBCProtocol{
				Host:     "localhost",
				Port:     "5432",
				Platform: "postgresql",
				Username: "postgres",
				Password: "password",
				Database: "testdb",
			},
			expectedURL: "postgres://postgres:password@localhost:5432/testdb?sslmode=disable",
			shouldError: false,
		},
		{
			name: "Direct URL",
			jdbc: &jobtypes.JDBCProtocol{
				URL: "mysql://user:pass@tcp(host:3306)/db",
			},
			expectedURL: "mysql://user:pass@tcp(host:3306)/db",
			shouldError: false,
		},
		{
			name: "Unsupported platform",
			jdbc: &jobtypes.JDBCProtocol{
				Host:     "localhost",
				Port:     "1234",
				Platform: "unsupported",
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := collector.constructDatabaseURL(tc.jdbc)

			if tc.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if url != tc.expectedURL {
				t.Errorf("Expected URL %s, got %s", tc.expectedURL, url)
			}
		})
	}
}

func TestJDBCCollector_GetTimeout(t *testing.T) {
	log := logger.DefaultLogger(os.Stdout, "debug")
	collector := NewJDBCCollector(log)

	testCases := []struct {
		input    string
		expected string
	}{
		{"", "30s"},        // default
		{"10", "10s"},      // pure number treated as seconds
		{"500ms", "500ms"}, // explicit milliseconds
		{"10s", "10s"},     // duration string
		{"5m", "5m0s"},     // minutes
		{"invalid", "30s"}, // fallback to default
	}

	for _, tc := range testCases {
		result := collector.getTimeout(tc.input)
		if result.String() != tc.expected {
			t.Errorf("Input: %s, Expected: %s, Got: %s", tc.input, tc.expected, result.String())
		}
	}
}
