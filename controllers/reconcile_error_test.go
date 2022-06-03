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

	"github.com/stretchr/testify/assert"
)

func TestAggregatedError(t *testing.T) {
	e := NewAggregatedError(errors.New("a"), errors.New("b"), errors.New("c"))
	assert.Equal(t, "a, b, c", e.Error())
}

func TestAggregatedError_Add(t *testing.T) {
	e := NewAggregatedError()

	assert.Equal(t, "", e.Error())

	e.Add(errors.New("a"))
	assert.Equal(t, "a", e.Error())

	e.Add(errors.New("b"))
	assert.Equal(t, "a, b", e.Error())

	e.Add(errors.New("c"))
	assert.Equal(t, "a, b, c", e.Error())
}
