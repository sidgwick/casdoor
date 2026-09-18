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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
)

// The TTL bounds stale reads on other Casdoor instances or writes that bypass
// the object mutation APIs; local authorization writes invalidate immediately.
const (
	permissionEnforcerCacheTTL      = 10 * time.Second
	permissionEnforcerCacheMaxItems = 256
)

type cachedPermissionEnforcer struct {
	enforcer  *casbin.SyncedEnforcer
	expiresAt time.Time
}

type permissionEnforcerBuild struct {
	done chan struct{}
	err  error
}

type permissionEnforcerCache struct {
	mu         sync.RWMutex
	generation uint64
	entries    map[string]cachedPermissionEnforcer
	inflight   map[string]*permissionEnforcerBuild
	ttl        time.Duration
	maxItems   int
}

func newPermissionEnforcerCache(ttl time.Duration, maxItems int) *permissionEnforcerCache {
	return &permissionEnforcerCache{
		entries:  map[string]cachedPermissionEnforcer{},
		inflight: map[string]*permissionEnforcerBuild{},
		ttl:      ttl,
		maxItems: maxItems,
	}
}

func (c *permissionEnforcerCache) get(key string, build func() (*casbin.SyncedEnforcer, error)) (*casbin.SyncedEnforcer, error) {
	for {
		now := time.Now()
		c.mu.RLock()
		entry, ok := c.entries[key]
		c.mu.RUnlock()
		if ok && now.Before(entry.expiresAt) {
			return entry.enforcer, nil
		}

		c.mu.Lock()
		if entry, ok = c.entries[key]; ok && time.Now().Before(entry.expiresAt) {
			c.mu.Unlock()
			return entry.enforcer, nil
		}
		if inProgress, ok := c.inflight[key]; ok {
			done := inProgress.done
			c.mu.Unlock()
			<-done
			if inProgress.err != nil {
				return nil, inProgress.err
			}
			continue
		}

		generation := c.generation
		inProgress := &permissionEnforcerBuild{done: make(chan struct{})}
		c.inflight[key] = inProgress
		c.mu.Unlock()

		enforcer, err := build()

		c.mu.Lock()
		if err == nil && generation == c.generation {
			c.removeExpiredLocked(time.Now())
			if c.maxItems > 0 && len(c.entries) >= c.maxItems {
				// Entries use a fixed TTL, so the earliest expiry is the least recently
				// inserted entry. Eviction only scans on cache misses, not on hot hits.
				var oldestKey string
				var oldestExpiry time.Time
				for candidateKey, candidate := range c.entries {
					if oldestKey == "" || candidate.expiresAt.Before(oldestExpiry) {
						oldestKey = candidateKey
						oldestExpiry = candidate.expiresAt
					}
				}
				delete(c.entries, oldestKey)
			}
			c.entries[key] = cachedPermissionEnforcer{enforcer: enforcer, expiresAt: time.Now().Add(c.ttl)}
		}
		inProgress.err = err
		delete(c.inflight, key)
		close(inProgress.done)
		c.mu.Unlock()

		if err != nil {
			return nil, err
		}
		if generation != c.currentGeneration() {
			continue
		}
		return enforcer, nil
	}
}

func (c *permissionEnforcerCache) currentGeneration() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}

func (c *permissionEnforcerCache) invalidate() {
	c.mu.Lock()
	c.generation++
	c.entries = map[string]cachedPermissionEnforcer{}
	c.mu.Unlock()
}

func (c *permissionEnforcerCache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

var permissionEnforcers = newPermissionEnforcerCache(permissionEnforcerCacheTTL, permissionEnforcerCacheMaxItems)

// InvalidatePermissionEnforcerCache drops cached policy snapshots after an
// authorization-relevant object or relationship has changed.
func InvalidatePermissionEnforcerCache() {
	permissionEnforcers.invalidate()
}

func getCachedPermissionEnforcer(p *Permission, permissionIDs ...string) (*casbin.SyncedEnforcer, error) {
	hasPermissionIDs := len(permissionIDs) != 0
	ids := append([]string(nil), permissionIDs...)
	if len(ids) == 0 {
		ids = []string{p.GetId()}
	}
	sort.Strings(ids)
	ids = compactStrings(ids)
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%q\x00%q\x00%q\x00%q\x00%q\x00%q\x00%q", p.GetId(), p.Model, p.Adapter, p.Users, p.Groups, p.Roles, p.Domains, p.Resources, p.Actions, p.Effect)
	key += "\x00" + strings.Join(ids, "\x00")

	return permissionEnforcers.get(key, func() (*casbin.SyncedEnforcer, error) {
		var enforcer *casbin.Enforcer
		var err error
		if hasPermissionIDs {
			enforcer, err = getPermissionEnforcer(p, ids...)
		} else {
			enforcer, err = getPermissionEnforcer(p)
		}
		if err != nil {
			return nil, err
		}
		// The enforcer is fully initialized before publication and thereafter
		// only used for read-only enforcement. SyncedEnforcer permits concurrent
		// Enforce/BatchEnforce calls safely.
		return &casbin.SyncedEnforcer{Enforcer: enforcer}, nil
	})
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
