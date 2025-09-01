package logger

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types"
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
	logger := NewLogger(os.Stdout, types.DefaultHertzbeatLogging())
	logger.Info("kv msg", "key", "value")
	logger.Sugar().Infof("template %s %d", "string", 123)

	logger.WithName(string(types.LogComponentHertzbeatCollector)).WithValues("runner", types.LogComponentHertzbeatCollector).Info("msg", "k", "v")

	defaultLogger := DefaultLogger(os.Stdout, types.LogLevelInfo)
	assert.NotNil(t, defaultLogger.logging)
	assert.NotNil(t, defaultLogger.sugaredLogger)

	fileLogger := FileLogger("/dev/stderr", "fl-test", types.LogLevelInfo)
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

	config := types.DefaultHertzbeatLogging()
	config.Level[types.LogComponentHertzbeatCollector] = types.LogLevelDebug

	logger := NewLogger(os.Stdout, config).WithName(string(types.LogComponentHertzbeatCollector))
	logger.Info("info message")
	logger.Sugar().Debugf("debug message")

	// Read from the pipe (captured stdout)
	outputBytes := make([]byte, 200)
	_, err := r.Read(outputBytes)
	require.NoError(t, err)
	capturedOutput := string(outputBytes)
	assert.Contains(t, capturedOutput, string(types.LogComponentHertzbeatCollector))
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

	config := types.DefaultHertzbeatLogging()
	config.Level[logName] = types.LogLevelDebug

	logger := NewLogger(os.Stdout, config).WithName(logName)

	logger.Sugar().Debugf("debugging message")

	// Read from the pipe (captured stdout)
	outputBytes := make([]byte, 200)
	_, err := r.Read(outputBytes)
	require.NoError(t, err)
	capturedOutput := string(outputBytes)
	assert.Contains(t, capturedOutput, "debugging message", logName)
}
