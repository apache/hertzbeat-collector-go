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

package timer

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/types/job"
)

const (
	// DefaultWheelSize default wheel size (512 slots)
	DefaultWheelSize = 512
	// DefaultTickDuration default tick duration (1 second)
	DefaultTickDuration = 1 * time.Second
)

// TimerWheel represents a hashed wheel timer implementation
type TimerWheel struct {
	tickDuration time.Duration
	wheelSize    int
	wheel        []*bucket
	currentTick  int64
	startTime    time.Time
	ticker       *time.Ticker
	workerPool   chan struct{}

	mutex   sync.RWMutex
	started atomic.Bool
	stopped atomic.Bool

	logger logger.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// bucket represents a slot in the timer wheel
type bucket struct {
	timeouts *list.List
	mutex    sync.RWMutex
}

// newBucket creates a new bucket
func newBucket() *bucket {
	return &bucket{
		timeouts: list.New(),
	}
}

// addTimeout adds a timeout to the bucket
func (b *bucket) addTimeout(timeout *job.Timeout) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if timeout.IsCancelled() {
		return
	}

	b.timeouts.PushBack(timeout)
}

// removeTimeout removes a timeout from the bucket
func (b *bucket) removeTimeout(timeout *job.Timeout) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for e := b.timeouts.Front(); e != nil; e = e.Next() {
		if e.Value.(*job.Timeout) == timeout {
			b.timeouts.Remove(e)
			return true
		}
	}
	return false
}

// expiredTimeouts returns and removes all expired timeouts
func (b *bucket) expiredTimeouts(currentTime time.Time) []*job.Timeout {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var expired []*job.Timeout
	for e := b.timeouts.Front(); e != nil; {
		timeout := e.Value.(*job.Timeout)
		next := e.Next()

		if timeout.IsCancelled() {
			b.timeouts.Remove(e)
		} else if !timeout.Deadline().After(currentTime) {
			expired = append(expired, timeout)
			b.timeouts.Remove(e)
		}

		e = next
	}

	return expired
}

// NewTimerWheel creates a new timer wheel with default settings
func NewTimerWheel(logger logger.Logger) *TimerWheel {
	return NewTimerWheelWithConfig(DefaultTickDuration, DefaultWheelSize, logger)
}

// NewTimerWheelWithConfig creates a new timer wheel with custom configuration
func NewTimerWheelWithConfig(tickDuration time.Duration, wheelSize int, logger logger.Logger) *TimerWheel {
	ctx, cancel := context.WithCancel(context.Background())

	tw := &TimerWheel{
		tickDuration: tickDuration,
		wheelSize:    wheelSize,
		wheel:        make([]*bucket, wheelSize),
		startTime:    time.Now(),
		workerPool:   make(chan struct{}, 64), // limit concurrent executions
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Initialize buckets
	for i := 0; i < wheelSize; i++ {
		tw.wheel[i] = newBucket()
	}

	return tw
}

// Start starts the timer wheel
func (tw *TimerWheel) Start() error {
	if !tw.started.CompareAndSwap(false, true) {
		return nil // already started
	}

	tw.logger.Info("starting timer wheel", "tickDuration", tw.tickDuration, "wheelSize", tw.wheelSize)

	tw.ticker = time.NewTicker(tw.tickDuration)

	go tw.run()

	return nil
}

// Stop stops the timer wheel
func (tw *TimerWheel) Stop() error {
	if !tw.stopped.CompareAndSwap(false, true) {
		return nil // already stopped
	}

	tw.logger.Info("stopping timer wheel")

	tw.cancel()

	if tw.ticker != nil {
		tw.ticker.Stop()
	}

	return nil
}

// NewTimeout schedules a new timeout task
func (tw *TimerWheel) NewTimeout(task job.TimerTask, delay time.Duration) *job.Timeout {
	if tw.stopped.Load() {
		tw.logger.Info("timer wheel is stopped, cannot schedule new timeout")
		return nil
	}

	timeout := job.NewTimeout(task, delay)

	tw.mutex.Lock()
	defer tw.mutex.Unlock()

	bucketIndex := tw.calculateBucketIndex(timeout.Deadline())
	timeout.SetWheelIndex(bucketIndex)

	tw.wheel[bucketIndex].addTimeout(timeout)

	tw.logger.Info("scheduled new timeout",
		"delay", delay,
		"deadline", timeout.Deadline(),
		"bucketIndex", bucketIndex)

	return timeout
}

// run is the main timer wheel loop
func (tw *TimerWheel) run() {
	defer tw.logger.Info("timer wheel stopped")

	for {
		select {
		case <-tw.ctx.Done():
			return
		case now := <-tw.ticker.C:
			tw.tick(now)
		}
	}
}

// tick processes one tick of the timer wheel
func (tw *TimerWheel) tick(now time.Time) {
	currentTick := atomic.AddInt64(&tw.currentTick, 1)

	// Check all buckets for expired timeouts, not just the current bucket
	// This is more robust for initial implementation
	totalExpired := 0
	for i := 0; i < tw.wheelSize; i++ {
		bucket := tw.wheel[i]
		expired := bucket.expiredTimeouts(now)

		if len(expired) > 0 {
			totalExpired += len(expired)
			for _, timeout := range expired {
				tw.executeTimeout(timeout)
			}
		}
	}

	if totalExpired > 0 {
		tw.logger.Info("processing expired timeouts",
			"count", totalExpired,
			"currentTick", currentTick,
			"now", now)
	}
}

// executeTimeout executes a timeout task
func (tw *TimerWheel) executeTimeout(timeout *job.Timeout) {
	if timeout.IsCancelled() {
		return
	}

	select {
	case tw.workerPool <- struct{}{}:
		go func() {
			defer func() {
				<-tw.workerPool
				if r := recover(); r != nil {
					tw.logger.Info("panic in timeout execution", "error", r)
				}
			}()

			if err := timeout.Task().Run(timeout); err != nil {
				tw.logger.Error(err, "error executing timeout task")
			}
		}()
	default:
		tw.logger.Info("worker pool is full, dropping timeout task")
	}
}

// calculateBucketIndex calculates which bucket a deadline should go to
func (tw *TimerWheel) calculateBucketIndex(deadline time.Time) int {
	ticksFromStart := deadline.Sub(tw.startTime) / tw.tickDuration
	return int(ticksFromStart) % tw.wheelSize
}

// IsStarted returns true if the timer wheel is started
func (tw *TimerWheel) IsStarted() bool {
	return tw.started.Load()
}

// IsStopped returns true if the timer wheel is stopped
func (tw *TimerWheel) IsStopped() bool {
	return tw.stopped.Load()
}

// Stats returns statistics about the timer wheel
func (tw *TimerWheel) Stats() TimerWheelStats {
	tw.mutex.RLock()
	defer tw.mutex.RUnlock()

	stats := TimerWheelStats{
		WheelSize:    tw.wheelSize,
		TickDuration: tw.tickDuration,
		CurrentTick:  atomic.LoadInt64(&tw.currentTick),
		StartTime:    tw.startTime,
		BucketStats:  make([]BucketStats, tw.wheelSize),
	}

	for i, bucket := range tw.wheel {
		bucket.mutex.RLock()
		stats.BucketStats[i] = BucketStats{
			Index:        i,
			TimeoutCount: bucket.timeouts.Len(),
		}
		stats.TotalTimeouts += bucket.timeouts.Len()
		bucket.mutex.RUnlock()
	}

	return stats
}

// TimerWheelStats contains statistics about the timer wheel
type TimerWheelStats struct {
	WheelSize     int           `json:"wheelSize"`
	TickDuration  time.Duration `json:"tickDuration"`
	CurrentTick   int64         `json:"currentTick"`
	StartTime     time.Time     `json:"startTime"`
	TotalTimeouts int           `json:"totalTimeouts"`
	BucketStats   []BucketStats `json:"bucketStats"`
}

// BucketStats contains statistics about a single bucket
type BucketStats struct {
	Index        int `json:"index"`
	TimeoutCount int `json:"timeoutCount"`
}
