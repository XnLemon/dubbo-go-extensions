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
	"bytes"
	"fmt"
	"strings"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common/extension"

	"gopkg.in/yaml.v3"
)

const Prefix = "hystrix"

// CommandConfig contains the Hystrix settings for one service or method key.
// Durations are expressed in milliseconds to match hystrix-go.
type CommandConfig struct {
	Timeout                int `yaml:"timeout"`
	MaxConcurrentRequests  int `yaml:"max-concurrent-requests"`
	RequestVolumeThreshold int `yaml:"request-volume-threshold"`
	SleepWindow            int `yaml:"sleep-window"`
	ErrorPercentThreshold  int `yaml:"error-percent-threshold"`
}

// Config is the typed, runtime-local Hystrix configuration. Commands are
// keyed by the exact service or method key supplied by the user.
type Config struct {
	Defaults CommandConfig
	Commands map[string]CommandConfig

	currentCommand string
}

// ConfigOption configures Hystrix defaults or the command most recently
// selected with WithCommandName.
type ConfigOption func(*Config) error

type option struct {
	options []ConfigOption
}

func (o option) Prefix() string {
	return Prefix
}

func (o option) Apply(value any) error {
	config, ok := value.(*runtimeConfig)
	if !ok {
		return fmt.Errorf("hystrix: expected runtime config, got %T", value)
	}
	config.currentCommand = ""
	defer func() { config.currentCommand = "" }()
	for index, apply := range o.options {
		if apply == nil {
			return fmt.Errorf("hystrix: config option %d is nil", index)
		}
		if err := apply(&config.Config); err != nil {
			return fmt.Errorf("hystrix: apply config option %d: %w", index, err)
		}
	}
	return nil
}

// WithConfig activates Hystrix and applies typed options after YAML.
func WithConfig(options ...ConfigOption) extension.Option {
	return option{options: append([]ConfigOption(nil), options...)}
}

// WithCommandName selects the exact service or method key configured by the
// following options. Dots and colons have no path semantics.
func WithCommandName(name string) ConfigOption {
	return func(config *Config) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("command name is required")
		}
		config.currentCommand = name
		if config.Commands == nil {
			config.Commands = make(map[string]CommandConfig)
		}
		if _, ok := config.Commands[name]; !ok {
			config.Commands[name] = CommandConfig{}
		}
		return nil
	}
}

func WithTimeout(milliseconds int) ConfigOption {
	return setPositive("timeout", milliseconds, func(command *CommandConfig, value int) {
		command.Timeout = value
	})
}

func WithMaxConcurrentRequests(maximum int) ConfigOption {
	return setPositive("max concurrent requests", maximum, func(command *CommandConfig, value int) {
		command.MaxConcurrentRequests = value
	})
}

func WithRequestVolumeThreshold(threshold int) ConfigOption {
	return setPositive("request volume threshold", threshold, func(command *CommandConfig, value int) {
		command.RequestVolumeThreshold = value
	})
}

func WithSleepWindow(milliseconds int) ConfigOption {
	return setPositive("sleep window", milliseconds, func(command *CommandConfig, value int) {
		command.SleepWindow = value
	})
}

func WithErrorPercentThreshold(percent int) ConfigOption {
	return func(config *Config) error {
		if percent <= 0 || percent > 100 {
			return fmt.Errorf("error percent threshold must be between 1 and 100")
		}
		updateSelected(config, func(command *CommandConfig) {
			command.ErrorPercentThreshold = percent
		})
		return nil
	}
}

func setPositive(name string, value int, set func(*CommandConfig, int)) ConfigOption {
	return func(config *Config) error {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", name)
		}
		updateSelected(config, func(command *CommandConfig) { set(command, value) })
		return nil
	}
}

func updateSelected(config *Config, update func(*CommandConfig)) {
	if config.currentCommand == "" {
		update(&config.Defaults)
		return
	}
	command := config.Commands[config.currentCommand]
	update(&command)
	config.Commands[config.currentCommand] = command
}

func decode(raw extension.RawConfig, value any) error {
	config, ok := value.(*runtimeConfig)
	if !ok {
		return fmt.Errorf("hystrix: expected runtime config, got %T", value)
	}
	if raw.Selected == nil || !raw.Selected.Present() {
		return nil
	}
	encoded, err := yaml.Marshal(raw.Selected.Value())
	if err != nil {
		return fmt.Errorf("encode raw config: %w", err)
	}
	commands := make(map[string]CommandConfig)
	decoder := yaml.NewDecoder(bytes.NewReader(encoded))
	decoder.KnownFields(true)
	if err := decoder.Decode(&commands); err != nil {
		return fmt.Errorf("decode commands: %w", err)
	}
	for name, command := range commands {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("command name is required")
		}
		if err := validateCommand(command); err != nil {
			return fmt.Errorf("command %q: %w", name, err)
		}
		config.Commands[trimmed] = command
	}
	return nil
}

func validateCommand(command CommandConfig) error {
	values := []struct {
		name  string
		value int
	}{
		{"timeout", command.Timeout},
		{"max concurrent requests", command.MaxConcurrentRequests},
		{"request volume threshold", command.RequestVolumeThreshold},
		{"sleep window", command.SleepWindow},
	}
	for _, item := range values {
		if item.value < 0 {
			return fmt.Errorf("%s cannot be negative", item.name)
		}
	}
	if command.ErrorPercentThreshold < 0 || command.ErrorPercentThreshold > 100 {
		return fmt.Errorf("error percent threshold must be between 0 and 100")
	}
	return nil
}
