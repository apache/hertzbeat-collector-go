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
