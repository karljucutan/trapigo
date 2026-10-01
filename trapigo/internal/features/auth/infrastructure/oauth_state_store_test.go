package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
)

func TestInMemoryOAuthStateStore_StoresAndExpiresStates(t *testing.T) {
	store := NewInMemoryOAuthStateStore(50 * time.Millisecond)
	state := &domain.OAuthState{
		State:        "abc123",
		Nonce:        "nonce456",
		CodeVerifier: "verifier789",
		CreatedAt:    time.Now(),
	}

	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	got, err := store.Get(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if got.State != state.State {
		t.Fatalf("unexpected state: %#v", got)
	}

	time.Sleep(75 * time.Millisecond)
	if _, err := store.Get(context.Background(), "abc123"); err == nil {
		t.Fatal("expected expired state to be removed")
	}
}
