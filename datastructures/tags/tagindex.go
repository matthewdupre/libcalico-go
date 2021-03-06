// Copyright (c) 2016 Tigera, Inc. All rights reserved.
//
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

package tags

import "github.com/golang/glog"

// endpointKey expected to be a WorkloadEndpointKey or a HostEndpointKey
// but we just need it to be hashable.
type EndpointKey interface{}

type Index interface {
	UpdateProfileTags(profileID string, tags []string)
	DeleteProfileTags(profileID string)
	UpdateEndpoint(key EndpointKey, profileIDs []string)
	DeleteEndpoint(key EndpointKey)
}

type indexKey struct {
	tag string
	key EndpointKey
}

type tagIndex struct {
	profileIDToTags         map[string][]string
	profileIDToEndpointKey  map[string]map[EndpointKey]bool
	endpointKeyToProfileIDs *EndpointKeyToProfileIDMap
	matches                 map[indexKey]map[string]bool

	onMatchStarted MatchCallback
	onMatchStopped MatchCallback
}

type MatchCallback func(key EndpointKey, tagID string)

func NewIndex(onMatchStarted, onMatchStopped MatchCallback) Index {
	idx := &tagIndex{
		profileIDToTags:         make(map[string][]string),
		profileIDToEndpointKey:  make(map[string]map[EndpointKey]bool),
		endpointKeyToProfileIDs: NewEndpointKeyToProfileIDMap(),
		matches:                 make(map[indexKey]map[string]bool),

		onMatchStarted: onMatchStarted,
		onMatchStopped: onMatchStopped,
	}
	return idx
}

func (idx *tagIndex) UpdateProfileTags(profileID string, tags []string) {
	glog.V(3).Infof("Updating tags for profile %v to %v", profileID, tags)
	oldTags := idx.profileIDToTags[profileID]
	// Calculate the added and removed tags.  Initialise removedTags with
	// a copy of the old tags, then remove any still-present tags.
	removedTags := make(map[string]bool)
	for _, tag := range oldTags {
		removedTags[tag] = true
	}
	addedTags := make(map[string]bool)
	for _, tag := range tags {
		if removedTags[tag] {
			delete(removedTags, tag)
		} else {
			addedTags[tag] = true
		}
	}

	// Find all the endpoints with this profile and update their
	// memberships.
	for epKey, _ := range idx.profileIDToEndpointKey[profileID] {
		for tag, _ := range addedTags {
			idx.addToIndex(epKey, tag, profileID)
		}
		for tag, _ := range removedTags {
			idx.removeFromIndex(epKey, tag, profileID)
		}
	}

	if len(tags) > 0 {
		idx.profileIDToTags[profileID] = tags
	} else {
		delete(idx.profileIDToTags, profileID)
	}
}

func (idx *tagIndex) DeleteProfileTags(profileID string) {
	idx.UpdateProfileTags(profileID, []string{})
}

func (idx *tagIndex) UpdateEndpoint(key EndpointKey, profileIDs []string) {
	// Figure out what's changed and update the cache.
	removedIDs, addedIDs := idx.endpointKeyToProfileIDs.Update(key, profileIDs)

	// Add the new IDs into the main index first so that we don't flap
	// when a profile is renamed.
	for id, _ := range addedIDs {
		// Update reverse index, which we use when resolving profile
		// updates.
		revIdx, ok := idx.profileIDToEndpointKey[id]
		if !ok {
			revIdx = make(map[EndpointKey]bool)
			idx.profileIDToEndpointKey[id] = revIdx
		}
		revIdx[key] = true

		// Update the main match index, triggering callbacks for
		// new matches.
		for _, tag := range idx.profileIDToTags[id] {
			idx.addToIndex(key, tag, id)
		}
	}
	// Now process removed profile IDs.
	for id, _ := range removedIDs {
		// Clean up the reverse index that we use when doing profile
		// updates.
		revIdx := idx.profileIDToEndpointKey[id]
		delete(revIdx, key)
		if len(revIdx) == 0 {
			delete(idx.profileIDToEndpointKey, id)
		}

		// Update the main match index, triggering callbacks for
		// stopped matches.
		for _, tag := range idx.profileIDToTags[id] {
			idx.removeFromIndex(key, tag, id)
		}
	}
}

func (idx *tagIndex) DeleteEndpoint(key EndpointKey) {
	// UpdateEndpoint will clean up if we pass it an empty slice.
	idx.UpdateEndpoint(key, []string{})
}

func (idx *tagIndex) addToIndex(epKey EndpointKey, tag string, profID string) {
	idxKey := indexKey{tag, epKey}
	matchingProfIDs, ok := idx.matches[idxKey]
	if !ok {
		matchingProfIDs = make(map[string]bool)
		idx.matches[idxKey] = matchingProfIDs
		idx.onMatchStarted(epKey, tag)
	}
	matchingProfIDs[profID] = true
}

func (idx *tagIndex) removeFromIndex(epKey EndpointKey, tag string, profID string) {
	idxKey := indexKey{tag, epKey}
	matchingProfIDs := idx.matches[idxKey]
	delete(matchingProfIDs, profID)
	if len(matchingProfIDs) == 0 {
		// There's no-longer a profile keeping this
		// tag alive.
		delete(idx.matches, idxKey)
		idx.onMatchStopped(epKey, tag)
	}
}

type EndpointKeyToProfileIDMap struct {
	endpointKeyToProfileIDs map[EndpointKey][]string
}

func NewEndpointKeyToProfileIDMap() *EndpointKeyToProfileIDMap {
	return &EndpointKeyToProfileIDMap{
		endpointKeyToProfileIDs: make(map[EndpointKey][]string),
	}
}

func (idx EndpointKeyToProfileIDMap) Update(key EndpointKey, profileIDs []string) (
	removedIDs, addedIDs map[string]bool) {
	oldIDs := idx.endpointKeyToProfileIDs[key]
	removedIDs = make(map[string]bool)
	for _, id := range oldIDs {
		removedIDs[id] = true
	}
	addedIDs = make(map[string]bool)
	for _, id := range profileIDs {
		if removedIDs[id] {
			delete(removedIDs, id)
		} else {
			addedIDs[id] = true
		}
	}

	// Store off the update in our cache.
	if len(profileIDs) > 0 {
		idx.endpointKeyToProfileIDs[key] = profileIDs
	} else {
		// No profiles is equivalent to deletion so we may as well
		// clean up completely.
		delete(idx.endpointKeyToProfileIDs, key)
	}

	return removedIDs, addedIDs
}
