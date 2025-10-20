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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// CreateSSHClientConfig creates SSH client configuration based on the protocol config
func CreateSSHClientConfig(config *jobtypes.SSHProtocol, logger logger.Logger) (*ssh.ClientConfig, error) {
	clientConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, you should verify host keys
	}

	// Configure authentication
	if config.PrivateKey != "" && strings.TrimSpace(config.PrivateKey) != "" {
		logger.Info("Using private key authentication")
		// Use private key authentication
		signer, err := ParsePrivateKey(config.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		clientConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else if config.Password != "" {
		logger.Info("Using password authentication")
		// Use password authentication
		clientConfig.Auth = []ssh.AuthMethod{ssh.Password(config.Password)}
	} else {
		return nil, fmt.Errorf("either password or private key must be provided")
	}

	return clientConfig, nil
}

// ParsePrivateKey parses a private key string and returns an ssh.Signer
func ParsePrivateKey(privateKeyStr string) (ssh.Signer, error) {
	// Decode the PEM block
	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}

	// Parse the private key
	var privateKey interface{}
	var err error

	switch block.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		privateKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		privateKey, err = x509.ParseECPrivateKey(block.Bytes)
	case "OPENSSH PRIVATE KEY":
		// Use ssh.ParseRawPrivateKey for OpenSSH format
		privateKey, err = ssh.ParseRawPrivateKey([]byte(privateKeyStr))
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create SSH signer
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from private key: %w", err)
	}

	return signer, nil
}

// createProxyClientConfig creates SSH client configuration for proxy server
func createProxyClientConfig(config *jobtypes.SSHProtocol, logger logger.Logger) (*ssh.ClientConfig, error) {
	clientConfig := &ssh.ClientConfig{
		User:            config.ProxyUsername,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Configure authentication for proxy
	if config.ProxyPrivateKey != "" && strings.TrimSpace(config.ProxyPrivateKey) != "" {
		logger.Info("Using proxy private key authentication")
		// Use private key authentication for proxy
		signer, err := ParsePrivateKey(config.ProxyPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy private key: %w", err)
		}
		clientConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else if config.ProxyPassword != "" {
		logger.Info("Using proxy password authentication")
		// Use password authentication for proxy
		clientConfig.Auth = []ssh.AuthMethod{ssh.Password(config.ProxyPassword)}
	} else {
		return nil, fmt.Errorf("either proxy password or proxy private key must be provided")
	}

	return clientConfig, nil
}

// dialViaProxy establishes SSH connection through a proxy server
func dialViaProxy(ctx context.Context, config *jobtypes.SSHProtocol, targetConfig *ssh.ClientConfig, logger logger.Logger) (*ssh.Client, error) {
	// Create proxy client configuration
	proxyConfig, err := createProxyClientConfig(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy client config: %w", err)
	}

	// Set timeout for proxy connection
	if timeout := targetConfig.Timeout; timeout > 0 {
		proxyConfig.Timeout = timeout
	}

	// Connect to proxy server
	proxyAddress := net.JoinHostPort(config.ProxyHost, config.ProxyPort)
	logger.Info("Connecting to proxy server", "address", proxyAddress)

	// Create dialer with context for proxy connection
	dialer := &net.Dialer{}
	proxyConn, err := dialer.DialContext(ctx, "tcp", proxyAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy server %s: %w", proxyAddress, err)
	}

	// Create SSH client connection to proxy
	proxyClientConn, proxyChans, proxyReqs, err := ssh.NewClientConn(proxyConn, proxyAddress, proxyConfig)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to establish SSH connection to proxy: %w", err)
	}
	proxyClient := ssh.NewClient(proxyClientConn, proxyChans, proxyReqs)

	// Create connection to target server through proxy
	targetAddress := net.JoinHostPort(config.Host, config.Port)
	logger.Info("Connecting to target server via proxy", "target", targetAddress)

	targetConn, err := proxyClient.Dial("tcp", targetAddress)
	if err != nil {
		proxyClient.Close()
		return nil, fmt.Errorf("failed to dial target server %s via proxy: %w", targetAddress, err)
	}

	// Create SSH client connection to target server
	targetClientConn, targetChans, targetReqs, err := ssh.NewClientConn(targetConn, targetAddress, targetConfig)
	if err != nil {
		targetConn.Close()
		proxyClient.Close()
		return nil, fmt.Errorf("failed to establish SSH connection to target server: %w", err)
	}

	// Note: We need to keep the proxy client alive, so we don't close it here
	// The target client will handle the connection lifecycle
	return ssh.NewClient(targetClientConn, targetChans, targetReqs), nil
}

// DialWithContext dials SSH connection with context support
func DialWithContext(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	// Create a dialer with context
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// Create SSH client connection
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return ssh.NewClient(c, chans, reqs), nil
}

// DialWithProxy dials SSH connection through proxy with context support
func DialWithProxy(ctx context.Context, sshConfig *jobtypes.SSHProtocol, clientConfig *ssh.ClientConfig, logger logger.Logger) (*ssh.Client, error) {
	// TODO: implement Reuse connection logic

	// Check if proxy is enabled and configured
	if sshConfig.UseProxy != "true" || sshConfig.ProxyHost == "" || sshConfig.ProxyPort == "" {
		// Use direct connection if proxy is not enabled or not configured
		address := net.JoinHostPort(sshConfig.Host, sshConfig.Port)
		return DialWithContext(ctx, "tcp", address, clientConfig)
	}

	// Validate proxy configuration
	if sshConfig.ProxyUsername == "" {
		return nil, fmt.Errorf("proxy username is required when using proxy")
	}
	if sshConfig.ProxyPassword == "" && sshConfig.ProxyPrivateKey == "" {
		return nil, fmt.Errorf("either proxy password or proxy private key is required when using proxy")
	}

	logger.Info("Using proxy connection", "proxyHost", sshConfig.ProxyHost, "proxyPort", sshConfig.ProxyPort)
	return dialViaProxy(ctx, sshConfig, clientConfig, logger)
}
