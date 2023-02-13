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

package bindings

// LinkAnnotation is used to associate the binding to the service account (even the referenced service accounts get annotated by this so that we can clean up their secret lists when the binding is deleted).
const LinkAnnotation = "spi.appstudio.redhat.com/linked-access-token-binding" //#nosec G101 -- false positive, this is just a label

// ManagedByLabel marks the other objects as managed by SPI. Meaning that their lifecycle is bound
// to the lifecycle of some SPI binding.
const ManagedByLabel = "spi.appstudio.redhat.com/managed-by-binding"
