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

package strategy

import (
	"sync"
)

// AbstractCollect interface
type AbstractCollect interface {
	SupportProtocol() string
}

// strategy container
var (
	collectStrategy = make(map[string]AbstractCollect)
	mu              sync.RWMutex
)

// Register a collect strategy
// Example: registration in init() of each implementation file
//
//	func init() {
//	    Register(&XXXCollect{})
//	}
func Register(collect AbstractCollect) {

	mu.Lock()
	defer mu.Unlock()

	collectStrategy[collect.SupportProtocol()] = collect
}

// Invoke returns the collect strategy for a protocol
func Invoke(protocol string) AbstractCollect {

	mu.RLock()
	defer mu.RUnlock()

	return collectStrategy[protocol]
}
