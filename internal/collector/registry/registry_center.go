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

package registry

import (
	"fmt"
	"sync"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/basic"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// CollectorFactory 采集器工厂函数类型
type CollectorFactory func(logger logger.Logger) basic.AbstractCollector

// CollectorRegistration 采集器注册信息
type CollectorRegistration struct {
	Protocol string
	Factory  CollectorFactory
	Enabled  bool
	Priority int // 优先级，数字越小优先级越高
}

// RegistryCenter 注册中心
type RegistryCenter struct {
	registrations map[string]*CollectorRegistration
	mutex         sync.RWMutex
}

// 全局注册中心实例
var globalCenter = &RegistryCenter{
	registrations: make(map[string]*CollectorRegistration),
}

// GetGlobalCenter 获取全局注册中心
func GetGlobalCenter() *RegistryCenter {
	return globalCenter
}

// RegisterFactory 注册采集器工厂函数
func (rc *RegistryCenter) RegisterFactory(protocol string, factory CollectorFactory, options ...RegistrationOption) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	registration := &CollectorRegistration{
		Protocol: protocol,
		Factory:  factory,
		Enabled:  true, // 默认启用
		Priority: 100,  // 默认优先级
	}

	// 应用选项
	for _, option := range options {
		option(registration)
	}

	rc.registrations[protocol] = registration
}

// RegistrationOption 注册选项函数类型
type RegistrationOption func(*CollectorRegistration)

// WithDisabled 禁用采集器选项
func WithDisabled() RegistrationOption {
	return func(reg *CollectorRegistration) {
		reg.Enabled = false
	}
}

// WithPriority 设置优先级选项
func WithPriority(priority int) RegistrationOption {
	return func(reg *CollectorRegistration) {
		reg.Priority = priority
	}
}

// GetRegistrations 获取所有注册信息
func (rc *RegistryCenter) GetRegistrations() map[string]*CollectorRegistration {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	// 返回副本，避免外部修改
	result := make(map[string]*CollectorRegistration)
	for k, v := range rc.registrations {
		result[k] = &CollectorRegistration{
			Protocol: v.Protocol,
			Factory:  v.Factory,
			Enabled:  v.Enabled,
			Priority: v.Priority,
		}
	}
	return result
}

// GetRegistration 获取特定协议的注册信息
func (rc *RegistryCenter) GetRegistration(protocol string) (*CollectorRegistration, bool) {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	reg, exists := rc.registrations[protocol]
	if !exists {
		return nil, false
	}

	return &CollectorRegistration{
		Protocol: reg.Protocol,
		Factory:  reg.Factory,
		Enabled:  reg.Enabled,
		Priority: reg.Priority,
	}, true
}

// SetEnabled 设置协议启用状态
func (rc *RegistryCenter) SetEnabled(protocol string, enabled bool) bool {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if reg, exists := rc.registrations[protocol]; exists {
		reg.Enabled = enabled
		return true
	}
	return false
}

// GetEnabledProtocols 获取启用的协议列表
func (rc *RegistryCenter) GetEnabledProtocols() []string {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	var enabled []string
	for protocol, reg := range rc.registrations {
		if reg.Enabled {
			enabled = append(enabled, protocol)
		}
	}
	return enabled
}

// GetAllProtocols 获取所有协议列表
func (rc *RegistryCenter) GetAllProtocols() []string {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	protocols := make([]string, 0, len(rc.registrations))
	for protocol := range rc.registrations {
		protocols = append(protocols, protocol)
	}
	return protocols
}

// CreateCollectors 创建所有启用的采集器实例
func (rc *RegistryCenter) CreateCollectors(logger logger.Logger) (map[string]basic.AbstractCollector, error) {
	registrations := rc.GetRegistrations()

	// 按优先级排序
	sortedRegs := make([]*CollectorRegistration, 0)
	for _, reg := range registrations {
		if reg.Enabled {
			sortedRegs = append(sortedRegs, reg)
		}
	}

	// 简单排序
	for i := 0; i < len(sortedRegs)-1; i++ {
		for j := i + 1; j < len(sortedRegs); j++ {
			if sortedRegs[i].Priority > sortedRegs[j].Priority {
				sortedRegs[i], sortedRegs[j] = sortedRegs[j], sortedRegs[i]
			}
		}
	}

	collectors := make(map[string]basic.AbstractCollector)

	for _, reg := range sortedRegs {
		collector := reg.Factory(logger.WithName("collector-" + reg.Protocol))
		if collector == nil {
			logger.Error(fmt.Errorf("工厂函数返回nil"), "采集器创建失败", "protocol", reg.Protocol)
			continue
		}

		// 验证协议匹配
		if collector.SupportProtocol() != reg.Protocol {
			logger.Error(fmt.Errorf("协议不匹配"), "注册协议与实际协议不符",
				"expected", reg.Protocol,
				"actual", collector.SupportProtocol())
			continue
		}

		collectors[reg.Protocol] = collector
		logger.Info("采集器创建成功",
			"protocol", reg.Protocol,
			"priority", reg.Priority)
	}

	logger.Info("采集器创建完成",
		"成功", len(collectors),
		"总数", len(sortedRegs))

	return collectors, nil
}

// 全局便捷函数
// RegisterCollectorFactory 注册采集器工厂到全局注册中心
func RegisterCollectorFactory(protocol string, factory CollectorFactory, options ...RegistrationOption) {
	globalCenter.RegisterFactory(protocol, factory, options...)
}

// GetRegisteredProtocols 获取全局注册中心的协议列表
func GetRegisteredProtocols() []string {
	return globalCenter.GetAllProtocols()
}

// GetEnabledProtocols 获取全局注册中心启用的协议列表
func GetEnabledProtocols() []string {
	return globalCenter.GetEnabledProtocols()
}

// EnableProtocol 启用协议
func EnableProtocol(protocol string, logger logger.Logger) bool {
	if globalCenter.SetEnabled(protocol, true) {
		logger.Info("启用协议", "protocol", protocol)
		return true
	}
	logger.Info("协议未注册，无法启用", "protocol", protocol)
	return false
}

// DisableProtocol 禁用协议
func DisableProtocol(protocol string, logger logger.Logger) bool {
	if globalCenter.SetEnabled(protocol, false) {
		logger.Info("禁用协议", "protocol", protocol)
		return true
	}
	logger.Info("协议未注册，无法禁用", "protocol", protocol)
	return false
}
