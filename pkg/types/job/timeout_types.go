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
	"context"
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
	ctx         context.Context
	cancel      context.CancelFunc
	wheelIndex  int
	bucketIndex int
}

// NewTimeout creates a new timeout instance
func NewTimeout(task TimerTask, delay time.Duration) *Timeout {
	ctx, cancel := context.WithCancel(context.Background())
	return &Timeout{
		task:     task,
		deadline: time.Now().Add(delay),
		ctx:      ctx,
		cancel:   cancel,
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
	t.cancel()
	return true
}

// Context returns the context of this timeout
func (t *Timeout) Context() context.Context {
	return t.ctx
}

// SetWheelIndex sets the wheel index for internal use
func (t *Timeout) SetWheelIndex(index int) {
	t.wheelIndex = index
}

// GetWheelIndex gets the wheel index
func (t *Timeout) GetWheelIndex() int {
	return t.wheelIndex
}

// SetBucketIndex sets the bucket index for internal use
func (t *Timeout) SetBucketIndex(index int) {
	t.bucketIndex = index
}

// GetBucketIndex gets the bucket index
func (t *Timeout) GetBucketIndex() int {
	return t.bucketIndex
}
