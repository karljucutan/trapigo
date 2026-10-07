package command

import (
	"context"
	"testing"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
)

type loginStateCapture struct {
	state *domain.OAuthState
}

func (s *loginStateCapture) Save(_ context.Context, state *domain.OAuthState) error {
	s.state = state
	return nil
}

type loginClientStub struct{}

func (loginClientStub) BuildAuthorizationURL(_ context.Context, params infrastructure.AuthorizationURLParams) (string, error) {
	return "https://id.example.com/login?state=" + params.State, nil
}

func TestLoginCommand_PreservesReturnURLInOAuthState(t *testing.T) {
	store := &loginStateCapture{}
	cmd := &LoginCommand{KeycloakClient: loginClientStub{}, StateStore: store}
	const returnURL = "http://localhost:3000/dashboard?tab=logs"
	if _, err := cmd.Execute(context.Background(), returnURL); err != nil {
		t.Fatal(err)
	}
	if store.state == nil || !store.state.Valid() || store.state.ReturnURL != returnURL {
		t.Fatalf("unexpected OAuth state: %+v", store.state)
	}
}
