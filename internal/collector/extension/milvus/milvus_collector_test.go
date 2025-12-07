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

package milvus

import (
	"os"
	"testing"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	loggertypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

func TestMilvusCollector_Collect(t *testing.T) {
	metrics := &job.Metrics{
		Name: "milvus_test",
		Milvus: &job.MilvusProtocol{
			Host: "localhost",
			Port: "19530",
		},
		AliasFields: []string{"version", "responseTime", "host", "port"},
	}

	l := logger.DefaultLogger(os.Stdout, loggertypes.LogLevelInfo)
	collector := NewMilvusCollector(l)
	result := collector.Collect(metrics)

	if result.Code != 0 {
		t.Logf("Collect failed: %s", result.Msg)
	} else {
		t.Logf("Collect success: %+v", result.Values)
	}
}
