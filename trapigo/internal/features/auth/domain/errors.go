package domain

import "errors"

var (
	ErrKeycloakConnection = errors.New("keycloak connection error")
	ErrInvalidJWT         = errors.New("invalid JWT")
	ErrExpiredToken       = errors.New("token expired")
	ErrInvalidState       = errors.New("invalid OAuth state")
	ErrInvalidCallback    = errors.New("invalid callback")
)
