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

import (
	"encoding/json"
	"testing"
)

func TestDingtalkDepartmentToOriginalGroupPreservesParent(t *testing.T) {
	provider := &DingtalkSyncerProvider{Syncer: &Syncer{Organization: "rino"}}

	root := provider.dingtalkDepartmentToOriginalGroup(&DingtalkDepartment{
		DeptId:   1,
		Name:     "Company",
		ParentId: 0,
	})
	if root.ParentId != "" {
		t.Fatalf("root ParentId = %q, want empty", root.ParentId)
	}

	child := provider.dingtalkDepartmentToOriginalGroup(&DingtalkDepartment{
		DeptId:   12,
		Name:     "Engineering",
		ParentId: 3,
	})
	if child.ParentId != "3" {
		t.Fatalf("child ParentId = %q, want %q", child.ParentId, "3")
	}
}

func TestDingtalkDepartmentPreservesLegacyProperties(t *testing.T) {
	provider := &DingtalkSyncerProvider{Syncer: &Syncer{Organization: "rino"}}
	var department DingtalkDepartment
	err := json.Unmarshal([]byte(`{"dept_id":12,"name":"Engineering","parent_id":3,"custom_field":"kept"}`), &department)
	if err != nil {
		t.Fatal(err)
	}
	original := provider.dingtalkDepartmentToOriginalGroup(&department)
	if original.Properties["dingtalk_dept_id"] != "12" || original.Properties["dingtalk_parent_dept_id"] != "3" {
		t.Fatalf("department identity properties missing: %#v", original.Properties)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(original.Properties["dingtalk_raw"]), &raw); err != nil {
		t.Fatalf("invalid dingtalk_raw: %v", err)
	}
	if raw["custom_field"] != "kept" {
		t.Fatalf("unknown DingTalk fields were not preserved in dingtalk_raw: %#v", raw)
	}

	group := (&Syncer{Organization: "rino"}).createGroupFromOriginalGroup(original)
	if group.Properties["dingtalk_dept_id"] != "12" {
		t.Fatalf("syncer dropped department properties: %#v", group.Properties)
	}
}
