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

package logger

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
)

func TestZapLogLevel(t *testing.T) {
	level, err := zapcore.ParseLevel("warn")
	if err != nil {
		t.Errorf("ParseLevel error %v", err)
	}
	zc := zap.NewDevelopmentConfig()
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(zc.EncoderConfig), zapcore.AddSync(os.Stdout), zap.NewAtomicLevelAt(level))
	zapLogger := zap.New(core, zap.AddCaller())
	log := zapLogger.Sugar()
	log.Info("ok", "k1", "v1")
	log.Error(errors.New("new error"), "error")
}

func TestLogger(t *testing.T) {
	logger := NewLogger(os.Stdout, loggertype.DefaultHertzbeatLogging())
	logger.Info("kv msg", "key", "value")
	logger.Sugar().Infof("template %s %d", "string", 123)

	logger.WithName(string(loggertype.LogComponentHertzbeatCollector)).WithValues("runner", loggertype.LogComponentHertzbeatCollector).Info("msg", "k", "v")

	defaultLogger := DefaultLogger(os.Stdout, loggertype.LogLevelInfo)
	assert.NotNil(t, defaultLogger.logging)
	assert.NotNil(t, defaultLogger.sugaredLogger)

	fileLogger := FileLogger("/dev/stderr", "fl-test", loggertype.LogLevelInfo)
	assert.NotNil(t, fileLogger.logging)
	assert.NotNil(t, fileLogger.sugaredLogger)
}

func TestLoggerWithName(t *testing.T) {
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		// Restore the original stdout and close the pipe
		os.Stdout = originalStdout
		err := w.Close()
		require.NoError(t, err)
	}()

	config := loggertype.DefaultHertzbeatLogging()
	config.Level[loggertype.LogComponentHertzbeatCollector] = loggertype.LogLevelDebug

	logger := NewLogger(os.Stdout, config).WithName(string(loggertype.LogComponentHertzbeatCollector))
	logger.Info("info message")
	logger.Sugar().Debugf("debug message")

	// Read from the pipe (captured stdout)
	outputBytes := make([]byte, 200)
	_, err := r.Read(outputBytes)
	require.NoError(t, err)
	capturedOutput := string(outputBytes)
	assert.Contains(t, capturedOutput, string(loggertype.LogComponentHertzbeatCollector))
	assert.Contains(t, capturedOutput, "info message")
	assert.Contains(t, capturedOutput, "debug message")
}

func TestLoggerSugarName(t *testing.T) {
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		// Restore the original stdout and close the pipe
		os.Stdout = originalStdout
		err := w.Close()
		require.NoError(t, err)
	}()

	const logName = "loggerName"

	config := loggertype.DefaultHertzbeatLogging()
	config.Level[logName] = loggertype.LogLevelDebug

	logger := NewLogger(os.Stdout, config).WithName(logName)

	logger.Sugar().Debugf("debugging message")

	// Read from the pipe (captured stdout)
	outputBytes := make([]byte, 200)
	_, err := r.Read(outputBytes)
	require.NoError(t, err)
	capturedOutput := string(outputBytes)
	assert.Contains(t, capturedOutput, "debugging message", logName)
}
