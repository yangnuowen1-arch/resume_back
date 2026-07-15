package mailbox

import (
	"sync"
	"testing"
	"time"
)

func TestInMemoryOAuthStateStoreConsumesOnlyOnce(t *testing.T) {
	now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	store := newInMemoryOAuthStateStore(func() time.Time { return now }, 10*time.Minute)

	state, err := store.Create(42, ProviderGoogle, "browser-binding")
	if err != nil {
		t.Fatalf("create state: %v", err)
	}
	if state == "" || len(state) < 40 {
		t.Fatalf("state should be an opaque random token, got %q", state)
	}

	value, ok := store.Consume(state, ProviderGoogle, "browser-binding")
	if !ok {
		t.Fatal("first consume should succeed")
	}
	if value.UserID != 42 || value.Provider != ProviderGoogle {
		t.Fatalf("unexpected state value: %#v", value)
	}
	if _, ok := store.Consume(state, ProviderGoogle, "browser-binding"); ok {
		t.Fatal("state must not be reusable")
	}
}

func TestInMemoryOAuthStateStoreRejectsWrongProviderAndExpiredState(t *testing.T) {
	now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	store := newInMemoryOAuthStateStore(func() time.Time { return now }, 10*time.Minute)

	state, err := store.Create(42, ProviderGoogle, "browser-binding")
	if err != nil {
		t.Fatalf("create state: %v", err)
	}
	if _, ok := store.Consume(state, "microsoft", "browser-binding"); ok {
		t.Fatal("state must reject a different provider")
	}
	if _, ok := store.Consume(state, ProviderGoogle, "browser-binding"); !ok {
		t.Fatal("provider mismatch must not consume the valid state")
	}

	expired, err := store.Create(42, ProviderGoogle, "browser-binding")
	if err != nil {
		t.Fatalf("create expired state: %v", err)
	}
	now = now.Add(10 * time.Minute)
	if _, ok := store.Consume(expired, ProviderGoogle, "browser-binding"); ok {
		t.Fatal("expired state must be rejected")
	}
}

func TestInMemoryOAuthStateStoreConcurrentConsume(t *testing.T) {
	store := NewInMemoryOAuthStateStore()
	state, err := store.Create(42, ProviderGoogle, "browser-binding")
	if err != nil {
		t.Fatalf("create state: %v", err)
	}

	const callers = 32
	var wg sync.WaitGroup
	results := make(chan bool, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := store.Consume(state, ProviderGoogle, "browser-binding")
			results <- ok
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for ok := range results {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent consumes succeeded %d times, want 1", successes)
	}
}
