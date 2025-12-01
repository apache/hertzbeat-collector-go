package milvus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/collect/strategy"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

func init() {
	strategy.RegisterFactory(ProtocolMilvus, func(logger logger.Logger) strategy.Collector {
		return NewMilvusCollector(logger)
	})
}

const (
	ProtocolMilvus = "milvus"
)

type MilvusCollector struct {
	logger logger.Logger
}

func NewMilvusCollector(log logger.Logger) *MilvusCollector {
	return &MilvusCollector{
		logger: log.WithName("milvus-collector"),
	}
}

func (c *MilvusCollector) Protocol() string {
	return ProtocolMilvus
}

func (c *MilvusCollector) PreCheck(metrics *job.Metrics) error {
	if metrics == nil || metrics.Milvus == nil {
		return fmt.Errorf("milvus configuration is missing")
	}
	return nil
}

func (c *MilvusCollector) Collect(metrics *job.Metrics) *job.CollectRepMetricsData {
	start := time.Now()
	milvusConfig := metrics.Milvus

	// 1. Prepare connection
	addr := fmt.Sprintf("%s:%s", milvusConfig.Host, milvusConfig.Port)

	timeout := 10 * time.Second
	if milvusConfig.Timeout != "" {
		if t, err := strconv.Atoi(milvusConfig.Timeout); err == nil {
			timeout = time.Duration(t) * time.Millisecond
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 2. Connect
	var milvusClient client.Client
	var err error

	if milvusConfig.Username != "" && milvusConfig.Password != "" {
		milvusClient, err = client.NewDefaultGrpcClientWithAuth(ctx, addr, milvusConfig.Username, milvusConfig.Password)
	} else {
		milvusClient, err = client.NewDefaultGrpcClient(ctx, addr)
	}

	if err != nil {
		return c.createFailResponse(metrics, constants.CollectUnConnectable, fmt.Sprintf("failed to connect: %v", err))
	}
	defer milvusClient.Close()

	// 3. Collect data
	// Currently only supports basic availability check and version retrieval
	version, err := milvusClient.GetVersion(ctx)
	if err != nil {
		return c.createFailResponse(metrics, constants.CollectFail, fmt.Sprintf("failed to get version: %v", err))
	}

	responseTime := time.Since(start).Milliseconds()

	// 4. Parse/Format Response
	response := c.createSuccessResponse(metrics)
	row := job.ValueRow{Columns: make([]string, len(metrics.AliasFields))}

	for i, field := range metrics.AliasFields {
		switch strings.ToLower(field) {
		case strings.ToLower(constants.RESPONSE_TIME):
			row.Columns[i] = strconv.FormatInt(responseTime, 10)
		case "version":
			row.Columns[i] = version
		case "host":
			row.Columns[i] = milvusConfig.Host
		case "port":
			row.Columns[i] = milvusConfig.Port
		default:
			row.Columns[i] = constants.NULL_VALUE
		}
	}

	response.Values = append(response.Values, row)
	return response
}

func (c *MilvusCollector) createSuccessResponse(metrics *job.Metrics) *job.CollectRepMetricsData {
	return &job.CollectRepMetricsData{
		Metrics: metrics.Name,
		Time:    time.Now().UnixMilli(),
		Code:    constants.CollectSuccess,
		Msg:     "success",
		Values:  make([]job.ValueRow, 0),
	}
}

func (c *MilvusCollector) createFailResponse(metrics *job.Metrics, code int, msg string) *job.CollectRepMetricsData {
	return &job.CollectRepMetricsData{
		Metrics: metrics.Name,
		Time:    time.Now().UnixMilli(),
		Code:    code,
		Msg:     msg,
	}
}
