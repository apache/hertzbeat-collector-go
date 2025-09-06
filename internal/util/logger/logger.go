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
	"io"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
)

type Logger struct {
	logr.Logger
	out           io.Writer
	logging       *logger.HertzBeatLogging
	sugaredLogger *zap.SugaredLogger
}

func NewLogger(w io.Writer, logging *logger.HertzBeatLogging) Logger {

	logger := initZapLogger(w, logging, logging.Level[logger.LogComponentHertzbeatDefault])

	return Logger{
		Logger:        zapr.NewLogger(logger),
		out:           w,
		logging:       logging,
		sugaredLogger: logger.Sugar(),
	}
}

func FileLogger(file, name string, level logger.LogLevel) Logger {

	writer, err := os.OpenFile(file, os.O_WRONLY, 0o666)
	if err != nil {
		panic(err)
	}

	logging := logger.DefaultHertzbeatLogging()
	logger := initZapLogger(writer, logging, level)

	return Logger{
		Logger:        zapr.NewLogger(logger).WithName(name),
		logging:       logging,
		out:           writer,
		sugaredLogger: logger.Sugar(),
	}
}

func DefaultLogger(out io.Writer, level logger.LogLevel) Logger {

	logging := logger.DefaultHertzbeatLogging()
	logger := initZapLogger(out, logging, level)

	return Logger{
		Logger:        zapr.NewLogger(logger),
		out:           out,
		logging:       logging,
		sugaredLogger: logger.Sugar(),
	}
}

// WithName returns a new Logger instance with the specified name element added
// to the Logger's name.  Successive calls with WithName append additional
// suffixes to the Logger's name.  It's strongly recommended that name segments
// contain only letters, digits, and hyphens (see the package documentation for
// more information).
func (l Logger) WithName(name string) Logger {

	logLevel := l.logging.Level[logger.HertzbeatLogComponent(name)]
	logger := initZapLogger(l.out, l.logging, logLevel)

	return Logger{
		Logger:        zapr.NewLogger(logger).WithName(name),
		logging:       l.logging,
		out:           l.out,
		sugaredLogger: logger.Sugar().Named(name),
	}
}

// WithValues returns a new Logger instance with additional key/value pairs.
// See Info for documentation on how key/value pairs work.
func (l Logger) WithValues(keysAndValues ...interface{}) Logger {

	l.Logger = l.Logger.WithValues(keysAndValues...)
	return l
}

// A Sugar wraps the base Logger functionality in a slower, but less
// verbose, API. Any Logger can be converted to a SugaredLogger with its Sugar
// method.
//
// Unlike the Logger, the SugaredLogger doesn't insist on structured logger.
// For each log level, it exposes four methods:
//
//   - methods named after the log level for log.Print-style logger
//   - methods ending in "w" for loosely-typed structured logger
//   - methods ending in "f" for log.Printf-style logger
//   - methods ending in "ln" for log.Println-style logger
//
// For example, the methods for InfoLevel are:
//
//	Info(...any)           Print-style logger
//	Infow(...any)          Structured logger (read as "info with")
//	Infof(string, ...any)  Printf-style logger
//	Infoln(...any)         Println-style logger
func (l Logger) Sugar() *zap.SugaredLogger {

	return l.sugaredLogger
}

func initZapLogger(w io.Writer, logging *logger.HertzBeatLogging, level logger.LogLevel) *zap.Logger {

	parseLevel, _ := zapcore.ParseLevel(string(logging.DefaultHertzBeatLoggingLevel(level)))
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), zapcore.AddSync(w), zap.NewAtomicLevelAt(parseLevel))

	return zap.New(core, zap.AddCaller())
}
