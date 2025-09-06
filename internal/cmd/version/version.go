// Copyright Envoy Gateway Authors
// SPDX-License-Identifier: Apache-2.0
// The full text of the Apache license is available in the LICENSE file at
// the root of the repo.

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
