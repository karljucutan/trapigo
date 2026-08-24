package command

import (
	"context"
	"fmt"

	"go-order-service/internal/platform/stores"
	"go-order-service/internal/platform/uow"
)

type DeleteOrderCommand struct {
	ID int64
}

type DeleteOrderHandler struct {
	uow uow.UnitOfWork[*stores.Stores]
}

func NewDeleteOrderHandler(u uow.UnitOfWork[*stores.Stores]) *DeleteOrderHandler {
	return &DeleteOrderHandler{uow: u}
}

func (h *DeleteOrderHandler) Handle(ctx context.Context, cmd DeleteOrderCommand) error {
	return h.uow.RunInTx(ctx, func(stores *stores.Stores) error {
		if _, err := stores.Orders.GetByID(ctx, cmd.ID); err != nil {
			return fmt.Errorf("%w: %d", errOrderNotFound, cmd.ID)
		}
		return stores.Orders.Delete(ctx, cmd.ID)
	})
}
