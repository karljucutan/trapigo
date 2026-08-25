package command

import (
	"context"
	"testing"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/stores"
)

type stubRepo struct {
	orders map[int64]domain.Order
	nextID int64
}

type stubUnitOfWork struct {
	repo *stubRepo
}

// RunInTx implements the generic UoW[*stores.TxRepositories] interface for testing.
func (u *stubUnitOfWork) RunInTx(ctx context.Context, fn func(*stores.TxRepositories) error) error {
	stores := &stores.TxRepositories{
		Orders: &stubOrderRepository{repo: u.repo},
	}
	return fn(stores)
}

// stubOrderRepository wraps stubRepo to implement the domain.OrderRepository interface.
type stubOrderRepository struct {
	repo *stubRepo
}

func (s *stubOrderRepository) Create(ctx context.Context, order domain.Order) (domain.Order, error) {
	if s.repo.nextID == 0 {
		s.repo.nextID = 1
	}
	order.ID = s.repo.nextID
	s.repo.nextID++
	s.repo.orders[order.ID] = order
	return order, nil
}

func (s *stubOrderRepository) GetByID(ctx context.Context, id int64) (domain.Order, error) {
	order, ok := s.repo.orders[id]
	if !ok {
		return domain.Order{}, errOrderNotFound
	}
	return order, nil
}

func (s *stubOrderRepository) List(ctx context.Context) ([]domain.Order, error) {
	items := make([]domain.Order, 0, len(s.repo.orders))
	for _, order := range s.repo.orders {
		items = append(items, order)
	}
	return items, nil
}

func (s *stubOrderRepository) Update(ctx context.Context, order domain.Order) (domain.Order, error) {
	s.repo.orders[order.ID] = order
	return order, nil
}

func (s *stubOrderRepository) Delete(ctx context.Context, id int64) error {
	delete(s.repo.orders, id)
	return nil
}

func TestCreateOrderHandler(t *testing.T) {
	repo := &stubRepo{orders: map[int64]domain.Order{}}
	uow := &stubUnitOfWork{repo: repo}
	handler := NewCreateOrderHandler(uow)

	created, err := handler.Handle(context.Background(), CreateOrderCommand{
		CustomerID: 12,
		Items:      []CreateItemInput{{ProductID: 10, Quantity: 2, UnitPriceCents: 2500}},
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if created.TotalAmountCents != 5000 {
		t.Fatalf("total amount cents mismatch: got %d want %d", created.TotalAmountCents, 5000)
	}
}
