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
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/hashicorp/go-hclog"

	"k8s.io/klog/v2"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	DebugLevel = 1
)

// InitDevelLoggers Configure zap backend development logger
func InitDevelLoggers() {
	InitLoggers(true, "", "", "", "iso8601")
}

// InitLoggers Configure zap backend for controller-runtime logger.
func InitLoggers(development bool, encoder string, logLevel string, stackTraceLevel string, timeEncoding string) {

	flagSet := flag.NewFlagSet("zap", flag.ContinueOnError)

	opts := crzap.Options{ZapOpts: []zap.Option{zap.WithCaller(true), zap.AddCallerSkip(-1)}}
	opts.BindFlags(flagSet)

	setFlagIfNotEmptyOrPanic(flagSet, "zap-devel", strconv.FormatBool(development))
	setFlagIfNotEmptyOrPanic(flagSet, "zap-encoder", encoder)
	setFlagIfNotEmptyOrPanic(flagSet, "zap-log-level", logLevel)
	setFlagIfNotEmptyOrPanic(flagSet, "zap-stacktrace-level", stackTraceLevel)
	setFlagIfNotEmptyOrPanic(flagSet, "zap-time-encoding", timeEncoding)

	// set everything up such that we can use the same logger in controller runtime zap.L().*
	logger := crzap.NewRaw(crzap.UseFlagOptions(&opts))
	_ = zap.ReplaceGlobals(logger)
	lg := zapr.NewLogger(logger).WithCallDepth(1)
	ctrl.SetLogger(lg)
	klog.SetLoggerWithOptions(lg, klog.ContextualLogger(true))
	hclog.SetDefault(NewHCLogAdapter(logger.WithOptions(zap.AddCallerSkip(1))))
}

func setFlagIfNotEmptyOrPanic(fs *flag.FlagSet, name, value string) {
	if len(value) > 0 {
		err := fs.Set(name, value)
		if err != nil {
			panic(err)
		}
	}
}

// TimeTrack used to time any function
// Example:
//
//	{
//	  defer logs.TimeTrack(lg, time.Now(), "fetch all github repositories")
//	}
func TimeTrack(log logr.Logger, start time.Time, name string) {
	elapsed := time.Since(start)
	log.V(DebugLevel).Info(fmt.Sprintf("Time took to %s", name), "time", elapsed)
}

// TimeTrackWithLazyLogger is very similar to TimeTrack. The only difference is that it obtains the logger lazily
// which enables the caller to use the logger instance as assigned at the very end of the function being tracked.
// Example:
//
//	{
//	  lg := log.FromContext(ctx)
//	  defer logs.TimeTrackWithLazyLogger(func() logr.Logger {return lg}, time.Now(), "this is hard work, man!")
//	  lg = lg.WithValues("work", "really hard")
//	  ...
//	}
//
// The log message produced by the time tracker will contain the "work" => "really hard" key-value pair.
func TimeTrackWithLazyLogger(loggerGetter func() logr.Logger, start time.Time, name string) {
	TimeTrack(loggerGetter(), start, name)
}

// AuditLog returns logger prepared with audit markers
func AuditLog(ctx context.Context) logr.Logger {
	return log.FromContext(ctx, "audit", "true")
}
