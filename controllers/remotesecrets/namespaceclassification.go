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

import (
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

type SpecTargetIndex int
type StatusTargetIndex int

// NamespaceClassification contains the results of the ClassifyTargetNamespaces function.
type NamespaceClassification struct {
	// Sync contains the target namespaces that should be synced. The keys in the map are indices to
	// the target array and the nested namespace array in the remote secret spec and the values are
	// indices to targets array in the remote secret status. These targets were determined to point
	// to the same namespace in the same cluster and therefore should correspond to the same secret
	// that should be synced according to the remote secret spec.
	Sync map[SpecTargetIndex]StatusTargetIndex
	// Remove is an array of indices in the target array of remote secret status that were determined
	// to longer have a counterpart in the target specs and therefore should be removed from
	// the target namespaces.
	Remove []StatusTargetIndex
	// DuplicateTargetSpecs contains all the duplicate entries in the spec array with the corresponding
	// duplicate entries in the status, if any.
	// The top level key is the index of the non-duplicate target in the spec. The nested key is
	// the index of the duplicate target(s) and the nested values are indices in the status that
	// correspond to the duplicate spec entries or -1 if the spec doesn't have a corresponding
	// status entry.
	DuplicateTargetSpecs map[SpecTargetIndex]map[SpecTargetIndex]StatusTargetIndex
	// OrphanDuplicateStatuses are indices in the status targets that mark a duplicate entry that
	// has no corresponding spec target. These may appear if for example the user removes a previously
	// reconciled duplicate entry from the spec.
	// These status entries can be safely removed from the array without any further action.
	OrphanDuplicateStatuses []StatusTargetIndex
}

// ClassifyTargetNamespaces looks at the targets specified in the remote secret spec and the targets
// in the remote secret status and, based on the namespace they each specify, classifies them
// to two groups - the ones that should be synced and the ones that should be removed from the target
// namespaces. Because this is done based on the namespace names, it is robust against the reordering
// of the targets in either the spec or the status of the remote secret.
// Targets in spec that do not have a corresponding status (i.e. the new targets that have not yet been
// deployed to) have the status index set to -1 in the returned classification's Sync map.
func ClassifyTargetNamespaces(rs *api.RemoteSecret) NamespaceClassification {
	specIndices, duplicateSpecs := specNamespaceIndices(rs.Spec.Targets)
	statusIndices, duplicateStatuses := statusNamespaceIndices(rs.Status.Targets)

	ret := NamespaceClassification{
		Sync:                 map[SpecTargetIndex]StatusTargetIndex{},
		Remove:               []StatusTargetIndex{},
		DuplicateTargetSpecs: map[SpecTargetIndex]map[SpecTargetIndex]StatusTargetIndex{},
	}

	for specApiUrl, specIdxByNs := range specIndices {
		for specNs, specIdx := range specIdxByNs {
			stIdx, ok := statusIndices[specApiUrl][specNs]
			if ok {
				ret.Sync[specIdx] = stIdx
				delete(statusIndices[specApiUrl], specNs)
			} else {
				ret.Sync[specIdx] = -1
			}
		}
	}

	for _, stIdxByNs := range statusIndices {
		for _, stIdx := range stIdxByNs {
			ret.Remove = append(ret.Remove, stIdx)
		}
	}

	// You may ask the Golang authors why they don't include this function in the standard library.
	// You may also understand that repetition ad nauseam is the go way...
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	for specApiUrl, specIdxsByNs := range duplicateSpecs {
		for specNs, specIdxs := range specIdxsByNs {
			originalSpecIdx := specIndices[specApiUrl][specNs]

			targetDuplicates, targetDuplicatesPresent := ret.DuplicateTargetSpecs[originalSpecIdx]
			if !targetDuplicatesPresent {
				targetDuplicates = map[SpecTargetIndex]StatusTargetIndex{}
				ret.DuplicateTargetSpecs[originalSpecIdx] = targetDuplicates
			}

			// We can match a duplicate spec with a duplicate status rather blindly because we
			// a duplicate does not have any state (i.e. it doesn't deploy anything into the target cluster).
			// They are merely matched by the target key (apiUrl + namespace).

			stIdxs := duplicateStatuses[specApiUrl][specNs]
			commonLength := min(len(specIdxs), len(stIdxs))
			// match the specs with all the statuses we can
			for i := 0; i < commonLength; i++ {
				targetDuplicates[specIdxs[i]] = stIdxs[i]
			}
			// if there are any unmatched specs, mark them as such
			for i := commonLength; i < len(specIdxs); i++ {
				targetDuplicates[specIdxs[i]] = -1
			}
			// if there are any unmatched status, proclaim them orphans
			for i := commonLength; i < len(stIdxs); i++ {
				ret.OrphanDuplicateStatuses = append(ret.OrphanDuplicateStatuses, stIdxs[i])
			}

			// clean up, because we'll be going through the statuses once more to see if there aren't
			// any more leftovers. These ones have been processed though..
			if stIdxs != nil {
				delete(duplicateStatuses[specApiUrl], specNs)
			}
		}
	}

	for _, stIdxsByNs := range duplicateStatuses {
		for _, stIdxs := range stIdxsByNs {
			ret.OrphanDuplicateStatuses = append(ret.OrphanDuplicateStatuses, stIdxs...)
		}
	}

	return ret
}

func specNamespaceIndices(targets []api.RemoteSecretTarget) (classifiedSpec map[string]map[string]SpecTargetIndex, duplicateTargets map[string]map[string][]SpecTargetIndex) {
	classifiedSpec = make(map[string]map[string]SpecTargetIndex, len(targets))
	duplicateTargets = map[string]map[string][]SpecTargetIndex{}
	for ti, t := range targets {
		indicesByNamespace, clusterPresent := classifiedSpec[t.ApiUrl]
		if !clusterPresent {
			indicesByNamespace = map[string]SpecTargetIndex{}
			classifiedSpec[t.ApiUrl] = indicesByNamespace
		}
		if _, namespacePresent := indicesByNamespace[t.Namespace]; namespacePresent {
			clusterDuplicates, duplicateClusterPresent := duplicateTargets[t.ApiUrl]
			if !duplicateClusterPresent {
				clusterDuplicates = map[string][]SpecTargetIndex{}
				duplicateTargets[t.ApiUrl] = clusterDuplicates
			}
			clusterDuplicates[t.Namespace] = append(clusterDuplicates[t.Namespace], SpecTargetIndex(ti))
		} else {
			indicesByNamespace[t.Namespace] = SpecTargetIndex(ti)
		}
	}
	return
}

func statusNamespaceIndices(targets []api.TargetStatus) (classifiedStatus map[string]map[string]StatusTargetIndex, duplicateStatuses map[string]map[string][]StatusTargetIndex) {
	classifiedStatus = make(map[string]map[string]StatusTargetIndex, len(targets))
	duplicateStatuses = map[string]map[string][]StatusTargetIndex{}
	for i, t := range targets {
		byNamespace, clusterPresent := classifiedStatus[t.ApiUrl]
		if !clusterPresent {
			byNamespace = map[string]StatusTargetIndex{}
			classifiedStatus[t.ApiUrl] = byNamespace
		}
		if _, namespacePresent := byNamespace[t.Namespace]; namespacePresent {
			clusterDuplicates, duplicateClusterPresent := duplicateStatuses[t.ApiUrl]
			if !duplicateClusterPresent {
				clusterDuplicates = map[string][]StatusTargetIndex{}
				duplicateStatuses[t.ApiUrl] = clusterDuplicates
			}
			clusterDuplicates[t.Namespace] = append(clusterDuplicates[t.Namespace], StatusTargetIndex(i))
		} else {
			byNamespace[t.Namespace] = StatusTargetIndex(i)
		}
	}
	return
}
