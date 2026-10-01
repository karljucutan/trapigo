package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"golang.org/x/oauth2"
)

type CallbackKeycloakClient interface {
	ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error)
}

type CallbackTokenValidator interface {
	Validate(ctx context.Context, token string) (*domain.Claims, error)
}

type CallbackStateStore interface {
	Get(ctx context.Context, stateValue string) (*domain.OAuthState, error)
	Delete(ctx context.Context, stateValue string) error
}

type CallbackResult struct {
	Token  *oauth2.Token
	Claims *domain.Claims
}

type CallbackCommand struct {
	KeycloakClient CallbackKeycloakClient
	StateStore     CallbackStateStore
	Validator      CallbackTokenValidator
}

func (c *CallbackCommand) Execute(ctx context.Context, code, state string) (*CallbackResult, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" {
		return nil, domain.ErrInvalidCallback
	}
	if c.KeycloakClient == nil || c.StateStore == nil || c.Validator == nil {
		return nil, fmt.Errorf("callback command misconfigured")
	}

	oauthState, err := c.StateStore.Get(ctx, state)
	if err != nil {
		return nil, err
	}
	if oauthState == nil || oauthState.State != state {
		return nil, domain.ErrInvalidState
	}

	token, err := c.KeycloakClient.ExchangeAuthorizationCode(ctx, code, oauthState.CodeVerifier)
	if err != nil {
		return nil, err
	}
	if err := validateIDTokenNonce(token, oauthState.Nonce); err != nil {
		return nil, err
	}
	claims, err := c.Validator.Validate(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	if err := c.StateStore.Delete(ctx, state); err != nil {
		return nil, err
	}

	return &CallbackResult{Token: token, Claims: claims}, nil
}

func validateIDTokenNonce(token *oauth2.Token, expectedNonce string) error {
	if token == nil {
		return domain.ErrInvalidCallback
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(idToken) == "" {
		return nil
	}

	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(idToken, claims)
	if err != nil {
		return domain.ErrInvalidCallback
	}
	nonce, _ := claims["nonce"].(string)
	if strings.TrimSpace(nonce) == "" || nonce != expectedNonce {
		return domain.ErrInvalidCallback
	}
	return nil
}
