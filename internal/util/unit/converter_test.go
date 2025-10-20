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

package unit

import (
	"testing"
)

func TestParseConversion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantField  string
		wantOrigin string
		wantNew    string
		wantErr    bool
	}{
		{
			name:       "valid byte conversion",
			input:      "committed=B->MB",
			wantField:  "committed",
			wantOrigin: "B",
			wantNew:    "MB",
			wantErr:    false,
		},
		{
			name:       "valid with spaces",
			input:      "memory = KB -> GB",
			wantField:  "memory",
			wantOrigin: "KB",
			wantNew:    "GB",
			wantErr:    false,
		},
		{
			name:       "time conversion",
			input:      "duration=ms->s",
			wantField:  "duration",
			wantOrigin: "ms",
			wantNew:    "s",
			wantErr:    false,
		},
		{
			name:    "missing arrow",
			input:   "committed=B",
			wantErr: true,
		},
		{
			name:    "missing equals",
			input:   "committed",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConversion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConversion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Field != tt.wantField {
					t.Errorf("ParseConversion() Field = %v, want %v", got.Field, tt.wantField)
				}
				if got.OriginUnit != tt.wantOrigin {
					t.Errorf("ParseConversion() OriginUnit = %v, want %v", got.OriginUnit, tt.wantOrigin)
				}
				if got.NewUnit != tt.wantNew {
					t.Errorf("ParseConversion() NewUnit = %v, want %v", got.NewUnit, tt.wantNew)
				}
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name       string
		value      float64
		originUnit string
		newUnit    string
		want       float64
		wantErr    bool
	}{
		{
			name:       "B to MB",
			value:      1048576,
			originUnit: "B",
			newUnit:    "MB",
			want:       1,
			wantErr:    false,
		},
		{
			name:       "KB to MB",
			value:      1024,
			originUnit: "KB",
			newUnit:    "MB",
			want:       1,
			wantErr:    false,
		},
		{
			name:       "MB to GB",
			value:      1024,
			originUnit: "MB",
			newUnit:    "GB",
			want:       1,
			wantErr:    false,
		},
		{
			name:       "GB to MB",
			value:      2,
			originUnit: "GB",
			newUnit:    "MB",
			want:       2048,
			wantErr:    false,
		},
		{
			name:       "same unit",
			value:      100,
			originUnit: "MB",
			newUnit:    "MB",
			want:       100,
			wantErr:    false,
		},
		{
			name:       "case insensitive",
			value:      1024,
			originUnit: "kb",
			newUnit:    "mb",
			want:       1,
			wantErr:    false,
		},
		{
			name:       "ms to s",
			value:      1000,
			originUnit: "ms",
			newUnit:    "s",
			want:       1,
			wantErr:    false,
		},
		{
			name:       "s to ms",
			value:      1,
			originUnit: "s",
			newUnit:    "ms",
			want:       1000,
			wantErr:    false,
		},
		{
			name:       "unsupported conversion",
			value:      100,
			originUnit: "MB",
			newUnit:    "UNKNOWN",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.value, tt.originUnit, tt.newUnit)
			if (err != nil) != tt.wantErr {
				t.Errorf("Convert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Convert() = %v, want %v", got, tt.want)
			}
		})
	}
}
