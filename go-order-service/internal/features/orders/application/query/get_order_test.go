package query

import (
	"context"
	"testing"

	"go-order-service/internal/features/orders/domain"
)

type queryStubRepo struct {
	orders map[int64]domain.Order
}

func (s *queryStubRepo) Create(ctx context.Context, order domain.Order) (domain.Order, error) {
	return order, nil
}

func (s *queryStubRepo) GetByID(ctx context.Context, id int64) (domain.Order, error) {
	order, ok := s.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return order, nil
}

func (s *queryStubRepo) List(ctx context.Context) ([]domain.Order, error) {
	items := make([]domain.Order, 0, len(s.orders))
	for _, order := range s.orders {
		items = append(items, order)
	}
	return items, nil
}

func (s *queryStubRepo) Update(ctx context.Context, order domain.Order) (domain.Order, error) {
	return order, nil
}

func (s *queryStubRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func TestGetOrderByIDHandler(t *testing.T) {
	repo := &queryStubRepo{orders: map[int64]domain.Order{1: {ID: 1, CustomerID: 10, Status: "pending", TotalAmountCents: 1000}}}
	handler := NewGetOrderByIDHandler(repo)

	order, err := handler.Handle(context.Background(), GetOrderByIDQuery{ID: 1})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if order.ID != 1 {
		t.Fatalf("order id mismatch: got %d want %d", order.ID, 1)
	}
}
