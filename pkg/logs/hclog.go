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
	"io"
	"log"

	"github.com/go-logr/zapr"

	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewHCLogAdapter creates a new adapter, wrapping an underlying
// zap.Logger inside an implementation that emulates hclog.Logger.
func NewHCLogAdapter(wrapped *zap.Logger) *HCLogAdapter {
	return &HCLogAdapter{
		name: "",
		zap:  wrapped,
	}
}

// HCLogAdapter is an adapter that allows to use a zap.Logger where
// and hclog.Logger is expected.
type HCLogAdapter struct {
	name string
	zap  *zap.Logger
}

// This variable is a guard to ensure that HCLogAdapter actually satisfies the hclog.Logger interface
var _ hclog.Logger = (*HCLogAdapter)(nil)

// This variable is a guard to ensure that HCLogAdapter actually satisfies the zapr.Underlier interface
var _ zapr.Underlier = (*HCLogAdapter)(nil)

func (l *HCLogAdapter) Clone() *HCLogAdapter {
	return &HCLogAdapter{
		name: l.name,
		zap:  l.zap,
	}
}

func (l *HCLogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace, hclog.Debug:
		l.zap.Debug(msg, toZapAny(args...)...)
	case hclog.NoLevel, hclog.Info:
		l.zap.Info(msg, toZapAny(args...)...)
	case hclog.Warn:
		l.zap.Warn(msg, toZapAny(args...)...)
	case hclog.Error:
		l.zap.Error(msg, toZapAny(args...)...)
	case hclog.Off:
		// do nothing
	}
}

// Trace - emit a message and key/value pairs at the TRACE level
func (l *HCLogAdapter) Trace(msg string, args ...interface{}) {
	l.zap.Debug(msg, toZapAny(args...)...)
}

// Debug - emit a message and key/value pairs at the DEBUG level
func (l *HCLogAdapter) Debug(msg string, args ...interface{}) {
	l.zap.Debug(msg, toZapAny(args...)...)
}

// Info - emit a message and key/value pairs at the INFO level
func (l *HCLogAdapter) Info(msg string, args ...interface{}) {
	l.zap.Info(msg, toZapAny(args...)...)
}

// Warn - emit a message and key/value pairs at the WARN level
func (l *HCLogAdapter) Warn(msg string, args ...interface{}) {
	l.zap.Warn(msg, toZapAny(args...)...)
}

// Error - emit a message and key/value pairs at the ERROR level
func (l *HCLogAdapter) Error(msg string, args ...interface{}) {
	l.zap.Error(msg, toZapAny(args...)...)
}

func toZapAny(args ...interface{}) []zapcore.Field {
	var fields []zapcore.Field
	for i := 0; i < len(args); i += 2 {
		fields = append(fields, zap.Any(args[i].(string), args[i+1]))
	}
	return fields
}
func (l *HCLogAdapter) IsTrace() bool {
	return l.zap.Core().Enabled(zap.DebugLevel)
}

func (l *HCLogAdapter) IsDebug() bool {
	return l.zap.Core().Enabled(zap.DebugLevel)
}

func (l *HCLogAdapter) IsInfo() bool {
	return l.zap.Core().Enabled(zap.InfoLevel)
}

func (l *HCLogAdapter) IsWarn() bool {
	return l.zap.Core().Enabled(zap.WarnLevel)
}

func (l *HCLogAdapter) IsError() bool {
	return l.zap.Core().Enabled(zap.ErrorLevel)
}

// ImpliedArgs has no implementation.
func (l *HCLogAdapter) ImpliedArgs() []interface{} {
	return nil
}

// With returns a logger with always-presented key-value pairs.
func (l *HCLogAdapter) With(args ...interface{}) hclog.Logger {
	return NewHCLogAdapter(l.zap.With(toZapAny(args...)...))
}

// Name returns a logger's name (if presented).
func (l *HCLogAdapter) Name() string {
	return l.name
}

// Named returns a logger with the specific name.
func (l *HCLogAdapter) Named(name string) hclog.Logger {
	nl := l.Clone()
	nl.name = name
	return nl
}

// ResetNamed has the same implementation as Named.
func (l *HCLogAdapter) ResetNamed(name string) hclog.Logger {
	nl := l.Clone()
	nl.name = name
	return nl
}

// SetLevel has no implementation.
func (l *HCLogAdapter) SetLevel(level hclog.Level) {
}

// GetLevel has no implementation.
func (l *HCLogAdapter) GetLevel() hclog.Level {
	switch l.zap.Level() {
	case zapcore.DebugLevel:
		return hclog.Debug
	case zapcore.InfoLevel:
		return hclog.Info
	case zapcore.WarnLevel:
		return hclog.Warn
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return hclog.Error
	default:
		return hclog.Off

	}
}

func (l *HCLogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return zap.NewStdLog(l.zap)
}

// StandardWriter returns os.Stderr as io.Writer.
func (l *HCLogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return hclog.DefaultOutput
}

func (l *HCLogAdapter) GetUnderlying() *zap.Logger {
	return l.zap
}
