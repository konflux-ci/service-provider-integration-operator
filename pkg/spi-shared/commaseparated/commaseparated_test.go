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

package commaseparated

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstructor(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		cs := Value("kachny")
		assert.Equal(t, "kachny", cs.String())
	})

	t.Run("interprets comma", func(t *testing.T) {
		cs := Value("a,b")
		assert.Equal(t, 2, cs.Len())
	})

	t.Run("strips spaces", func(t *testing.T) {
		cs := Value("a,b")
		assert.Equal(t, 2, cs.Len())
		assert.Equal(t, "a,b", cs.String())
	})

	t.Run("deduplicates", func(t *testing.T) {
		cs := Value("kachny,kachny")
		assert.Equal(t, 1, cs.Len())
	})
}

func TestAdd(t *testing.T) {
	t.Run("add to empty", func(t *testing.T) {
		cs := Empty()
		cs.Add("a")
		assert.Equal(t, 1, cs.Len())
		assert.Equal(t, "a", cs.String())

	})

	t.Run("simple", func(t *testing.T) {
		cs := Value("a")
		cs.Add("b")
		assert.Equal(t, 2, cs.Len())
		assert.Equal(t, "a,b", cs.String())
	})

	t.Run("trims space", func(t *testing.T) {
		cs := Value("a")
		cs.Add("  b ")
		assert.Equal(t, 2, cs.Len())
		assert.Equal(t, "a,b", cs.String())
	})

	t.Run("ignores duplicates", func(t *testing.T) {
		cs := Value("a")
		cs.Add("a")
		assert.Equal(t, 1, cs.Len())
		assert.Equal(t, "a", cs.String())
	})
}

func TestRemove(t *testing.T) {
	t.Run("from empty", func(t *testing.T) {})
	t.Run("first", func(t *testing.T) {})
	t.Run("middle", func(t *testing.T) {})
	t.Run("last", func(t *testing.T) {})
}

func TestContains(t *testing.T) {
	cs := Value("a,b,c")
	assert.True(t, cs.Contains("a"))
	assert.True(t, cs.Contains("b"))
	assert.True(t, cs.Contains("c"))
	assert.False(t, cs.Contains("d"))
}
