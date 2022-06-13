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
	"flag"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	DebugLvl = 1
)

// InitLoggers Configure zap backend for controller-runtime logger.
func InitLoggers(development bool, fs *flag.FlagSet) {

	opts := crzap.Options{ZapOpts: []zap.Option{zap.WithCaller(true), zap.AddCallerSkip(-2)}}
	opts.BindFlags(fs)
	opts.Development = development

	// set everything up such that we can use the same logger in controller runtime zap.L().*
	logger := crzap.NewRaw(crzap.UseFlagOptions(&opts))
	_ = zap.ReplaceGlobals(logger)
	ctrl.SetLogger(zapr.NewLogger(logger))
}

// TimeTrack used to time any function
// Example:
//  {
//    defer logs.TimeTrack(lg, time.Now(), "fetch all github repositories")
//  }
func TimeTrack(log logr.Logger, start time.Time, name string) {
	elapsed := time.Since(start)
	log.V(DebugLvl).Info("Time took", "name", name, "time", elapsed)
}
