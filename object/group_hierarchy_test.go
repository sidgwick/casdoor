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
	"reflect"
	"testing"
)

func TestGetGroupDescendantIDs(t *testing.T) {
	groups := []*Group{
		{Owner: "org", Name: "root", IsTopGroup: true},
		{Owner: "org", Name: "engineering", ParentId: "root"},
		{Owner: "org", Name: "platform", ParentId: "engineering"},
		{Owner: "org", Name: "qa", ParentId: "root"},
		{Owner: "other", Name: "private", ParentId: "engineering"},
	}

	got := getGroupDescendantIDs(groups, "org/engineering")
	want := []string{"org/engineering", "org/platform"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getGroupDescendantIDs() = %#v, want %#v", got, want)
	}
}

func TestGetGroupDescendantIDsStopsOnCycles(t *testing.T) {
	groups := []*Group{
		{Owner: "org", Name: "a", ParentId: "b"},
		{Owner: "org", Name: "b", ParentId: "a"},
	}

	got := getGroupDescendantIDs(groups, "org/a")
	want := []string{"org/a", "org/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getGroupDescendantIDs() = %#v, want %#v", got, want)
	}
}

func TestGetGroupAndAncestorIDs(t *testing.T) {
	groups := []*Group{
		{Owner: "org", Name: "root", IsTopGroup: true},
		{Owner: "org", Name: "engineering", ParentId: "root"},
		{Owner: "org", Name: "platform", ParentId: "engineering"},
		{Owner: "org", Name: "qa", ParentId: "root"},
	}

	got := getGroupAndAncestorIDs(groups, "org/platform")
	want := []string{"org/platform", "org/engineering", "org/root"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getGroupAndAncestorIDs() = %#v, want %#v", got, want)
	}
}
