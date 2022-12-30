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

	"github.com/hashicorp/go-hclog"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestHCLogAdapter_LogName(t *testing.T) {
	withLogger(zap.DebugLevel, nil, func(logger *HCLogAdapter, logs *observer.ObservedLogs) {
		tests := []struct {
			method        func(string, ...interface{})
			expectedLevel zapcore.Level
		}{
			{logger.Trace, zap.DebugLevel},
			{logger.Debug, zap.DebugLevel},
			{logger.Info, zap.InfoLevel},
			{logger.Warn, zap.WarnLevel},
			{logger.Error, zap.ErrorLevel},
		}
		for i, tt := range tests {
			tt.method("Message", "param", "pam-pam")
			output := logs.AllUntimed()
			assert.Equal(t, i+1, len(output), "Unexpected number of logs.")
			assert.Equal(t, 1, len(output[i].Context), "Unexpected context on first log.")
			assert.Equal(t, zapcore.Field{
				Type:   zapcore.StringType,
				Key:    "param",
				String: "pam-pam",
			}, output[i].Context[0], "Unexpected context on first log.")
			assert.Equal(
				t,
				zapcore.Entry{Level: tt.expectedLevel, Message: "Message"},
				output[i].Entry,
				"Unexpected output from %s-level logger method.", tt.expectedLevel)
		}
	})
}

func TestHCLogAdapter_Log(t *testing.T) {
	withLogger(zap.DebugLevel, nil, func(logger *HCLogAdapter, logs *observer.ObservedLogs) {
		tests := []struct {
			level         hclog.Level
			expectedLevel zapcore.Level
		}{
			{hclog.Trace, zap.DebugLevel},
			{hclog.Debug, zap.DebugLevel},
			{hclog.Info, zap.InfoLevel},
			{hclog.NoLevel, zap.InfoLevel},
			{hclog.Warn, zap.WarnLevel},
			{hclog.Error, zap.ErrorLevel},
			//{hclog.Off, zap.ErrorLevel},
		}
		for i, tt := range tests {
			logger.Log(tt.level, "Message", "param", "pam-pam")
			output := logs.AllUntimed()
			assert.Equal(t, i+1, len(output), "Unexpected number of logs.")
			assert.Equal(t, 1, len(output[i].Context), "Unexpected context on first log.")
			assert.Equal(t, zapcore.Field{
				Type:   zapcore.StringType,
				Key:    "param",
				String: "pam-pam",
			}, output[i].Context[0], "Unexpected context on first log.")
			assert.Equal(
				t,
				zapcore.Entry{Level: tt.expectedLevel, Message: "Message"},
				output[i].Entry,
				"Unexpected output from %s-level logger method.", tt.expectedLevel)
		}
	})
}

func TestHCLogAdapter_GetLevel(t *testing.T) {
	tests := []struct {
		testLevel     zapcore.Level
		expectedLevel hclog.Level
	}{
		{zapcore.DebugLevel, hclog.Debug},
		{zapcore.InfoLevel, hclog.Info},
		{zapcore.WarnLevel, hclog.Warn},
		{zapcore.ErrorLevel, hclog.Error},
		{zapcore.DPanicLevel, hclog.Error},
		{zapcore.PanicLevel, hclog.Error},
		{zapcore.FatalLevel, hclog.Error},
	}
	for _, tt := range tests {
		withLogger(tt.testLevel, nil, func(logger *HCLogAdapter, logs *observer.ObservedLogs) {
			assert.Equal(t, tt.expectedLevel, logger.GetLevel(), "Unexpected output %s-level from GetLevel method.", tt.expectedLevel)
		})
	}
}

func TestHCLogAdapter_ShouldDiscardOffLevel(t *testing.T) {
	withLogger(zap.DebugLevel, nil, func(logger *HCLogAdapter, logs *observer.ObservedLogs) {
		logger.Log(hclog.Off, "Message", "param", "pam-pam")
		output := logs.AllUntimed()
		assert.Equal(t, 0, len(output), "Unexpected number of logs.")
	})
}
func withLogger(e zapcore.LevelEnabler, opts []zap.Option, f func(*HCLogAdapter, *observer.ObservedLogs)) {
	fac, logs := observer.New(e)
	log := NewHCLogAdapter(zap.New(fac, opts...))
	f(log, logs)
}
