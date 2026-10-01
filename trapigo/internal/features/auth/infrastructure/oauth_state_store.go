package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
)

// OAuthStateStore persists temporary OAuth state values for browser login flows.
type OAuthStateStore interface {
	Save(ctx context.Context, state *domain.OAuthState) error
	Get(ctx context.Context, stateValue string) (*domain.OAuthState, error)
	Delete(ctx context.Context, stateValue string) error
}

type InMemoryOAuthStateStore struct {
	ttl    time.Duration
	mu     sync.RWMutex
	states map[string]*domain.OAuthState
}

func NewInMemoryOAuthStateStore(ttl time.Duration) *InMemoryOAuthStateStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &InMemoryOAuthStateStore{
		ttl:    ttl,
		states: map[string]*domain.OAuthState{},
	}
}

func (s *InMemoryOAuthStateStore) Save(_ context.Context, state *domain.OAuthState) error {
	if state == nil || !state.Valid() {
		return domain.ErrInvalidState
	}
	if state.CreatedAt.IsZero() {
		state.CreatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state.State] = state
	return nil
}

func (s *InMemoryOAuthStateStore) Get(_ context.Context, stateValue string) (*domain.OAuthState, error) {
	if stateValue == "" {
		return nil, domain.ErrInvalidState
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[stateValue]
	if !ok || state == nil {
		return nil, fmt.Errorf("%w: state not found", domain.ErrInvalidState)
	}
	if state.IsExpired(time.Now().UTC(), s.ttl) {
		delete(s.states, stateValue)
		return nil, fmt.Errorf("%w: state expired", domain.ErrInvalidState)
	}
	return state, nil
}

func (s *InMemoryOAuthStateStore) Delete(_ context.Context, stateValue string) error {
	if stateValue == "" {
		return domain.ErrInvalidState
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, stateValue)
	return nil
}
