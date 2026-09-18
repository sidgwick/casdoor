// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import "github.com/casdoor/casdoor/util"

// getGroupDescendantIDs returns the selected group and all its descendants' IDs.
// ParentId stores a parent group's name, so hierarchy is scoped to each owner.
func getGroupDescendantIDs(groups []*Group, groupID string) []string {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(groupID)
	if err != nil || owner == "" || name == "" {
		return []string{groupID}
	}

	childrenByParent := map[string][]*Group{}
	for _, group := range groups {
		if group == nil || group.Owner != owner || group.IsTopGroup {
			continue
		}
		childrenByParent[group.ParentId] = append(childrenByParent[group.ParentId], group)
	}

	result := []string{groupID}
	visited := map[string]struct{}{groupID: {}}
	queue := []string{name}
	for len(queue) > 0 {
		parentName := queue[0]
		queue = queue[1:]
		for _, child := range childrenByParent[parentName] {
			childID := child.GetId()
			if _, ok := visited[childID]; ok {
				continue
			}
			visited[childID] = struct{}{}
			result = append(result, childID)
			queue = append(queue, child.Name)
		}
	}

	return result
}

// getGroupAndAncestorIDs returns a user's direct group and each existing parent
// group. It is used for group-to-role inheritance; descendants never inherit
// roles granted to their children.
func getGroupAndAncestorIDs(groups []*Group, groupID string) []string {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(groupID)
	if err != nil || owner == "" || name == "" {
		return []string{groupID}
	}

	groupsByName := map[string]*Group{}
	for _, group := range groups {
		if group != nil && group.Owner == owner {
			groupsByName[group.Name] = group
		}
	}

	result := []string{groupID}
	visited := map[string]struct{}{groupID: {}}
	current := groupsByName[name]
	for current != nil && !current.IsTopGroup {
		parent := groupsByName[current.ParentId]
		if parent == nil {
			break
		}
		parentID := parent.GetId()
		if _, ok := visited[parentID]; ok {
			break
		}
		visited[parentID] = struct{}{}
		result = append(result, parentID)
		current = parent
	}

	return result
}
