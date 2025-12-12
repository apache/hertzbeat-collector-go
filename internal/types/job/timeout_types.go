/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package job

import (
	"time"
)

// TimerTask represents a task that can be scheduled in the timer wheel
type TimerTask interface {
	Run(timeout *Timeout) error
}

// Timeout represents a scheduled task in the timer wheel
type Timeout struct {
	task        TimerTask
	deadline    time.Time
	cancelled   bool
	wheelIndex  int
	bucketIndex int
}

// NewTimeout creates a new timeout instance
func NewTimeout(task TimerTask, delay time.Duration) *Timeout {
	return &Timeout{
		task:     task,
		deadline: time.Now().Add(delay),
	}
}

// Task returns the timer task
func (t *Timeout) Task() TimerTask {
	return t.task
}

// Deadline returns the deadline of this timeout
func (t *Timeout) Deadline() time.Time {
	return t.deadline
}

// IsCancelled returns true if this timeout was cancelled
func (t *Timeout) IsCancelled() bool {
	return t.cancelled
}

// Cancel cancels this timeout
func (t *Timeout) Cancel() bool {
	if t.cancelled {
		return false
	}
	t.cancelled = true
	return true
}

// SetWheelIndex sets the wheel index for this timeout
func (t *Timeout) SetWheelIndex(index int) {
	t.wheelIndex = index
}

// SetBucketIndex sets the bucket index for this timeout
func (t *Timeout) SetBucketIndex(index int) {
	t.bucketIndex = index
}

// WheelIndex returns the wheel index
func (t *Timeout) WheelIndex() int {
	return t.wheelIndex
}

// BucketIndex returns the bucket index
func (t *Timeout) BucketIndex() int {
	return t.bucketIndex
}
