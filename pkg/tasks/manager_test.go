package tasks

import (
	"sync"
	"testing"
)

// TestGetOrCreateUserPool_Concurrent pins the property that PR #244
// fixed: many concurrent calls for the same owner must all return
// the *same* *userPool. The previous Load+Store dance allowed two
// goroutines to both miss the Load and both Store, leaving the
// caller-A pool orphaned and any tasks created via it invisible to
// later GetTask / CancelTask lookups.
//
// We assert two things:
//
//  1. all returned pointers for the same owner compare equal;
//  2. the manager's userPools map ends up with exactly one entry
//     per distinct owner.
//
// Run with `go test -race` to also catch any future regression that
// reintroduces an unsynchronized path.
func TestGetOrCreateUserPool_Concurrent(t *testing.T) {
	const (
		owners       = 5
		callsPerUser = 64
	)

	mgr := &taskManager{}

	type result struct {
		owner string
		pool  *userPool
	}
	results := make(chan result, owners*callsPerUser)

	var wg sync.WaitGroup
	for i := 0; i < owners; i++ {
		owner := ownerName(i)
		for j := 0; j < callsPerUser; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- result{owner: owner, pool: mgr.getOrCreateUserPool(owner)}
			}()
		}
	}
	wg.Wait()
	close(results)

	got := map[string]*userPool{}
	for r := range results {
		if existing, ok := got[r.owner]; ok {
			if existing != r.pool {
				t.Fatalf("getOrCreateUserPool(%q) returned two different *userPool values: %p vs %p",
					r.owner, existing, r.pool)
			}
		} else {
			got[r.owner] = r.pool
		}
	}

	if len(got) != owners {
		t.Fatalf("got %d distinct owners in result map, want %d", len(got), owners)
	}

	// Manager's userPools must also have exactly `owners` entries
	// (no duplicates).
	stored := 0
	mgr.userPools.Range(func(_, _ any) bool {
		stored++
		return true
	})
	if stored != owners {
		t.Fatalf("manager.userPools has %d entries, want %d", stored, owners)
	}
}

func ownerName(i int) string {
	return "user-" + string(rune('a'+i))
}
