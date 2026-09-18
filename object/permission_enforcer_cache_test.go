package object

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/casbin/casbin/v2"
)

func TestPermissionEnforcerCacheHitAndInvalidation(t *testing.T) {
	cache := newPermissionEnforcerCache(time.Minute, 8)
	var builds int32
	build := func() (*casbin.SyncedEnforcer, error) {
		atomic.AddInt32(&builds, 1)
		return &casbin.SyncedEnforcer{Enforcer: &casbin.Enforcer{}}, nil
	}

	first, err := cache.get("permission", build)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.get("permission", build)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected cache hit to return the same enforcer")
	}
	if got := atomic.LoadInt32(&builds); got != 1 {
		t.Fatalf("expected one build before invalidation, got %d", got)
	}

	cache.invalidate()
	third, err := cache.get("permission", build)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("expected invalidation to replace the cached enforcer")
	}
	if got := atomic.LoadInt32(&builds); got != 2 {
		t.Fatalf("expected rebuild after invalidation, got %d builds", got)
	}
}

func TestPermissionEnforcerCacheCoalescesConcurrentBuilds(t *testing.T) {
	cache := newPermissionEnforcerCache(time.Minute, 8)
	started := make(chan struct{})
	release := make(chan struct{})
	var builds int32
	build := func() (*casbin.SyncedEnforcer, error) {
		if atomic.AddInt32(&builds, 1) == 1 {
			close(started)
		}
		<-release
		return &casbin.SyncedEnforcer{Enforcer: &casbin.Enforcer{}}, nil
	}

	const callers = 8
	results := make(chan *casbin.SyncedEnforcer, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enforcer, err := cache.get("permission", build)
			if err != nil {
				t.Errorf("cache get failed: %v", err)
				return
			}
			results <- enforcer
		}()
	}
	<-started
	time.Sleep(5 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)

	var first *casbin.SyncedEnforcer
	for enforcer := range results {
		if first == nil {
			first = enforcer
		} else if enforcer != first {
			t.Fatal("concurrent callers did not share the cached enforcer")
		}
	}
	if got := atomic.LoadInt32(&builds); got != 1 {
		t.Fatalf("expected one shared build, got %d builds", got)
	}
}

func TestPermissionEnforcerCacheExpires(t *testing.T) {
	cache := newPermissionEnforcerCache(time.Millisecond, 8)
	var builds int32
	build := func() (*casbin.SyncedEnforcer, error) {
		atomic.AddInt32(&builds, 1)
		return &casbin.SyncedEnforcer{Enforcer: &casbin.Enforcer{}}, nil
	}

	if _, err := cache.get("permission", build); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := cache.get("permission", build); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&builds); got != 2 {
		t.Fatalf("expected expired entry to rebuild, got %d builds", got)
	}
}

func TestPermissionEnforcerCacheDoesNotPublishAcrossInvalidation(t *testing.T) {
	cache := newPermissionEnforcerCache(time.Minute, 8)
	started := make(chan struct{})
	release := make(chan struct{})
	var builds int32
	build := func() (*casbin.SyncedEnforcer, error) {
		call := atomic.AddInt32(&builds, 1)
		if call == 1 {
			close(started)
			<-release
		}
		return &casbin.SyncedEnforcer{Enforcer: &casbin.Enforcer{}}, nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := cache.get("permission", build); err != nil {
			t.Errorf("cache get failed: %v", err)
		}
	}()

	<-started
	cache.invalidate()
	close(release)
	wg.Wait()
	if got := atomic.LoadInt32(&builds); got != 2 {
		t.Fatalf("expected stale in-progress build to be discarded and retried, got %d builds", got)
	}
}
