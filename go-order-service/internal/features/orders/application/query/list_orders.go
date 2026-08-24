package query

import (
	"context"

	"go-order-service/internal/features/orders/domain"
)

type ListOrdersResponse struct {
	Orders []domain.Order
}

type ListOrdersHandler struct {
	repo domain.OrderRepository
}

func NewListOrdersHandler(repo domain.OrderRepository) *ListOrdersHandler {
	return &ListOrdersHandler{repo: repo}
}

func (h *ListOrdersHandler) Handle(ctx context.Context) ([]domain.Order, error) {
	return h.repo.List(ctx)
}
