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

package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job/protocol"
	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/logger"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// createTestLogger creates a test logger
func createTestLogger() logger.Logger {
	logging := loggertype.DefaultHertzbeatLogging()
	return logger.NewLogger(io.Discard, logging)
}

// generatePrivateKey generates a private key for testing
func generatePrivateKey() (string, error) {
	// Generate a new private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	// Marshal the private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)
	return string(privateKeyBytes), nil
}

func TestCreateSSHClientConfig_PasswordAuth(t *testing.T) {
	testLogger := createTestLogger()

	// Test configuration with password authentication
	config := &protocol.SSHProtocol{
		Username: "testuser",
		Password: "testpassword",
	}

	clientConfig, err := CreateSSHClientConfig(config, testLogger)

	require.NoError(t, err)
	assert.NotNil(t, clientConfig)
	assert.Equal(t, "testuser", clientConfig.User)
	assert.Len(t, clientConfig.Auth, 1)
}

func TestCreateSSHClientConfig_PrivateKeyAuth(t *testing.T) {
	testLogger := createTestLogger()

	// Generate a private key for testing
	privateKeyStr, err := generatePrivateKey()
	require.NoError(t, err)

	// Test configuration with private key authentication
	config := &protocol.SSHProtocol{
		Username:   "testuser",
		PrivateKey: privateKeyStr,
	}

	clientConfig, err := CreateSSHClientConfig(config, testLogger)

	require.NoError(t, err)
	assert.NotNil(t, clientConfig)
	assert.Equal(t, "testuser", clientConfig.User)
	assert.Len(t, clientConfig.Auth, 1)
}

func TestCreateSSHClientConfig_NoAuthMethod(t *testing.T) {
	testLogger := createTestLogger()

	// Test configuration with no authentication method
	config := &protocol.SSHProtocol{
		Username: "testuser",
	}

	_, err := CreateSSHClientConfig(config, testLogger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either password or private key must be provided")
}

func TestParsePrivateKey_RSA(t *testing.T) {
	// Generate an RSA private key for testing
	privateKeyStr, err := generatePrivateKey()
	require.NoError(t, err)

	signer, err := ParsePrivateKey(privateKeyStr)

	require.NoError(t, err)
	assert.NotNil(t, signer)
	assert.Implements(t, (*ssh.Signer)(nil), signer)
}

func TestParsePrivateKey_InvalidPEM(t *testing.T) {
	invalidKey := "invalid-pem-data"

	_, err := ParsePrivateKey(invalidKey)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block from private key")
}

func TestParsePrivateKey_UnsupportedType(t *testing.T) {
	// Create a PEM block with unsupported type
	privateKeyPEM := &pem.Block{
		Type:  "UNSUPPORTED PRIVATE KEY",
		Bytes: []byte("test-bytes"),
	}

	privateKeyStr := string(pem.EncodeToMemory(privateKeyPEM))

	_, err := ParsePrivateKey(privateKeyStr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported private key type")
}

func TestCreateProxyClientConfig_PasswordAuth(t *testing.T) {
	testLogger := createTestLogger()

	// Test configuration with proxy password authentication
	config := &protocol.SSHProtocol{
		ProxyUsername: "proxyuser",
		ProxyPassword: "proxypassword",
	}

	clientConfig, err := createProxyClientConfig(config, testLogger)

	require.NoError(t, err)
	assert.NotNil(t, clientConfig)
	assert.Equal(t, "proxyuser", clientConfig.User)
	assert.Len(t, clientConfig.Auth, 1)
}

func TestCreateProxyClientConfig_PrivateKeyAuth(t *testing.T) {
	testLogger := createTestLogger()

	// Generate a private key for testing
	privateKeyStr, err := generatePrivateKey()
	require.NoError(t, err)

	// Test configuration with proxy private key authentication
	config := &protocol.SSHProtocol{
		ProxyUsername:   "proxyuser",
		ProxyPrivateKey: privateKeyStr,
	}

	clientConfig, err := createProxyClientConfig(config, testLogger)

	require.NoError(t, err)
	assert.NotNil(t, clientConfig)
	assert.Equal(t, "proxyuser", clientConfig.User)
	assert.Len(t, clientConfig.Auth, 1)
}

func TestCreateProxyClientConfig_NoAuthMethod(t *testing.T) {
	testLogger := createTestLogger()

	// Test configuration with no proxy authentication method
	config := &protocol.SSHProtocol{
		ProxyUsername: "proxyuser",
	}

	_, err := createProxyClientConfig(config, testLogger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either proxy password or proxy private key must be provided")
}

func TestDialWithContext(t *testing.T) {
	// This is a unit test - we can't actually dial a connection
	// but we can test that the function signature and basic structure is correct

	// Test with cancelled context to verify it handles context properly
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// This should fail quickly due to cancelled context
	_, err := DialWithContext(ctx, "tcp", "localhost:22", config)

	// We expect an error due to the cancelled context
	assert.Error(t, err)
}

func TestDialWithProxy_Disabled(t *testing.T) {
	testLogger := createTestLogger()

	// Test with proxy disabled - should fall back to direct connection
	sshConfig := &protocol.SSHProtocol{
		Host:      "localhost",
		Port:      "22",
		Username:  "testuser",
		Password:  "testpassword",
		UseProxy:  "false", // Proxy disabled
		ProxyHost: "",
		ProxyPort: "",
	}

	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Test with cancelled context to verify the flow without actually connecting
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := DialWithProxy(ctx, sshConfig, clientConfig, testLogger)

	// We expect an error due to the cancelled context, not due to proxy issues
	assert.Error(t, err)
}

func TestDialWithProxy_EnabledButNotConfigured(t *testing.T) {
	testLogger := createTestLogger()

	// Test with proxy enabled but not properly configured
	sshConfig := &protocol.SSHProtocol{
		Host:      "localhost",
		Port:      "22",
		Username:  "testuser",
		Password:  "testpassword",
		UseProxy:  "true", // Proxy enabled
		ProxyHost: "",     // But not configured
		ProxyPort: "",
	}

	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Test with cancelled context to avoid actual connection attempts
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := DialWithProxy(ctx, sshConfig, clientConfig, testLogger)

	// When proxy is enabled but not configured, it should still try to use proxy
	// but will fail due to cancelled context rather than configuration issues
	assert.Error(t, err)
}

func TestDialWithProxy_EnabledAndConfigured(t *testing.T) {
	testLogger := createTestLogger()

	// Test with proxy enabled and configured but missing credentials
	sshConfig := &protocol.SSHProtocol{
		Host:          "localhost",
		Port:          "22",
		Username:      "testuser",
		Password:      "testpassword",
		UseProxy:      "true", // Proxy enabled
		ProxyHost:     "proxyhost",
		ProxyPort:     "1080",
		ProxyUsername: "", // Missing proxy username
	}

	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ctx := context.Background()

	_, err := DialWithProxy(ctx, sshConfig, clientConfig, testLogger)

	// Should fail due to missing proxy username
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proxy username is required when using proxy")
}

func TestDialWithProxy_EnabledAndFullyConfigured(t *testing.T) {
	testLogger := createTestLogger()

	// Test with proxy enabled and fully configured but missing proxy auth
	sshConfig := &protocol.SSHProtocol{
		Host:          "localhost",
		Port:          "22",
		Username:      "testuser",
		Password:      "testpassword",
		UseProxy:      "true", // Proxy enabled
		ProxyHost:     "proxyhost",
		ProxyPort:     "1080",
		ProxyUsername: "proxyuser",
		// Missing proxy password or private key
	}

	clientConfig := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ctx := context.Background()

	_, err := DialWithProxy(ctx, sshConfig, clientConfig, testLogger)

	// Should fail due to missing proxy authentication
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either proxy password or proxy private key is required when using proxy")
}
