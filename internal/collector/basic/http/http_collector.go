/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package http

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/job/protocol"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/constants"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

const (
	ProtocolHTTP = "http"

	// Parse Types
	ParseTypeDefault    = "default"
	ParseTypeJsonPath   = "jsonPath"
	ParseTypePrometheus = "prometheus"
	ParseTypeWebsite    = "website"
	ParseTypeHeader     = "header"

	// Auth Types
	AuthTypeBasic  = "Basic Auth"
	AuthTypeBearer = "Bearer Token"
	AuthTypeDigest = "Digest Auth"
)

type HTTPCollector struct {
	logger logger.Logger
}

func NewHTTPCollector(log logger.Logger) *HTTPCollector {
	return &HTTPCollector{
		logger: log.WithName("http-collector"),
	}
}

func (hc *HTTPCollector) Protocol() string {
	return ProtocolHTTP
}

func (hc *HTTPCollector) PreCheck(metrics *job.Metrics) error {
	if metrics == nil || metrics.HTTP == nil {
		return fmt.Errorf("http configuration is missing")
	}
	return nil
}

func (hc *HTTPCollector) Collect(metrics *job.Metrics) *job.CollectRepMetricsData {
	start := time.Now()

	// MetricsCollector has already performed parameter replacement on metrics.HTTP
	httpConfig := metrics.HTTP

	// 1. Prepare URL
	targetURL := httpConfig.URL

	// Parse SSL bool string
	isSSL := false
	if httpConfig.SSL != "" {
		if val, err := strconv.ParseBool(httpConfig.SSL); err == nil {
			isSSL = val
		}
	}

	if targetURL == "" {
		schema := "http"
		if isSSL {
			schema = "https"
		}
		// Use metrics.ConfigMap values if URL is empty
		// Note: metrics.ConfigMap is already populated with job.Configmap values
		host := metrics.ConfigMap["host"]
		port := metrics.ConfigMap["port"]
		if host != "" && port != "" {
			targetURL = fmt.Sprintf("%s://%s:%s", schema, host, port)
		}
	}

	if targetURL != "" && !strings.HasPrefix(targetURL, "http") {
		schema := "http"
		if isSSL {
			schema = "https"
		}
		targetURL = fmt.Sprintf("%s://%s", schema, targetURL)
	}

	if targetURL == "" {
		return hc.createFailResponse(metrics, constants.CollectFail, "target URL is empty")
	}

	// 2. Create Request
	req, err := hc.createRequest(httpConfig, targetURL)
	if err != nil {
		return hc.createFailResponse(metrics, constants.CollectFail, fmt.Sprintf("failed to create request: %v", err))
	}

	// 3. Create Client
	timeoutStr := metrics.Timeout
	if httpConfig.Timeout != "" {
		timeoutStr = httpConfig.Timeout
	}

	timeout := hc.getTimeout(timeoutStr)
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 4. Execute Request (with Digest retry logic)
	resp, err := client.Do(req)

	// Handle Digest Auth Challenge (401)
	if err == nil && resp.StatusCode == http.StatusUnauthorized &&
		httpConfig.Authorization != nil &&
		strings.EqualFold(httpConfig.Authorization.Type, AuthTypeDigest) {

		// Close the first response body
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		authHeader := resp.Header.Get("WWW-Authenticate")
		if authHeader != "" {
			// Create new request with Authorization header
			digestReq, digestErr := hc.handleDigestAuth(req, authHeader, httpConfig.Authorization)
			if digestErr == nil {
				// Retry request
				resp, err = client.Do(digestReq)
			} else {
				hc.logger.Error(digestErr, "failed to handle digest auth")
			}
		}
	}

	if err != nil {
		return hc.createFailResponse(metrics, constants.CollectUnReachable, fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	responseTime := time.Since(start).Milliseconds()

	// 5. Parse Response
	parseType := httpConfig.ParseType
	if parseType == "" {
		parseType = ParseTypeDefault
	}

	var responseData *job.CollectRepMetricsData

	// Read body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return hc.createFailResponse(metrics, constants.CollectFail, fmt.Sprintf("failed to read body: %v", err))
	}

	switch parseType {
	case ParseTypePrometheus:
		responseData, err = hc.parsePrometheus(bodyBytes, metrics)
	case ParseTypeWebsite:
		responseData, err = hc.parseWebsite(bodyBytes, resp.StatusCode, responseTime, metrics, httpConfig)
	case ParseTypeHeader:
		responseData, err = hc.parseHeader(resp.Header, metrics)
	case ParseTypeJsonPath, ParseTypeDefault:
		responseData, err = hc.parseJsonPath(bodyBytes, metrics, responseTime, httpConfig)
	default:
		responseData, err = hc.parseJsonPath(bodyBytes, metrics, responseTime, httpConfig)
	}

	if err != nil {
		hc.logger.Error(err, "parse response failed", "type", parseType)
		return hc.createFailResponse(metrics, constants.CollectFail, fmt.Sprintf("parse error: %v", err))
	}

	hc.fillCommonFields(responseData, metrics.AliasFields, responseTime, resp.StatusCode)

	return responseData
}

// handleDigestAuth generates a new request with the Digest Authorization header
func (hc *HTTPCollector) handleDigestAuth(originalReq *http.Request, authHeader string, authConfig *protocol.Authorization) (*http.Request, error) {
	params := hc.parseAuthHeader(authHeader)
	realm := params["realm"]
	nonce := params["nonce"]
	qop := params["qop"]
	opaque := params["opaque"]
	algorithm := params["algorithm"]
	if algorithm == "" {
		algorithm = "MD5"
	}

	if realm == "" || nonce == "" {
		return nil, fmt.Errorf("missing realm or nonce in digest header")
	}

	username := authConfig.DigestAuthUsername
	password := authConfig.DigestAuthPassword
	ha1 := hc.md5Hex(fmt.Sprintf("%s:%s:%s", username, realm, password))

	method := originalReq.Method
	uri := originalReq.URL.RequestURI()
	ha2 := hc.md5Hex(fmt.Sprintf("%s:%s", method, uri))

	nc := "00000001"
	cnonce := hc.generateCnonce()

	var responseStr string
	if qop == "" {
		responseStr = hc.md5Hex(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
	} else if strings.Contains(qop, "auth") {
		responseStr = hc.md5Hex(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, "auth", ha2))
	} else {
		return nil, fmt.Errorf("unsupported qop: %s", qop)
	}

	authVal := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="%s"`,
		username, realm, nonce, uri, responseStr, algorithm)

	if opaque != "" {
		authVal += fmt.Sprintf(`, opaque="%s"`, opaque)
	}
	if qop != "" {
		authVal += fmt.Sprintf(`, qop=auth, nc=%s, cnonce="%s"`, nc, cnonce)
	}

	var newReq *http.Request
	if originalReq.GetBody != nil {
		body, _ := originalReq.GetBody()
		newReq, _ = http.NewRequest(originalReq.Method, originalReq.URL.String(), body)
	} else {
		newReq, _ = http.NewRequest(originalReq.Method, originalReq.URL.String(), nil)
	}

	for k, v := range originalReq.Header {
		newReq.Header[k] = v
	}

	newReq.Header.Set("Authorization", authVal)
	return newReq, nil
}

func (hc *HTTPCollector) parseAuthHeader(header string) map[string]string {
	header = strings.TrimPrefix(header, "Digest ")
	result := make(map[string]string)

	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"")
			result[key] = val
		}
	}
	return result
}

func (hc *HTTPCollector) md5Hex(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}

func (hc *HTTPCollector) generateCnonce() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

func (hc *HTTPCollector) createRequest(config *protocol.HTTPProtocol, targetURL string) (*http.Request, error) {
	method := strings.ToUpper(config.Method)
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	if config.Body != "" {
		body = strings.NewReader(config.Body)
	}

	req, err := http.NewRequest(method, targetURL, body)
	if err != nil {
		return nil, err
	}

	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	if config.Authorization != nil {
		auth := config.Authorization
		if auth.Type == "" || strings.EqualFold(auth.Type, AuthTypeBasic) {
			if auth.BasicAuthUsername != "" && auth.BasicAuthPassword != "" {
				req.SetBasicAuth(auth.BasicAuthUsername, auth.BasicAuthPassword)
			}
		} else if strings.EqualFold(auth.Type, AuthTypeBearer) {
			if auth.BearerTokenToken != "" {
				req.Header.Set("Authorization", "Bearer "+auth.BearerTokenToken)
			}
		}
	}

	if len(config.Params) > 0 {
		q := req.URL.Query()
		for k, v := range config.Params {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	return req, nil
}

// --- Parsers ---

func (hc *HTTPCollector) parsePrometheus(body []byte, metrics *job.Metrics) (*job.CollectRepMetricsData, error) {
	response := hc.createSuccessResponse(metrics)
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	for _, mf := range metricFamilies {
		for _, m := range mf.Metric {
			row := job.ValueRow{Columns: make([]string, len(metrics.AliasFields))}
			labels := make(map[string]string)
			for _, pair := range m.Label {
				labels[pair.GetName()] = pair.GetValue()
			}
			hasValue := false
			for i, field := range metrics.AliasFields {
				if field == constants.NULL_VALUE {
					continue
				}
				if field == "value" || field == "prom_value" {
					if m.Gauge != nil {
						row.Columns[i] = fmt.Sprintf("%f", m.Gauge.GetValue())
					} else if m.Counter != nil {
						row.Columns[i] = fmt.Sprintf("%f", m.Counter.GetValue())
					} else if m.Untyped != nil {
						row.Columns[i] = fmt.Sprintf("%f", m.Untyped.GetValue())
					}
					hasValue = true
				} else {
					if val, ok := labels[field]; ok {
						row.Columns[i] = val
						hasValue = true
					} else {
						row.Columns[i] = constants.NULL_VALUE
					}
				}
			}
			if hasValue {
				response.Values = append(response.Values, row)
			}
		}
	}
	response.Time = now
	return response, nil
}

func (hc *HTTPCollector) parseWebsite(body []byte, statusCode int, responseTime int64, metrics *job.Metrics, httpConfig *protocol.HTTPProtocol) (*job.CollectRepMetricsData, error) {
	response := hc.createSuccessResponse(metrics)
	row := job.ValueRow{Columns: make([]string, len(metrics.AliasFields))}
	keyword := httpConfig.Keyword
	keywordCount := 0
	if keyword != "" {
		keywordCount = strings.Count(string(body), keyword)
	}
	for i, field := range metrics.AliasFields {
		switch strings.ToLower(field) {
		case strings.ToLower(constants.StatusCode):
			row.Columns[i] = strconv.Itoa(statusCode)
		case strings.ToLower(constants.RESPONSE_TIME):
			row.Columns[i] = strconv.FormatInt(responseTime, 10)
		case "keyword":
			row.Columns[i] = strconv.Itoa(keywordCount)
		default:
			row.Columns[i] = constants.NULL_VALUE
		}
	}
	response.Values = append(response.Values, row)
	return response, nil
}

func (hc *HTTPCollector) parseHeader(header http.Header, metrics *job.Metrics) (*job.CollectRepMetricsData, error) {
	response := hc.createSuccessResponse(metrics)
	row := job.ValueRow{Columns: make([]string, len(metrics.AliasFields))}
	for i, field := range metrics.AliasFields {
		val := header.Get(field)
		if val != "" {
			row.Columns[i] = val
		} else {
			row.Columns[i] = constants.NULL_VALUE
		}
	}
	response.Values = append(response.Values, row)
	return response, nil
}

func (hc *HTTPCollector) parseJsonPath(body []byte, metrics *job.Metrics, responseTime int64, httpConfig *protocol.HTTPProtocol) (*job.CollectRepMetricsData, error) {
	response := hc.createSuccessResponse(metrics)
	var data interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return response, nil
	}
	parseScript := httpConfig.ParseScript
	root := data
	if parseScript != "" && parseScript != "$" {
		root = hc.navigateJson(data, parseScript)
	}
	if root == nil {
		return response, nil
	}
	if arr, ok := root.([]interface{}); ok {
		for _, item := range arr {
			row := hc.extractRow(item, metrics.AliasFields)
			response.Values = append(response.Values, row)
		}
	} else {
		row := hc.extractRow(root, metrics.AliasFields)
		response.Values = append(response.Values, row)
	}
	return response, nil
}

func (hc *HTTPCollector) navigateJson(data interface{}, path string) interface{} {
	path = strings.TrimPrefix(path, "$.")
	if path == "" {
		return data
	}
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		if current == nil {
			return nil
		}
		key := part
		idx := -1
		if i := strings.Index(part, "["); i > -1 && strings.HasSuffix(part, "]") {
			key = part[:i]
			idxStr := part[i+1 : len(part)-1]
			if val, err := strconv.Atoi(idxStr); err == nil {
				idx = val
			}
		}
		if key != "" {
			if m, ok := current.(map[string]interface{}); ok {
				if val, exists := m[key]; exists {
					current = val
				} else {
					return nil
				}
			} else {
				return nil
			}
		}
		if idx > -1 {
			if arr, ok := current.([]interface{}); ok {
				if idx < len(arr) {
					current = arr[idx]
				} else {
					return nil
				}
			} else {
				return nil
			}
		}
	}
	return current
}

func (hc *HTTPCollector) extractRow(item interface{}, fields []string) job.ValueRow {
	row := job.ValueRow{Columns: make([]string, len(fields))}
	m, isMap := item.(map[string]interface{})
	for i, field := range fields {
		if strings.EqualFold(field, constants.RESPONSE_TIME) || strings.EqualFold(field, constants.StatusCode) {
			row.Columns[i] = constants.NULL_VALUE
			continue
		}
		var val interface{}
		if isMap {
			if v, ok := m[field]; ok {
				val = v
			} else {
				val = hc.navigateJson(item, field)
			}
		}
		if val != nil {
			row.Columns[i] = fmt.Sprintf("%v", val)
		} else {
			row.Columns[i] = constants.NULL_VALUE
		}
	}
	return row
}

func (hc *HTTPCollector) fillCommonFields(resp *job.CollectRepMetricsData, fields []string, responseTime int64, statusCode int) {
	if resp == nil {
		return
	}
	if len(resp.Fields) == 0 {
		resp.Fields = make([]job.Field, len(fields))
		for i, f := range fields {
			resp.Fields[i] = job.Field{Field: f, Type: constants.TYPE_STRING}
		}
	}
	if len(resp.Values) == 0 {
		row := job.ValueRow{Columns: make([]string, len(fields))}
		for k := range row.Columns {
			row.Columns[k] = constants.NULL_VALUE
		}
		resp.Values = append(resp.Values, row)
	}
	for i := range resp.Values {
		for j, field := range fields {
			if strings.EqualFold(field, constants.RESPONSE_TIME) {
				resp.Values[i].Columns[j] = strconv.FormatInt(responseTime, 10)
			}
			if strings.EqualFold(field, constants.StatusCode) {
				resp.Values[i].Columns[j] = strconv.Itoa(statusCode)
			}
		}
	}
}

func (hc *HTTPCollector) getTimeout(timeoutStr string) time.Duration {
	if timeoutStr == "" {
		return 10 * time.Second
	}
	if val, err := strconv.Atoi(timeoutStr); err == nil {
		return time.Duration(val) * time.Millisecond
	}
	return 10 * time.Second
}

func (hc *HTTPCollector) createSuccessResponse(metrics *job.Metrics) *job.CollectRepMetricsData {
	return &job.CollectRepMetricsData{
		Metrics: metrics.Name,
		Time:    time.Now().UnixMilli(),
		Code:    constants.CollectSuccess,
		Msg:     "success",
		Values:  make([]job.ValueRow, 0),
	}
}

func (hc *HTTPCollector) createFailResponse(metrics *job.Metrics, code int, msg string) *job.CollectRepMetricsData {
	return &job.CollectRepMetricsData{
		Metrics: metrics.Name,
		Time:    time.Now().UnixMilli(),
		Code:    code,
		Msg:     msg,
	}
}
