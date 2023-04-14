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

import "strings"

// CommaSeparated represents a set of non-empty strings that can be
// constructed from and serialized into a comma-separated string.
type CommaSeparated struct {
	value []string
}

func Empty() *CommaSeparated {
	return &CommaSeparated{}
}

func Value(str string) *CommaSeparated {
	ret := &CommaSeparated{}
	for _, v := range strings.Split(str, ",") {
		ret = ret.Add(v)
	}
	return ret
}

func (cs *CommaSeparated) Len() int {
	return len(cs.value)
}

func (cs *CommaSeparated) Add(str string) *CommaSeparated {
	str = strings.TrimSpace(str)
	for _, v := range strings.Split(str, ",") {
		if cs.Contains(v) {
			continue
		}

		if len(v) > 0 {
			cs.value = append(cs.value, v)
		}
	}

	return cs
}

func (cs *CommaSeparated) Remove(str string) *CommaSeparated {
	removeIdx := -1
	for i, v := range cs.value {
		if v == str {
			removeIdx = i
			break
		}
	}

	if removeIdx != -1 {
		if removeIdx == 0 {
			cs.value = cs.value[1:]
		} else {
			// we're re-ordering the set here, but that really doesn't make much difference, because sets are unordered anyway.
			lastIdx := len(cs.value) - 1
			if removeIdx != lastIdx {
				cs.value[removeIdx] = cs.value[lastIdx]
			}
			cs.value = cs.value[:lastIdx]
		}

	}

	return cs
}

func (cs *CommaSeparated) Contains(str string) bool {
	str = strings.TrimSpace(str)
	for _, v := range cs.value {
		if v == str {
			return true
		}
	}

	return false
}

func (cs *CommaSeparated) String() string {
	return strings.Join(cs.value, ",")
}

func (cs *CommaSeparated) Values() []string {
	return cs.value
}
