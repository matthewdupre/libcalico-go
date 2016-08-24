// Copyright (c) 2016 Tigera, Inc. All rights reserved.

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

package api

import (
	. "github.com/tigera/libcalico-go/lib/api/unversioned"
)

type TierMetadata struct {
	ObjectMetadata
	Name string `json:"name,omitempty" validate:"omitempty,name"`
}

type TierSpec struct {
	Order *float32 `json:"order,omitempty"`
}

type Tier struct {
	TypeMetadata              // MD4 Removable...  Adds complexity; serializer can do it if it's needed.
	Metadata     TierMetadata `json:"metadata,omitempty"`
	Spec         TierSpec     `json:"spec,omitempty"`
}

func NewTier() *Tier { // MD4 Clearer to pass name into this as well and make a real constructor?
	return &Tier{TypeMetadata: TypeMetadata{Kind: "tier", APIVersion: "v1"}} // MD4 How much do we really get from the version?  Do we even know how it works.
}

type TierList struct { // MD4 I wish we could remove these.  Just make all the Gets return Tier[]s.
	TypeMetadata
	Metadata ListMetadata `json:"metadata,omitempty"` // MD4 Could remove?
	Items    []Tier       `json:"items" validate:"dive"`
}

func NewTierList() *TierList {
	return &TierList{TypeMetadata: TypeMetadata{Kind: "tierList", APIVersion: "v1"}}
}
