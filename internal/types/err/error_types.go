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

package err

import "errors"

// Collector Server Error Types
var (
	CollectorConfigIsNil = errors.New("collector config is nil")
	CollectorIPIsNil     = errors.New("collector ip is empty")
	CollectorPortIsNil   = errors.New("collector port is empty")
	CollectorServerStop  = errors.New("collector server stop")
)

// Collector Banner Error Types
var (
	BannerPrintReaderError  = errors.New("print banner error")
	BannerPrintExecuteError = errors.New("print banner execute error")
)
