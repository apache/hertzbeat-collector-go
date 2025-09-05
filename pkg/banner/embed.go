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

package banner

import (
	"embed"
	"os"
	"strconv"
	"text/template"
)

//go:embed banner.txt
var EmbedLogo embed.FS

// LoggerInterface defines the interface for logging
type LoggerInterface interface {
	Error(err error, msg string)
	Info(msg string, keysAndValues ...interface{})
}

type Banner struct {
	logger LoggerInterface
}

func New(logger LoggerInterface) *Banner {
	return &Banner{logger: logger}
}

type bannerVars struct {
	CollectorName string
	ServerPort    string
	Pid           string
	Version       string
}

func (b *Banner) PrintBanner(appName, port string) error {
	return b.PrintBannerWithVersion(appName, port, "unknown")
}

func (b *Banner) PrintBannerWithVersion(appName, port, version string) error {
	data, err := EmbedLogo.ReadFile("banner.txt")
	if err != nil {
		b.logger.Error(err, "read banner file failed")
		return err
	}

	tmpl, err := template.New("banner").Parse(string(data))
	if err != nil {
		b.logger.Error(err, "parse banner template failed")
		return err
	}

	vars := bannerVars{
		CollectorName: appName,
		ServerPort:    port,
		Pid:           strconv.Itoa(os.Getpid()),
		Version:       version,
	}

	err = tmpl.Execute(os.Stdout, vars)
	if err != nil {
		return err
	}

	return nil
}
