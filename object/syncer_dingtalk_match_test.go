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
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDingtalkUserConversionPrefersEmailForNameAndPersistsSourceProperties(t *testing.T) {
	provider := &DingtalkSyncerProvider{Syncer: &Syncer{
		Organization: "rino",
		TableColumns: []*TableColumn{{Name: "unionid", CasdoorName: "Name"}},
	}}
	source := &DingtalkUser{
		UserId:     "userid-1",
		UnionId:    "unionid-1",
		Name:       "张三",
		Department: []int64{1, 12},
		Position:   "Engineer",
		Mobile:     "13800000001",
		Email:      "alice@example.com",
		OrgEmail:   "zhangsan@example.com",
		JobNumber:  "A-1001",
		Active:     true,
	}

	user := provider.dingtalkUserToOriginalUser(source)
	wantID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("casdoor:rino:dingtalk:unionid-1")).String()
	if user.Id != wantID {
		t.Fatalf("Id = %q, want stable UUID %q", user.Id, wantID)
	}
	if user.Name != "alice" {
		t.Fatalf("Name = %q, want the prefix of the primary email", user.Name)
	}
	if user.DisplayName != "张三" || user.Email != "alice@example.com" {
		t.Fatalf("unexpected display/email mapping: %#v / %q", user.DisplayName, user.Email)
	}
	if user.DingTalk != "" {
		t.Fatalf("new synced account must not pretend userid is OAuth OpenId: %q", user.DingTalk)
	}
	if user.Properties["dingtalk_job_number"] != "A-1001" || user.Properties["dingtalk_unionid"] != "unionid-1" {
		t.Fatalf("DingTalk identity/job properties missing: %#v", user.Properties)
	}
	if user.Properties["dingtalk_department_ids"] != `[1,12]` {
		t.Fatalf("department IDs = %q", user.Properties["dingtalk_department_ids"])
	}
	var raw DingtalkUser
	if err := json.Unmarshal([]byte(user.Properties["dingtalk_raw"]), &raw); err != nil {
		t.Fatalf("dingtalk_raw is not valid JSON: %v", err)
	}
	if raw.JobNumber != source.JobNumber || raw.OrgEmail != source.OrgEmail {
		t.Fatalf("dingtalk_raw did not preserve source details: %#v", raw)
	}
}

func TestDingtalkUserConversionFallsBackToPinyinNameWithoutEmail(t *testing.T) {
	provider := &DingtalkSyncerProvider{Syncer: &Syncer{Organization: "rino"}}
	source := &DingtalkUser{UserId: "userid-1", Name: "张三", Active: true}

	user := provider.dingtalkUserToOriginalUser(source)
	if user.Name != "zhangsan" {
		t.Fatalf("Name = %q, want current pinyin fallback", user.Name)
	}
}

func TestPrepareDingtalkUsersPrefersEmailForNewAccountName(t *testing.T) {
	syncer := &Syncer{Organization: "rino", Type: "DingTalk"}
	source := &User{
		Id: "generated-id", DisplayName: "张三", Email: "first.last+ota@example.com",
		Properties: map[string]string{"dingtalk_userid": "userid-1", "dingtalk_unionid": "unionid-1"},
	}

	prepared, err := syncer.prepareDingTalkUsers(nil, []*User{source})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 || prepared[0].Name != "first.last-ota" {
		t.Fatalf("new user's account name did not prefer email: %#v", prepared)
	}
}

func TestDingtalkStableUserIdFallsBackToUserid(t *testing.T) {
	first := dingtalkStableUserId("rino", "userid-1")
	second := dingtalkStableUserId("rino", "userid-1")
	want := uuid.NewSHA1(uuid.NameSpaceURL, []byte("casdoor:rino:dingtalk:userid-1")).String()
	if first != want || second != want {
		t.Fatalf("userid fallback ID is not stable: first=%q second=%q want=%q", first, second, want)
	}
}

func TestDingtalkUserRawPreservesUnknownFields(t *testing.T) {
	provider := &DingtalkSyncerProvider{Syncer: &Syncer{Organization: "rino"}}
	var source DingtalkUser
	err := json.Unmarshal([]byte(`{"userid":"u1","unionid":"union-1","name":"张三","dept_id_list":[1],"job_number":"A1","active":true,"custom_field":"kept"}`), &source)
	if err != nil {
		t.Fatal(err)
	}
	user := provider.dingtalkUserToOriginalUser(&source)
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(user.Properties["dingtalk_raw"]), &raw); err != nil {
		t.Fatalf("invalid dingtalk_raw: %v", err)
	}
	if raw["custom_field"] != "kept" {
		t.Fatalf("unknown DingTalk user field was not preserved: %#v", raw)
	}
}

func TestPrepareDingtalkUsersPrefersUnionIdAndPreservesOAuthLink(t *testing.T) {
	byUnionId := &User{
		Owner:    "rino",
		Name:     "zhangsan",
		Id:       "stable-existing-id",
		DingTalk: "real-open-id",
		Phone:    "13900000001",
		Properties: map[string]string{
			"dingtalk_unionid":           "unionid-1",
			"dingtalk_userid":            "old-userid",
			"oauth_DingTalk_id":          "real-open-id",
			"oauth_DingTalk_unionId":     "unionid-1",
			"oauth_DingTalk_accessToken": "preserve-me",
		},
	}
	byUserId := &User{
		Owner: "rino", Name: "other", Id: "other-id",
		Properties: map[string]string{"dingtalk_userid": "new-userid"},
	}
	syncer := &Syncer{Organization: "rino", Type: "DingTalk"}
	source := &User{
		Id: "generated-id", Name: "unionid-1", DisplayName: "张三", Phone: "13800000001",
		DingTalk: "", Groups: []string{"rino/12"},
		Properties: map[string]string{
			"dingtalk_unionid": "unionid-1",
			"dingtalk_userid":  "new-userid",
			"dingtalk_sub":     "generated-id",
		},
	}

	prepared, err := syncer.prepareDingTalkUsers([]*User{byUnionId, byUserId}, []*User{source})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 {
		t.Fatalf("prepared %d users, want 1", len(prepared))
	}
	got := prepared[0]
	if got.Id != byUnionId.Id || got.Name != byUnionId.Name {
		t.Fatalf("unionId match did not preserve account identity: id=%q name=%q", got.Id, got.Name)
	}
	if got.DingTalk != "real-open-id" {
		t.Fatalf("OAuth OpenId was not preserved: %q", got.DingTalk)
	}
	if got.Properties["oauth_DingTalk_accessToken"] != "preserve-me" {
		t.Fatalf("OAuth properties were lost: %#v", got.Properties)
	}
	if got.Properties["dingtalk_userid"] != "new-userid" || got.Properties["dingtalk_sub"] != byUnionId.Id {
		t.Fatalf("DingTalk identity properties were not refreshed: %#v", got.Properties)
	}
	if len(got.Groups) != 1 || got.Groups[0] != "rino/12" {
		t.Fatalf("department membership was not retained: %#v", got.Groups)
	}
}

func TestPrepareDingtalkUsersUsesUniquePhoneFallbackAndSkipsDuplicateSourcePhones(t *testing.T) {
	syncer := &Syncer{Organization: "rino", Type: "DingTalk"}
	existing := &User{Owner: "rino", Name: "existing", Id: "existing-id", Phone: "+86 130-0199-1325"}
	phoneMatch := &User{
		Id: "generated", Name: "new-name", DisplayName: "李四", Phone: "13001991325",
		Properties: map[string]string{"dingtalk_userid": "new-userid"},
	}
	prepared, err := syncer.prepareDingTalkUsers([]*User{existing}, []*User{phoneMatch})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 || prepared[0].Id != existing.Id || prepared[0].Name != existing.Name {
		t.Fatalf("normalized phone fallback failed: %#v", prepared)
	}

	duplicateA := &User{Id: "a", DisplayName: "王五", Phone: "13000000000", Properties: map[string]string{"dingtalk_userid": "a"}}
	duplicateB := &User{Id: "b", DisplayName: "赵六", Phone: "+86 13000000000", Properties: map[string]string{"dingtalk_userid": "b"}}
	prepared, err = syncer.prepareDingTalkUsers(nil, []*User{duplicateA, duplicateB})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 0 {
		t.Fatalf("duplicate-phone users without stable match should be skipped, got %d", len(prepared))
	}
}

func TestPrepareDingtalkUsersAllocatesLegacyPinyinNamesDeterministically(t *testing.T) {
	syncer := &Syncer{Organization: "rino", Type: "DingTalk"}
	existing := &User{Owner: "rino", Name: "zhangsan", Id: "existing-id", Phone: "13000000000"}
	first := &User{
		Id: "first-generated", DisplayName: "张三", Phone: "13800000001",
		Properties: map[string]string{"dingtalk_userid": "first", "dingtalk_unionid": "union-first"},
	}
	second := &User{
		Id: "second-generated", DisplayName: "张三", Phone: "13800000002",
		Properties: map[string]string{"dingtalk_userid": "second", "dingtalk_unionid": "union-second"},
	}
	prepared, err := syncer.prepareDingTalkUsers([]*User{existing}, []*User{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 2 {
		t.Fatalf("prepared %d users, want 2", len(prepared))
	}
	if prepared[0].Name != "zhangsan-13800000001" || prepared[1].Name != "zhangsan-13800000002" {
		t.Fatalf("pinyin names do not follow legacy collision rules: %q, %q", prepared[0].Name, prepared[1].Name)
	}
}

func TestDingtalkHashChangesWhenGroupsOrPropertiesChange(t *testing.T) {
	syncer := &Syncer{Type: "DingTalk"}
	base := &User{Groups: []string{"rino/1"}, Properties: map[string]string{"dingtalk_job_number": "A1"}}
	changedGroup := &User{Groups: []string{"rino/2"}, Properties: map[string]string{"dingtalk_job_number": "A1"}}
	changedProperty := &User{Groups: []string{"rino/1"}, Properties: map[string]string{"dingtalk_job_number": "A2"}}
	baseHash := syncer.calculateHash(base)
	if baseHash == syncer.calculateHash(changedGroup) {
		t.Fatal("department membership change did not affect sync hash")
	}
	if baseHash == syncer.calculateHash(changedProperty) {
		t.Fatal("DingTalk property change did not affect sync hash")
	}
	if strings.TrimSpace(dingtalkAccountName("张三")) != "zhangsan" {
		t.Fatal("Chinese account-name transliteration differs from the legacy pinyin rule")
	}
}

func TestFindUserByDingtalkUnionId(t *testing.T) {
	users := []*User{
		{Owner: "rino", Name: "zhangsan", Id: "id-1", Properties: map[string]string{"dingtalk_unionid": "unionid-1"}},
		{Owner: "rino", Name: "lisi", Id: "id-2", Properties: map[string]string{"oauth_DingTalk_unionId": "unionid-2"}},
		{Owner: "rino", Name: "old-unionid", Id: "id-3", Properties: map[string]string{"dingtalk_unionid": "current-unionid"}},
	}
	user, err := findUserByDingTalkUnionId(users, "unionid-2")
	if err != nil || user == nil || user.Name != "lisi" {
		t.Fatalf("unionId lookup = %#v, %v", user, err)
	}
	user, err = findUserByDingTalkUnionId(users, "old-unionid")
	if err != nil || user != nil {
		t.Fatalf("stale account name must not override its stored unionId: %#v, %v", user, err)
	}

	users = append(users, &User{Owner: "rino", Name: "other", Id: "id-3", Properties: map[string]string{"dingtalk_unionid": "unionid-2"}})
	if _, err = findUserByDingTalkUnionId(users, "unionid-2"); err == nil {
		t.Fatal("duplicate unionId should report an ambiguity")
	}
}
