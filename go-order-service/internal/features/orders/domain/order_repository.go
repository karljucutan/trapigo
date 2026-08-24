package domain

import "context"

type OrderRepository interface {
	Create(ctx context.Context, order Order) (Order, error)
	GetByID(ctx context.Context, id int64) (Order, error)
	List(ctx context.Context) ([]Order, error)
	Update(ctx context.Context, order Order) (Order, error)
	Delete(ctx context.Context, id int64) error
}
