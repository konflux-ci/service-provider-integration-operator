//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs

import (
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/klog/v2"
)

func TestInitDevelLoggers(t *testing.T) {

	//Ensure preconditions.
	assert.Equal(t, zap.L(), zap.NewNop())
	klog.ClearLogger()
	//when
	InitDevelLoggers()
	klog.ContextualLogger(true)
	//then
	assert.NotEqual(t, zap.L(), zap.NewNop())
	assert.IsType(t, &zap.Logger{}, klog.Background().GetSink().(zapr.Underlier).GetUnderlying())
	assert.IsType(t, &zap.Logger{}, hclog.Default().(zapr.Underlier).GetUnderlying())

}

func TestTimeTrack(t *testing.T) {
	//given
	fac, logs := observer.New(zap.DebugLevel)
	logger := zapr.NewLogger(zap.New(fac))
	//when
	TimeTrack(logger, time.Now().Add(time.Duration(-5)*time.Second), "MSG")
	//then
	output := logs.AllUntimed()
	assert.Equal(t, 1, len(output), "Unexpected number of logs.")
	assert.Equal(t, 1, len(output[0].Context), "Unexpected context on first log.")
	assert.Equal(t, zapcore.DurationType, output[0].Context[0].Type, "Unexpected context type")
	assert.Equal(t, "time", output[0].Context[0].Key, "Unexpected context key")
	assert.True(t, output[0].Context[0].Integer > 0, "Unexpected context value %n", output[0].Context[0].Integer)

	assert.Equal(
		t,
		zapcore.Entry{Level: zapcore.DebugLevel, Message: "Time took to MSG"},
		output[0].Entry,
		"Unexpected output from %s-level logger method.", zapcore.DebugLevel)
}
