package domain

import "errors"

var (
	ErrInvalidCustomerID = errors.New("customer id is required")
	ErrOrderEmpty        = errors.New("order must contain at least one item")
	ErrInvalidQuantity   = errors.New("quantity must be greater than zero")
	ErrInvalidPrice      = errors.New("unit price must be greater than zero")
	ErrInvalidProductID  = errors.New("product id is required")
	ErrOrderNotFound     = errors.New("order not found")
	ErrOrderItemNotFound = errors.New("order item not found")
)
