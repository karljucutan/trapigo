package command

import (
	"context"
	"errors"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/stores"
	"go-order-service/internal/platform/uow"
)

type UpdateOrderStatusCommand struct {
	ID     int64
	Status string
}

// UpdateOrderStatusHandler updates the status of an order within a transaction.
// It uses the generic UnitOfWork[*stores.Stores] pattern via RunInTx.
type UpdateOrderStatusHandler struct {
	uow uow.UnitOfWork[*stores.Stores]
}

func NewUpdateOrderStatusHandler(u uow.UnitOfWork[*stores.Stores]) *UpdateOrderStatusHandler {
	return &UpdateOrderStatusHandler{uow: u}
}

func (h *UpdateOrderStatusHandler) Handle(ctx context.Context, cmd UpdateOrderStatusCommand) (domain.Order, error) {
	if cmd.Status == "" {
		return domain.Order{}, errors.New("status is required")
	}

	var updated domain.Order

	err := h.uow.RunInTx(ctx, func(stores *stores.Stores) error {
		order, err := stores.Orders.GetByID(ctx, cmd.ID)
		if err != nil {
			return fmt.Errorf("%w: %d", errOrderNotFound, cmd.ID)
		}

		order.UpdateStatus(cmd.Status)

		updated, err = stores.Orders.Update(ctx, order)
		return err
	})

	return updated, err
}
