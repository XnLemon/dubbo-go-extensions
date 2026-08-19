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

// Package hystrix provides a typed, resource-aware Hystrix extension.
package hystrix

import (
	"context"
)

import (
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"

	afexhystrix "github.com/afex/hystrix-go/hystrix"

	"github.com/dubbogo/gost/log/logger"
)

// FilterError reports a Hystrix fallback and whether the circuit was open.
type FilterError struct {
	err         error
	circuitOpen bool
}

func (e *FilterError) Error() string     { return e.err.Error() }
func (e *FilterError) Unwrap() error     { return e.err }
func (e *FilterError) CircuitOpen() bool { return e.circuitOpen }

func NewHystrixFilterError(err error, circuitOpen bool) error {
	return &FilterError{err: err, circuitOpen: circuitOpen}
}

// Filter applies the resource-local Hystrix runtime selected during Dial or
// Register. Method-level settings are resolved on first invocation.
type Filter struct {
	runtime *resourceRuntime
}

func (f *Filter) Invoke(ctx context.Context, invoker base.Invoker, invocation base.Invocation) result.Result {
	command, err := f.runtime.command(invocation.MethodName())
	if err != nil {
		response := &result.RPCResult{}
		response.SetError(err)
		return response
	}

	responses := make(chan result.Result, 1)
	err = afexhystrix.DoC(ctx, command.name, func(runCtx context.Context) error {
		response := invoker.Invoke(runCtx, invocation)
		responses <- response
		if response == nil {
			return nil
		}
		return response.Error()
	}, nil)
	if err != nil {
		_, circuitOpen := err.(afexhystrix.CircuitError)
		if circuitOpen {
			logger.Debugf("[Hystrix Filter] circuit opened for %s", command.name)
		} else {
			logger.Debugf("[Hystrix Filter] fallback for %s: %v", command.name, err)
		}
		response := &result.RPCResult{}
		response.SetError(NewHystrixFilterError(err, circuitOpen))
		return response
	}
	return <-responses
}

func (f *Filter) OnResponse(_ context.Context, response result.Result, _ base.Invoker, _ base.Invocation) result.Result {
	return response
}
