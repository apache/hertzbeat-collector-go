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

	clrserver "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/server"
	bannertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/err"
)

//go:embed banner.txt
var EmbedLogo embed.FS

type Config struct {
	clrserver.Server
}

// Runner implements the banner display functionality.
type Runner struct {
	clrserver.Server
}

type bannerVars struct {
	CollectorName string
	ServerPort    string
	Pid           string
}

func New(srv *Config) *Runner {

	return &Runner{
		Server: srv.Server,
	}
}

func (r *Runner) PrintBanner(appName, port string) error {

	r.Logger = r.Logger.WithName("banner").WithValues("runner", "banner")

	data, err := EmbedLogo.ReadFile("banner.txt")
	if err != nil {
		r.Logger.Error(bannertypes.BannerPrintReaderError, "output banner read error", "error", err)
		return err
	}

	tmpl, err := template.New("banner").Parse(string(data))
	if err != nil {
		r.Logger.Error(bannertypes.BannerPrintExecuteError, "template parse error", "error", err)
		return err
	}

	vars := bannerVars{
		CollectorName: appName,
		ServerPort:    port,
		Pid:           strconv.Itoa(os.Getpid()),
	}

	err = tmpl.Execute(os.Stdout, vars)
	if err != nil {
		r.Logger.Error(bannertypes.BannerPrintExecuteError, "template parse error", "error", err)
		return err
	}

	return nil
}
