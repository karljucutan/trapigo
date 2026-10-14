package command

import (
	"context"
	"strings"
	"testing"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	"golang.org/x/oauth2"
)

type loginStateCapture struct {
	state *domain.OAuthState
}

func (s *loginStateCapture) Save(_ context.Context, state *domain.OAuthState) error {
	s.state = state
	return nil
}

type loginClientStub struct {
	scopes []string
}

func (client *loginClientStub) BuildAuthorizationURL(_ context.Context, params infrastructure.AuthorizationURLParams) (string, error) {
	client.scopes = params.Scopes
	return "https://id.example.com/login?state=" + params.State, nil
}

func (*loginClientStub) ExchangeAuthorizationCode(context.Context, string, string) (*oauth2.Token, error) {
	return nil, nil
}

func (*loginClientStub) EndSession(context.Context, string) error {
	return nil
}

func TestLoginCommand_PreservesReturnURLInOAuthState(t *testing.T) {
	store := &loginStateCapture{}
	client := &loginClientStub{}
	cmd := &LoginCommand{
		IdentityProvider: client,
		StateStore:       store,
		RequiredScopes:   []string{"trapigo-go-order-service"},
	}
	const returnURL = "http://localhost:3000/dashboard?tab=logs"
	if _, err := cmd.Execute(context.Background(), returnURL); err != nil {
		t.Fatal(err)
	}
	if store.state == nil || !store.state.Valid() || store.state.ReturnURL != returnURL {
		t.Fatalf("unexpected OAuth state: %+v", store.state)
	}
	if got := strings.Join(client.scopes, " "); got != "openid profile email trapigo-go-order-service" {
		t.Fatalf("unexpected OAuth scopes: %s", got)
	}
}
