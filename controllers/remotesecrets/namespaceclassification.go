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

package remotesecrets

import api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

type SpecTargetIndex int
type StatusTargetIndex int

// NamespaceClassification contains the results of the ClassifyTargetNamespaces function.
type NamespaceClassification struct {
	// Sync contains the targets that should be synced. The keys in the map are indices to
	// the target array in the remote secret spec and the values are indices to targets array
	// in the remote secret status. These targets were determined to point to the same namespace
	// and therefore should correspond to the same secret that should be synced according to
	// the remote secret spec.
	Sync map[SpecTargetIndex]StatusTargetIndex
	// Remove is an array of indices in the target array of remote secret status that were determined
	// to longer have a counterpart in the target specs and therefore should be removed from
	// the target namespaces.
	Remove []StatusTargetIndex
}

// ClassifyTargetNamespaces looks at the targets specified in the remote secret spec and the targets
// in the remote secret status and, based on the namespace they each specify, classifies them
// to two groups - the ones that should be synced and the ones that should be removed from the target
// namespaces. Because this is done based on the namespace names, it is robust against the reordering
// of the targets in either the spec or the status of the remote secret.
// Targets in spec that do not have a corresponding status (i.e. the new targets that have not yet been
// deployed to) have the status index set to -1 in the returned classification's Sync map.
func ClassifyTargetNamespaces(rs *api.RemoteSecret) NamespaceClassification {
	specIndices := specNamespaceIndices(rs.Spec.Targets)
	statusIndices := statusNamespaceIndices(rs.Status.Targets)

	ret := NamespaceClassification{
		Sync:   map[SpecTargetIndex]StatusTargetIndex{},
		Remove: []StatusTargetIndex{},
	}

	for specNs, specIdx := range specIndices {
		stIdx, ok := statusIndices[specNs]
		if ok {
			ret.Sync[specIdx] = stIdx
			delete(statusIndices, specNs)
		} else {
			ret.Sync[specIdx] = -1
		}
	}

	for _, stIdx := range statusIndices {
		ret.Remove = append(ret.Remove, stIdx)
	}

	return ret
}

func specNamespaceIndices(targets []api.RemoteSecretTarget) map[string]SpecTargetIndex {
	ret := make(map[string]SpecTargetIndex, len(targets))
	for i, t := range targets {
		ret[t.Namespace] = SpecTargetIndex(i)
	}
	return ret
}

func statusNamespaceIndices(targets []api.TargetStatus) map[string]StatusTargetIndex {
	ret := make(map[string]StatusTargetIndex, len(targets))
	for i, t := range targets {
		ret[t.Namespace] = StatusTargetIndex(i)
	}
	return ret
}
