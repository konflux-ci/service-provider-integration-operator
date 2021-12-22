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

package controllers

import "fmt"

var (
	_ error = (*ReconcileError)(nil)
)

// ReconcileError is just a common error type for reconciliation errors that contains a cause and can be unwrapped so
// that a full stacktrace is produced in the logs.
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
