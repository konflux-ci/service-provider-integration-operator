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

import (
	"errors"
	"testing"

	"github.com/redhat-appstudio/remote-secret/pkg/rerror"

	"github.com/stretchr/testify/assert"
)

func TestAggregatedError(t *testing.T) {
	e := rerror.NewAggregatedError(errors.New("a"), errors.New("b"), errors.New("c"))
	assert.Equal(t, "a, b, c", e.Error())
}

func TestAggregatedError_Add(t *testing.T) {
	e := rerror.NewAggregatedError()

	assert.Equal(t, "", e.Error())

	e.Add(errors.New("a"))
	assert.Equal(t, "a", e.Error())

	e.Add(errors.New("b"))
	assert.Equal(t, "a, b", e.Error())

	e.Add(errors.New("c"))
	assert.Equal(t, "a, b, c", e.Error())
}

func TestAggregateNonNilErrors(t *testing.T) {
	t.Run("single nil doesn't count", func(t *testing.T) {
		assert.Nil(t, rerror.AggregateNonNilErrors(nil))
	})

	t.Run("all nils don't count", func(t *testing.T) {
		assert.Nil(t, rerror.AggregateNonNilErrors(nil, nil, nil))
	})

	t.Run("single error returned as is", func(t *testing.T) {
		err := errors.New("asdf")
		assert.Same(t, err, rerror.AggregateNonNilErrors(err))
	})

	t.Run("single error among nils returned as is", func(t *testing.T) {
		err := errors.New("asdf")
		assert.Same(t, err, rerror.AggregateNonNilErrors(nil, err, nil, nil))
	})

	t.Run("multiple errors aggregated", func(t *testing.T) {
		err1 := errors.New("1")
		err2 := errors.New("2")
		agg := rerror.AggregateNonNilErrors(err1, err2)
		assert.IsType(t, &rerror.AggregatedError{}, agg)
		assert.Equal(t, "1, 2", agg.Error())
	})
}
