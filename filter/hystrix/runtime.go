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

package hystrix

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/filter"

	afexhystrix "github.com/afex/hystrix-go/hystrix"
)

const filterID = "hystrix:circuit-breaker"

var nextRuntimeID atomic.Uint64

type runtimeState struct {
	id     uint64
	role   common.RoleType
	closed atomic.Bool
}

type runtimeConfig struct {
	Config
	state *runtimeState
}

type resourceRuntime struct {
	config   *runtimeConfig
	resource extension.Resource
	commands sync.Map
}

type commandRuntime struct {
	name string
}

func init() {
	extension.MustRegister(extension.Definition{
		Prefix:    Prefix,
		Scopes:    extension.ClientScope | extension.ServerScope,
		NewConfig: newConfig,
		Decode:    decode,
		Init:      initialize,
		Filters:   filters,
		Close:     closeRuntime,
	})
}

func newConfig() any {
	return &runtimeConfig{
		Config: Config{
			Defaults: CommandConfig{
				Timeout:                afexhystrix.DefaultTimeout,
				MaxConcurrentRequests:  afexhystrix.DefaultMaxConcurrent,
				RequestVolumeThreshold: afexhystrix.DefaultVolumeThreshold,
				SleepWindow:            afexhystrix.DefaultSleepWindow,
				ErrorPercentThreshold:  afexhystrix.DefaultErrorPercentThreshold,
			},
			Commands: make(map[string]CommandConfig),
		},
	}
}

func initialize(ctx *extension.Context) error {
	config, ok := ctx.Config.(*runtimeConfig)
	if !ok {
		return fmt.Errorf("hystrix: expected runtime config, got %T", ctx.Config)
	}
	config.currentCommand = ""
	config.state = &runtimeState{
		id:   nextRuntimeID.Add(1),
		role: ctx.Role,
	}
	return nil
}

func filters(ctx *extension.Context) ([]extension.FilterSpec, error) {
	config, ok := ctx.Config.(*runtimeConfig)
	if !ok {
		return nil, fmt.Errorf("hystrix: expected runtime config, got %T", ctx.Config)
	}
	if config.state == nil {
		return nil, fmt.Errorf("hystrix: runtime is not initialized")
	}
	if ctx.Resource == nil {
		return nil, fmt.Errorf("hystrix: resource is required when binding filters")
	}
	resource := *ctx.Resource
	runtime := &resourceRuntime{config: config, resource: resource}
	return []extension.FilterSpec{{
		ID:      filterID,
		Order:   100,
		Factory: func() filter.Filter { return &Filter{runtime: runtime} },
	}}, nil
}

func closeRuntime(ctx *extension.Context) error {
	config, ok := ctx.Config.(*runtimeConfig)
	if !ok {
		return fmt.Errorf("hystrix: expected runtime config, got %T", ctx.Config)
	}
	if config.state != nil {
		config.state.closed.Store(true)
	}
	return nil
}

func (r *resourceRuntime) command(method string) (*commandRuntime, error) {
	if r.config.state.closed.Load() {
		return nil, fmt.Errorf("hystrix: runtime is closed")
	}
	if command, ok := r.commands.Load(method); ok {
		return command.(*commandRuntime), nil
	}
	settings := r.settings(method)
	name := strings.Join([]string{
		"dubbo-extension-hystrix",
		strconv.FormatUint(r.config.state.id, 10),
		strconv.Itoa(int(r.config.state.role)),
		r.resource.ServiceKey,
		method,
	}, "|")
	afexhystrix.ConfigureCommand(name, afexhystrix.CommandConfig{
		Timeout:                settings.Timeout,
		MaxConcurrentRequests:  settings.MaxConcurrentRequests,
		RequestVolumeThreshold: settings.RequestVolumeThreshold,
		SleepWindow:            settings.SleepWindow,
		ErrorPercentThreshold:  settings.ErrorPercentThreshold,
	})
	created := &commandRuntime{name: name}
	actual, _ := r.commands.LoadOrStore(method, created)
	return actual.(*commandRuntime), nil
}

func (r *resourceRuntime) settings(method string) CommandConfig {
	settings := r.config.Defaults
	keys := []string{
		colonKey(r.resource.Interface, r.resource.Group, r.resource.Version, method),
		colonKey(r.resource.Interface, "", "", method),
		r.resource.ServiceKey,
		colonKey(r.resource.Interface, r.resource.Group, r.resource.Version, ""),
		r.resource.Interface,
	}
	for _, key := range keys {
		if configured, ok := r.config.Commands[key]; ok {
			return mergeCommand(settings, configured)
		}
	}
	return settings
}

func colonKey(interfaceName, group, version, method string) string {
	return interfaceName + ":" + group + ":" + version + ":" + method
}

func mergeCommand(defaults, configured CommandConfig) CommandConfig {
	if configured.Timeout != 0 {
		defaults.Timeout = configured.Timeout
	}
	if configured.MaxConcurrentRequests != 0 {
		defaults.MaxConcurrentRequests = configured.MaxConcurrentRequests
	}
	if configured.RequestVolumeThreshold != 0 {
		defaults.RequestVolumeThreshold = configured.RequestVolumeThreshold
	}
	if configured.SleepWindow != 0 {
		defaults.SleepWindow = configured.SleepWindow
	}
	if configured.ErrorPercentThreshold != 0 {
		defaults.ErrorPercentThreshold = configured.ErrorPercentThreshold
	}
	return defaults
}
