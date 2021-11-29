package controllers

import "fmt"

var (
	_ error = (*ReconcileError)(nil)
)

type ReconcileError struct {
	message string
	cause   error
}

func NewReconcileError(err error, format string, args ...interface{}) *ReconcileError {
	if format == "" {
		format = "reconciliation failed"
	}
	return &ReconcileError{
		message: fmt.Sprintf(format, args...),
		cause:   err,
	}
}

func (e *ReconcileError) Error() string {
	if e.cause == nil {
		return e.message
	}

	return e.message + ": " + e.cause.Error()
}

func (e *ReconcileError) Unwrap() error {
	return e.cause
}
