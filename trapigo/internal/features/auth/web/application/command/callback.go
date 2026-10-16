package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/application"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"golang.org/x/oauth2"
)

type CallbackTokenValidator interface {
	Validate(ctx context.Context, token string) (*domain.Claims, error)
}

type CallbackIDTokenValidator interface {
	ValidateIDToken(ctx context.Context, token, expectedNonce string) error
}

type CallbackStateStore interface {
	Get(ctx context.Context, stateValue string) (*domain.OAuthState, error)
	Delete(ctx context.Context, stateValue string) error
}

type CallbackResult struct {
	Token     *oauth2.Token
	IDToken   string
	Claims    *domain.Claims
	ReturnURL string
}

type CallbackCommand struct {
	IdentityProvider application.IdentityProvider
	StateStore       CallbackStateStore
	Validator        CallbackTokenValidator
	IDTokenValidator CallbackIDTokenValidator
}

func (c *CallbackCommand) Execute(ctx context.Context, code, state string) (*CallbackResult, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" {
		return nil, domain.ErrInvalidCallback
	}
	if c.IdentityProvider == nil || c.StateStore == nil || c.Validator == nil || c.IDTokenValidator == nil {
		return nil, fmt.Errorf("callback command misconfigured")
	}

	oauthState, err := c.StateStore.Get(ctx, state)
	if err != nil {
		return nil, err
	}
	if oauthState == nil || oauthState.State != state {
		return nil, domain.ErrInvalidState
	}

	token, err := c.IdentityProvider.ExchangeAuthorizationCode(ctx, code, oauthState.CodeVerifier)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, domain.ErrInvalidCallback
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(idToken) == "" {
		return nil, domain.ErrInvalidCallback
	}
	if err := c.IDTokenValidator.ValidateIDToken(ctx, idToken, oauthState.Nonce); err != nil {
		return nil, err
	}
	claims, err := c.Validator.Validate(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	if err := c.StateStore.Delete(ctx, state); err != nil {
		return nil, err
	}

	return &CallbackResult{Token: token, IDToken: idToken, Claims: claims, ReturnURL: oauthState.ReturnURL}, nil
}
