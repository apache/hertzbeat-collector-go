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

package version

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Info struct {
	HCGVersion    string `json:"envoyGatewayVersion"`
	GitCommitID   string `json:"gitCommitID"`
	GolangVersion string `json:"golangVersion"`
}

func Get() Info {
	return Info{
		HCGVersion:    hcgVersion,
		GitCommitID:   gitCommitID,
		GolangVersion: runtime.Version(),
	}
}

var (
	hcgVersion  string
	gitCommitID string
)

// Print shows the versions of the Envoy Gateway.
func Print(w io.Writer, format string) error {
	v := Get()
	switch format {
	case "json":
		if marshalled, err := json.MarshalIndent(v, "", "  "); err == nil {
			_, _ = fmt.Fprintln(w, string(marshalled))
		}
	case "yaml":
		if marshalled, err := yaml.Marshal(v); err == nil {
			_, _ = fmt.Fprintln(w, string(marshalled))
		}
	default:
		_, _ = fmt.Fprintf(w, "HERTZBEAT_COLECTOR_GO_VERSION: %s\n", v.HCGVersion)
		_, _ = fmt.Fprintf(w, "GIT_COMMIT_ID: %s\n", v.GitCommitID)
		_, _ = fmt.Fprintf(w, "GOLANG_VERSION: %s\n", v.GolangVersion)
	}

	return nil
}
