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
	"context"
	"errors"
	"testing"
	"time"
)

import (
	"dubbo.apache.org/dubbo-go/v3/client"
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/protocol/base"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/protocol/result"
	"dubbo.apache.org/dubbo-go/v3/server"

	afexhystrix "github.com/afex/hystrix-go/hystrix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCommand = "greet.GreetService:::Greet"

func TestNewHystrixFilterError(t *testing.T) {
	underlying := errors.New("test")
	err := NewHystrixFilterError(underlying, true)
	assert.True(t, err.(*FilterError).CircuitOpen())
	assert.Equal(t, "test", err.Error())
	assert.ErrorIs(t, err, underlying)
}

func TestTypedOptionsOverrideYAMLWithoutSplittingCommandKey(t *testing.T) {
	root, err := extension.NewRawNode(map[string]any{
		"consumer": map[string]any{
			testCommand: map[string]any{
				"timeout":                  250,
				"max-concurrent-requests":  3,
				"request-volume-threshold": 4,
				"sleep-window":             500,
				"error-percent-threshold":  60,
			},
		},
	})
	require.NoError(t, err)
	plan := extension.NewPlan(func(prefix string, selectedKeys ...string) (extension.RawConfig, bool, error) {
		assert.Equal(t, Prefix, prefix)
		return extension.BuildRawConfig(root, selectedKeys...), true, nil
	}, WithConfig(
		WithCommandName(testCommand),
		WithTimeout(1000),
	)).WithRawConfigPrefixes(Prefix)

	runtime, err := plan.Build(extension.ClientScope, common.CONSUMER)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	ctx, ok := runtime.Context(Prefix)
	require.True(t, ok)
	config := ctx.Config.(*runtimeConfig)

	command, ok := config.Commands[testCommand]
	require.True(t, ok)
	assert.Equal(t, 1000, command.Timeout)
	assert.Equal(t, 3, command.MaxConcurrentRequests)
	assert.Equal(t, 4, command.RequestVolumeThreshold)
	assert.Equal(t, 500, command.SleepWindow)
	assert.Equal(t, 60, command.ErrorPercentThreshold)
}

func TestBindResourceCreatesAutomaticFilterAndMethodSettings(t *testing.T) {
	runtime := buildClientRuntime(t, WithConfig(
		WithCommandName(testCommand),
		WithTimeout(321),
		WithMaxConcurrentRequests(7),
	))
	resource := extension.Resource{
		ServiceKey: "greet.GreetService",
		Interface:  "greet.GreetService",
	}

	specs, err := runtime.BindResource(resource)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, filterID, specs[0].ID)
	filter, ok := specs[0].Factory().(*Filter)
	require.True(t, ok)
	inv := invocation.NewRPCInvocation("Greet", []any{"OK"}, nil)
	response := filter.Invoke(context.Background(), base.NewBaseInvoker(&common.URL{}), inv)
	require.NotNil(t, response)
	require.NoError(t, response.Error())

	command, err := filter.runtime.command("Greet")
	require.NoError(t, err)
	settings := afexhystrix.GetCircuitSettings()[command.name]
	require.NotNil(t, settings)
	assert.Equal(t, int64(321), settings.Timeout.Milliseconds())
	assert.Equal(t, 7, settings.MaxConcurrentRequests)
}

func TestRuntimeAndResourceCommandNamesAreIsolated(t *testing.T) {
	firstRuntime := buildClientRuntime(t, WithConfig())
	secondRuntime := buildClientRuntime(t, WithConfig())
	firstResource := extension.Resource{ServiceKey: "payment.PaymentService", Interface: "payment.PaymentService"}
	secondResource := extension.Resource{ServiceKey: "user.UserService", Interface: "user.UserService"}

	firstPayment := bindFilter(t, firstRuntime, firstResource)
	secondPayment := bindFilter(t, secondRuntime, firstResource)
	firstUser := bindFilter(t, firstRuntime, secondResource)
	firstName, err := firstPayment.runtime.command("Pay")
	require.NoError(t, err)
	secondName, err := secondPayment.runtime.command("Pay")
	require.NoError(t, err)
	userName, err := firstUser.runtime.command("Pay")
	require.NoError(t, err)

	assert.NotEqual(t, firstName.name, secondName.name)
	assert.NotEqual(t, firstName.name, userName.name)
}

func TestClosedRuntimeRejectsInvocation(t *testing.T) {
	runtime := buildClientRuntime(t, WithConfig())
	filter := bindFilter(t, runtime, extension.Resource{
		ServiceKey: "greet.GreetService",
		Interface:  "greet.GreetService",
	})
	require.NoError(t, runtime.Close())

	response := filter.Invoke(
		context.Background(),
		base.NewBaseInvoker(&common.URL{}),
		invocation.NewRPCInvocation("Greet", nil, nil),
	)
	require.Error(t, response.Error())
	assert.Contains(t, response.Error().Error(), "closed")
}

type slowInvoker struct {
	*base.BaseInvoker
}

func (i *slowInvoker) Invoke(context.Context, base.Invocation) result.Result {
	time.Sleep(20 * time.Millisecond)
	return &result.RPCResult{}
}

func TestTimeoutDoesNotShareResponseWithLateInvocation(t *testing.T) {
	runtime := buildClientRuntime(t, WithConfig(
		WithTimeout(1),
	))
	filter := bindFilter(t, runtime, extension.Resource{
		ServiceKey: "slow.Service",
		Interface:  "slow.Service",
	})
	response := filter.Invoke(
		context.Background(),
		&slowInvoker{BaseInvoker: base.NewBaseInvoker(&common.URL{})},
		invocation.NewRPCInvocation("Slow", nil, nil),
	)
	require.Error(t, response.Error())
	time.Sleep(30 * time.Millisecond)
}

func TestConfigOptionValidation(t *testing.T) {
	_, err := extension.NewPlan(nil, WithConfig(WithTimeout(0))).Build(extension.ClientScope, common.CONSUMER)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "greater than zero")
}

func TestClientTypedExtensionAPI(t *testing.T) {
	cli, err := client.NewClient(client.WithExtension(WithConfig(
		WithCommandName(testCommand),
		WithTimeout(1000),
		WithMaxConcurrentRequests(10),
		WithRequestVolumeThreshold(5),
		WithSleepWindow(5000),
		WithErrorPercentThreshold(50),
	)))
	require.NoError(t, err)
	require.NoError(t, cli.CloseExtensions())
}

func TestServerTypedExtensionAPI(t *testing.T) {
	srv, err := server.NewServer(server.WithExtension(WithConfig(
		WithCommandName(testCommand),
		WithTimeout(1500),
	)))
	require.NoError(t, err)
	require.NoError(t, srv.CloseExtensions())
}

func buildClientRuntime(t *testing.T, option extension.Option) *extension.Runtime {
	t.Helper()
	runtime, err := extension.NewPlan(nil, option).Build(extension.ClientScope, common.CONSUMER)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	return runtime
}

func bindFilter(t *testing.T, runtime *extension.Runtime, resource extension.Resource) *Filter {
	t.Helper()
	specs, err := runtime.BindResource(resource)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	filter, ok := specs[0].Factory().(*Filter)
	require.True(t, ok)
	return filter
}
