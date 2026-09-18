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
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/mozillazg/go-pinyin"
)

var dingtalkInvalidNameChars = regexp.MustCompile(`[^A-Za-z0-9]+`)
var dingtalkNonPhoneChars = regexp.MustCompile(`[^0-9]`)
var dingtalkValidAccountName = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-._][A-Za-z0-9]+)*$`)
var dingtalkInvalidAccountNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
var dingtalkRepeatedAccountNameSeparators = regexp.MustCompile(`[-._]{2,}`)

type dingtalkUserIndexes struct {
	byUnionId map[string][]*User
	byUserId  map[string][]*User
	byPhone   map[string][]*User
}

func dingtalkStableUserId(owner string, identity string) string {
	if identity == "" {
		return uuid.NewString()
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("casdoor:%s:dingtalk:%s", owner, identity))).String()
}

func dingtalkAccountName(displayName string) string {
	args := pinyin.NewArgs()
	name := strings.Join(pinyin.LazyPinyin(strings.TrimSpace(displayName), args), "")
	name = strings.ToLower(strings.Trim(dingtalkInvalidNameChars.ReplaceAllString(name, "-"), "-_"))
	if name == "" {
		return "user"
	}
	return name
}

func dingtalkPreferredAccountName(email string, displayName string) string {
	if email = strings.TrimSpace(email); email != "" {
		if prefix, _, ok := strings.Cut(email, "@"); ok {
			if prefix != "" {
				prefix = strings.ToLower(strings.TrimSpace(prefix))
				prefix = dingtalkInvalidAccountNameChars.ReplaceAllString(prefix, "-")
				prefix = dingtalkRepeatedAccountNameSeparators.ReplaceAllString(prefix, "-")
				prefix = strings.Trim(prefix, "-._")
				if dingtalkValidAccountName.MatchString(prefix) {
					return prefix
				}
				return dingtalkAccountName(prefix)
			}
		}
	}
	return dingtalkAccountName(displayName)
}

func normalizeDingTalkPhone(value string) string {
	digits := dingtalkNonPhoneChars.ReplaceAllString(value, "")
	if strings.HasPrefix(digits, "0086") && len(digits) == 15 {
		return digits[4:]
	}
	if strings.HasPrefix(digits, "86") && len(digits) == 13 {
		return digits[2:]
	}
	return digits
}

func dingtalkProperty(user *User, keys ...string) string {
	if user == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(user.Properties[key]); value != "" {
			return value
		}
	}
	return ""
}

func dingtalkPropertyValues(user *User, keys ...string) []string {
	values := []string{}
	seen := map[string]bool{}
	if user == nil {
		return values
	}
	for _, key := range keys {
		value := strings.TrimSpace(user.Properties[key])
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func newDingTalkUserIndexes(users []*User) *dingtalkUserIndexes {
	indexes := &dingtalkUserIndexes{
		byUnionId: map[string][]*User{},
		byUserId:  map[string][]*User{},
		byPhone:   map[string][]*User{},
	}
	for _, user := range users {
		unionIds := dingtalkPropertyValues(user, "dingtalk_unionid", "oauth_DingTalk_unionId")
		// The previous built-in mapping used unionid as Casdoor Name. Keep that
		// legacy representation matchable while the properties are backfilled.
		if len(unionIds) == 0 && user.Name != "" {
			unionIds = append(unionIds, user.Name)
		}
		for _, unionId := range unionIds {
			indexes.byUnionId[unionId] = append(indexes.byUnionId[unionId], user)
		}

		identityValues := dingtalkPropertyValues(user,
			"dingtalk_userid",
			"oauth_DingTalk_userId",
			"oauth_DingTalk_userid",
			"oauth_DingTalk_id",
		)
		// Older built-in syncer versions stored userid in Id and DingTalk. Do not
		// treat a known OAuth OpenId as userid; it is a different identifier.
		if user.Id != "" && dingtalkProperty(user, "oauth_DingTalk_id") == "" {
			identityValues = append(identityValues, user.Id)
		}
		if user.DingTalk != "" && dingtalkProperty(user, "oauth_DingTalk_id") == "" {
			identityValues = append(identityValues, user.DingTalk)
		}
		seenIdentity := map[string]bool{}
		for _, value := range identityValues {
			if value != "" && !seenIdentity[value] {
				indexes.byUserId[value] = append(indexes.byUserId[value], user)
				seenIdentity[value] = true
			}
		}

		phone := normalizeDingTalkPhone(user.Phone)
		if phone != "" {
			indexes.byPhone[phone] = append(indexes.byPhone[phone], user)
		}
	}
	return indexes
}

func uniqueDingTalkMatch(index map[string][]*User, value string, label string) (*User, error) {
	if value == "" {
		return nil, nil
	}
	candidates := index[value]
	if len(candidates) == 0 {
		return nil, nil
	}
	unique := map[string]*User{}
	for _, user := range candidates {
		unique[user.GetId()] = user
	}
	if len(unique) > 1 {
		names := make([]string, 0, len(unique))
		for _, user := range unique {
			names = append(names, user.GetId())
		}
		sort.Strings(names)
		return nil, fmt.Errorf("ambiguous DingTalk %s %q matches Casdoor users: %s", label, value, strings.Join(names, ", "))
	}
	for _, user := range unique {
		return user, nil
	}
	return nil, nil
}

func mergeDingTalkProperties(existing map[string]string, incoming map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(incoming))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range incoming {
		merged[key] = value
	}
	return merged
}

// prepareDingTalkUsers preserves the former standalone syncer's identity and
// account-name behavior before the generic Casdoor sync engine keys users by Id.
func (syncer *Syncer) prepareDingTalkUsers(existingUsers []*User, sourceUsers []*OriginalUser) ([]*OriginalUser, error) {
	indexes := newDingTalkUserIndexes(existingUsers)
	occupiedNames := map[string]bool{}
	for _, user := range existingUsers {
		occupiedNames[strings.ToLower(user.Name)] = true
	}

	phones := map[string]int{}
	for _, user := range sourceUsers {
		phone := normalizeDingTalkPhone(user.Phone)
		if phone != "" {
			phones[phone]++
		}
	}
	duplicatePhones := map[string]bool{}
	for phone, count := range phones {
		if count > 1 {
			duplicatePhones[phone] = true
		}
	}

	sort.SliceStable(sourceUsers, func(i, j int) bool {
		left := dingtalkProperty(sourceUsers[i], "dingtalk_userid")
		right := dingtalkProperty(sourceUsers[j], "dingtalk_userid")
		return left < right
	})

	prepared := make([]*OriginalUser, 0, len(sourceUsers))
	claimedUsers := map[string]string{}
	for _, source := range sourceUsers {
		unionId := dingtalkProperty(source, "dingtalk_unionid")
		userId := dingtalkProperty(source, "dingtalk_userid")
		phone := normalizeDingTalkPhone(source.Phone)

		matched, err := uniqueDingTalkMatch(indexes.byUnionId, unionId, "unionId")
		if err != nil {
			return nil, err
		}
		if matched == nil {
			matched, err = uniqueDingTalkMatch(indexes.byUserId, userId, "userid")
			if err != nil {
				return nil, err
			}
		}
		if matched == nil && !duplicatePhones[phone] {
			matched, err = uniqueDingTalkMatch(indexes.byPhone, phone, "phone")
			if err != nil {
				return nil, err
			}
		}

		if matched == nil && duplicatePhones[phone] {
			fmt.Printf("Skipping DingTalk user %q: duplicate mobile requires a stable unionId/userid match\n", userId)
			continue
		}
		if matched != nil {
			if previous, claimed := claimedUsers[matched.GetId()]; claimed && previous != userId {
				return nil, fmt.Errorf("DingTalk users %q and %q both match Casdoor user %s", previous, userId, matched.GetId())
			}
			claimedUsers[matched.GetId()] = userId

			if matched.Id != "" {
				source.Id = matched.Id
			} else {
				source.Id = dingtalkStableUserId(syncer.Organization, dingtalkFirstNonEmpty(unionId, userId))
			}
			source.Name = matched.Name
			source.DingTalk = matched.DingTalk
			source.Type = matched.Type
			source.Properties = mergeDingTalkProperties(matched.Properties, source.Properties)
			source.Properties["dingtalk_sub"] = source.Id
		} else {
			base := dingtalkPreferredAccountName(source.Email, source.DisplayName)
			source.Name = uniqueDingTalkAccountName(base, source.Phone, userId, occupiedNames)
		}

		occupiedNames[strings.ToLower(source.Name)] = true
		prepared = append(prepared, source)
	}
	return prepared, nil
}

func uniqueDingTalkAccountName(base string, phone string, userId string, occupied map[string]bool) string {
	if !occupied[strings.ToLower(base)] {
		return base
	}
	suffix := normalizeDingTalkPhone(phone)
	if suffix == "" {
		suffix = dingtalkInvalidNameChars.ReplaceAllString(userId, "-")
		suffix = strings.Trim(suffix, "-_")
	}
	if suffix == "" {
		suffix = "user"
	}
	candidate := base + "-" + suffix
	if !occupied[strings.ToLower(candidate)] {
		return candidate
	}
	safeUserId := strings.Trim(dingtalkInvalidNameChars.ReplaceAllString(userId, "-"), "-_")
	if safeUserId == "" {
		safeUserId = "user"
	}
	candidate += "-" + safeUserId
	for i := 2; occupied[strings.ToLower(candidate)]; i++ {
		candidate = base + "-" + suffix + "-" + fmt.Sprint(i)
	}
	return candidate
}

func dingtalkFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
