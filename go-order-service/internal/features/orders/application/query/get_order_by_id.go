package query

import (
	"context"
	"fmt"

	"go-order-service/internal/features/orders/domain"
)

type GetOrderByIDQuery struct {
	ID int64 `json:"id"`
}

type GetOrderByIDResponse struct {
	Order domain.Order
}

type GetOrderByIDHandler struct {
	repo domain.OrderRepository
}

func NewGetOrderByIDHandler(repo domain.OrderRepository) *GetOrderByIDHandler {
	return &GetOrderByIDHandler{repo: repo}
}

func (h *GetOrderByIDHandler) Handle(ctx context.Context, query GetOrderByIDQuery) (domain.Order, error) {
	order, err := h.repo.GetByID(ctx, query.ID)
	if err != nil {
		return domain.Order{}, fmt.Errorf("%w: %d", domain.ErrOrderNotFound, query.ID)
	}
	return order, nil
}
