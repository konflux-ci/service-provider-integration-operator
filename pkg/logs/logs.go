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

// InitLoggers Configure zap backend for controller-runtime logger.
func InitLoggers(development bool, fs *flag.FlagSet) {

	opts := crzap.Options{ZapOpts: []zap.Option{zap.WithCaller(true), zap.AddCallerSkip(-1)}}
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
	log.V(1).Info("Time took", "name", name, "time", elapsed)
}
