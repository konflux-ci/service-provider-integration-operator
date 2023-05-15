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

package integrationtests

import (
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
)

var _ = Describe("RemoteSecret", func() {
	Describe("Create", func() {
		test := TestSetup{}

		BeforeEach(func() {
			test.BeforeEach(nil)
		})

		AfterEach(func() {
		})

		When("no targets", func() {
			It("succeeds", func() {
			})
		})

		When("with targets", func() {
		})
	})

	Describe("Update", func() {
		When("target removed", func() {
		})

		When("target added", func() {
		})

		When("secret spec changed", func() {
		})

		When("linked SAs changed", func() {
		})
	})

	Describe("Delete", func() {
		When("no targets present", func() {
		})

		When("targets present", func() {
		})
	})
})
