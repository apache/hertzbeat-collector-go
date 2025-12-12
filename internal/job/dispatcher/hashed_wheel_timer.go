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

package dispatcher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	jobtypes "hertzbeat.apache.org/hertzbeat-collector-go/internal/types/job"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// hashedWheelTimer implements a time wheel for efficient timeout management
type hashedWheelTimer struct {
	logger       logger.Logger
	tickDuration time.Duration
	wheelSize    int
	wheel        []*bucket
	currentTick  int64
	startTime    time.Time
	ticker       *time.Ticker
	started      int32
	stopped      int32
	wg           sync.WaitGroup
	cancel       context.CancelFunc // Only store cancel, not the context itself
}

// bucket represents a time wheel bucket containing timeouts
type bucket struct {
	timeouts []*jobtypes.Timeout
	mu       sync.RWMutex
}

// NewHashedWheelTimer creates a new hashed wheel timer
func NewHashedWheelTimer(wheelSize int, tickDuration time.Duration, logger logger.Logger) HashedWheelTimer {
	if wheelSize <= 0 {
		wheelSize = 512
	}
	if tickDuration <= 0 {
		tickDuration = 100 * time.Millisecond
	}

	timer := &hashedWheelTimer{
		logger:       logger.WithName("hashed-wheel-timer"),
		tickDuration: tickDuration,
		wheelSize:    wheelSize,
		wheel:        make([]*bucket, wheelSize),
	}

	// Initialize buckets
	for i := 0; i < wheelSize; i++ {
		timer.wheel[i] = &bucket{
			timeouts: make([]*jobtypes.Timeout, 0),
		}
	}

	return timer
}

// NewTimeout creates a new timeout and adds it to the wheel
func (hwt *hashedWheelTimer) NewTimeout(task jobtypes.TimerTask, delay time.Duration) *jobtypes.Timeout {
	if atomic.LoadInt32(&hwt.stopped) == 1 {
		hwt.logger.Info("timer is stopped, cannot create new timeout")
		return nil
	}

	timeout := jobtypes.NewTimeout(task, delay)

	// Calculate which bucket and tick this timeout belongs to
	totalTicks := int64(delay / hwt.tickDuration)
	currentTick := atomic.LoadInt64(&hwt.currentTick)
	targetTick := currentTick + totalTicks

	bucketIndex := int(targetTick % int64(hwt.wheelSize))
	timeout.SetWheelIndex(int(targetTick))
	timeout.SetBucketIndex(bucketIndex)

	// Add to appropriate bucket
	bucket := hwt.wheel[bucketIndex]
	bucket.mu.Lock()
	bucket.timeouts = append(bucket.timeouts, timeout)
	bucket.mu.Unlock()

	// Log timeout creation only for debugging
	hwt.logger.V(1).Info("created timeout",
		"delay", delay,
		"bucketIndex", bucketIndex,
		"targetTick", targetTick)

	return timeout
}

// Start starts the timer wheel
func (hwt *hashedWheelTimer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&hwt.started, 0, 1) {
		return nil // already started
	}

	hwt.logger.Info("starting hashed wheel timer")

	hwt.startTime = time.Now()
	hwt.ticker = time.NewTicker(hwt.tickDuration)

	// Create a cancellable context for this timer
	ctx, cancel := context.WithCancel(ctx)
	hwt.cancel = cancel

	hwt.wg.Add(1)
	go hwt.run(ctx)

	hwt.logger.Info("hashed wheel timer started")
	return nil
}

// Stop stops the timer wheel
func (hwt *hashedWheelTimer) Stop() error {
	if !atomic.CompareAndSwapInt32(&hwt.stopped, 0, 1) {
		return nil // already stopped
	}

	hwt.cancel()

	if hwt.ticker != nil {
		hwt.ticker.Stop()
	}

	hwt.wg.Wait()

	hwt.logger.Info("hashed wheel timer stopped")
	return nil
}

// run is the main timer loop
func (hwt *hashedWheelTimer) run(ctx context.Context) {
	defer hwt.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hwt.ticker.C:
			hwt.tick()
		}
	}
}

// tick processes one tick of the wheel
func (hwt *hashedWheelTimer) tick() {
	currentTick := atomic.AddInt64(&hwt.currentTick, 1)
	bucketIndex := int(currentTick % int64(hwt.wheelSize))

	bucket := hwt.wheel[bucketIndex]
	bucket.mu.Lock()

	// Only log tick processing if there are timeouts to process
	if len(bucket.timeouts) > 0 {
		hwt.logger.V(1).Info("processing tick",
			"currentTick", currentTick,
			"bucketIndex", bucketIndex,
			"timeoutsCount", len(bucket.timeouts))
	}

	// Process expired timeouts
	var remaining []*jobtypes.Timeout
	for _, timeout := range bucket.timeouts {
		if timeout.IsCancelled() {
			continue // skip cancelled timeouts
		}

		now := time.Now()
		deadline := timeout.Deadline()

		// Check if timeout should execute based on wheel position
		// A timeout should execute when currentTick >= its targetTick
		wheelIndex := timeout.WheelIndex()
		if currentTick >= int64(wheelIndex) {
			// Only log actual task execution at INFO level
			hwt.logger.Info("executing scheduled task",
				"currentTick", currentTick,
				"targetTick", wheelIndex)

			// Execute the timeout task asynchronously
			go func(t *jobtypes.Timeout) {
				defer func() {
					if r := recover(); r != nil {
						hwt.logger.Error(nil, "panic in timeout task execution", "panic", r)
					}
				}()

				if err := t.Task().Run(t); err != nil {
					hwt.logger.Error(err, "error executing timeout task")
				}
			}(timeout)
		} else {
			// Not yet time to execute, keep in bucket
			hwt.logger.V(1).Info("timeout not ready by wheel position",
				"currentTick", currentTick,
				"targetTick", wheelIndex,
				"deadline", deadline,
				"now", now)
			remaining = append(remaining, timeout)
		}
	}

	bucket.timeouts = remaining
	bucket.mu.Unlock()
}

// GetStats returns timer statistics
func (hwt *hashedWheelTimer) GetStats() map[string]interface{} {
	totalTimeouts := 0
	for _, bucket := range hwt.wheel {
		bucket.mu.RLock()
		totalTimeouts += len(bucket.timeouts)
		bucket.mu.RUnlock()
	}

	return map[string]interface{}{
		"wheelSize":     hwt.wheelSize,
		"tickDuration":  hwt.tickDuration.String(),
		"currentTick":   atomic.LoadInt64(&hwt.currentTick),
		"totalTimeouts": totalTimeouts,
		"started":       atomic.LoadInt32(&hwt.started) == 1,
		"stopped":       atomic.LoadInt32(&hwt.stopped) == 1,
		"wheelPeriod":   (time.Duration(hwt.wheelSize) * hwt.tickDuration).String(),
	}
}
